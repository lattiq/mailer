// Package dryrun provides a no-delivery mailer provider that records every
// send to a slog.Logger instead of dispatching it to an external service. It
// runs the same code path as production providers (validation, retry hooks,
// circuit breaker, etc.) without needing real credentials — intended for
// local development and tests where the email contents should be visible
// in the application log for inspection.
package dryrun

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/lattiq/mailer/internal/core"
)

// Provider implements core.Provider. Each Send call is logged at info level
// with the email's envelope and a body-size hint; SendBatch iterates Send.
// No network calls are made and no errors are returned from a well-formed
// request.
//
// Recognised settings (each "true" / anything else, default off):
//   - "include_text" — when "true", the rendered text body is appended to
//     each log entry as text_body.
//   - "include_html" — when "true", the rendered HTML body is appended to
//     each log entry as html_body.
//
// Both are off by default to keep log lines short and avoid dumping large
// rendered templates into stdout.
type Provider struct {
	logger      *slog.Logger
	includeText bool
	includeHTML bool
}

// NewProvider creates a new dry-run provider.
func NewProvider(settings core.ProviderSettings) (core.Provider, error) {
	return &Provider{
		logger:      slog.Default().With("component", "mailer-dryrun"),
		includeText: settings.Get("include_text") == "true",
		includeHTML: settings.Get("include_html") == "true",
	}, nil
}

// Send logs the email envelope and returns a synthetic SendResult. The
// message id is a random hex string prefixed with "dryrun-" so it is
// unique per call and obviously not a real provider id. Email validation
// is the client's responsibility — sibling providers behave the same way.
func (p *Provider) Send(ctx context.Context, email *core.Email) (*core.SendResult, error) {
	attrs := []any{
		"from", email.From.String(),
		"to", addressList(email.To),
		"cc", addressList(email.CC),
		"bcc", addressList(email.BCC),
		"subject", email.Subject,
		"priority", email.Priority.String(),
		"text_body_size", len(email.TextBody),
		"html_body_size", len(email.HTMLBody),
		"attachments", len(email.Attachments),
		"attachment_names", attachmentList(email.Attachments),
	}
	if p.includeText {
		attrs = append(attrs, "text_body", email.TextBody)
	}
	if p.includeHTML {
		attrs = append(attrs, "html_body", email.HTMLBody)
	}
	p.logger.InfoContext(ctx, "email (dry-run — no delivery)", attrs...)
	return &core.SendResult{
		MessageID: "dryrun-" + randomID(),
		Provider:  p.Name(),
		Timestamp: time.Now().UTC(),
	}, nil
}

// SendBatch logs each email individually. Send cannot fail for the
// dry-run provider, so every input reports as successful in practice;
// the failure-collection loop is kept for shape-parity with other
// providers and to absorb errors from any future Send variant.
func (p *Provider) SendBatch(ctx context.Context, emails []*core.Email) (*core.BatchResult, error) {
	result := &core.BatchResult{
		Total:    len(emails),
		Provider: p.Name(),
	}
	for i, email := range emails {
		sendResult, err := p.Send(ctx, email)
		if err != nil {
			result.Failed = append(result.Failed, core.BatchFailure{
				Index: i,
				Email: email,
				Error: err,
			})
			continue
		}
		result.Successful = append(result.Successful, sendResult)
	}
	return result, nil
}

// ValidateConfig accepts any configuration — the dry-run provider has no
// required settings.
func (p *Provider) ValidateConfig() error {
	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "dryrun"
}

func addressList(addrs []core.Address) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

// attachmentList names each attachment, marking inline ones with the
// cid: reference the HTML must use, e.g. "logo.png (inline cid:logo)".
// It does not read attachment data.
func attachmentList(atts []core.Attachment) []string {
	if len(atts) == 0 {
		return nil
	}
	out := make([]string, len(atts))
	for i := range atts {
		out[i] = atts[i].Filename
		if atts[i].Inline {
			out[i] += " (inline cid:" + atts[i].InlineContentID() + ")"
		}
	}
	return out
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
