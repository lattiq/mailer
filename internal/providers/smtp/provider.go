package smtp

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"time"

	"github.com/lattiq/mailer/internal/core"
	"github.com/lattiq/mailer/internal/message"
)

// Provider implements the core.Provider interface for SMTP.
type Provider struct {
	config core.ProviderSettings
}

// NewProvider creates a new SMTP provider.
func NewProvider(settings core.ProviderSettings) (core.Provider, error) {
	host := settings.Get("host")
	if host == "" {
		return nil, core.NewValidationError("host", "SMTP host is required")
	}

	port := settings.Get("port")
	if port == "" {
		return nil, core.NewValidationError("port", "SMTP port is required")
	}

	// Validate port number
	if _, err := strconv.Atoi(port); err != nil {
		return nil, core.NewValidationError("port", "invalid port number: "+port)
	}

	provider := &Provider{
		config: settings,
	}

	return provider, nil
}

// Send sends a single email using SMTP.
func (p *Provider) Send(ctx context.Context, email *core.Email) (*core.SendResult, error) {
	host := p.config.Get("host")
	port := p.config.Get("port")
	username := p.config.Get("username")
	password := p.config.Get("password")
	useTLS := p.config.Get("tls") == "true"

	addr := host + ":" + port

	// TLS handling is a stub: net/smtp.SendMail does STARTTLS opportunistically
	// using its own default tls.Config, so user TLS settings (ServerName,
	// MinVersion, InsecureSkipVerify) are not honored. A future revision
	// should wire smtp.Client manually to apply a custom *tls.Config.

	// Build email message
	msg, err := message.Build(email)
	if err != nil {
		return nil, core.NewProviderError("smtp", "build_error", "failed to build message: "+err.Error())
	}

	// Send email
	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	// Envelope recipients: To, Cc and Bcc (Bcc is not in the message headers)
	var recipients []string
	for _, r := range email.AllRecipients() {
		recipients = append(recipients, r.Email)
	}

	// Send the email
	var sendErr error
	if useTLS {
		sendErr = p.sendMailTLS(addr, auth, email.From.Email, recipients, msg)
	} else {
		sendErr = smtp.SendMail(addr, auth, email.From.Email, recipients, msg)
	}

	if sendErr != nil {
		return nil, core.NewProviderError("smtp", "send_error", "failed to send email: "+sendErr.Error())
	}

	// Generate a simple message ID (SMTP doesn't provide one)
	messageID := fmt.Sprintf("%d@%s", time.Now().UnixNano(), host)

	return &core.SendResult{
		MessageID: messageID,
		Provider:  p.Name(),
		Timestamp: time.Now(),
	}, nil
}

// SendBatch sends multiple emails individually.
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
		} else {
			result.Successful = append(result.Successful, sendResult)
		}
	}

	return result, nil
}

// ValidateConfig validates the provider configuration.
func (p *Provider) ValidateConfig() error {
	if p.config.Get("host") == "" {
		return core.NewValidationError("host", "SMTP host is required")
	}

	port := p.config.Get("port")
	if port == "" {
		return core.NewValidationError("port", "SMTP port is required")
	}

	if _, err := strconv.Atoi(port); err != nil {
		return core.NewValidationError("port", "invalid port number: "+port)
	}

	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "smtp"
}

// sendMailTLS sends mail with TLS expected. net/smtp.SendMail performs
// STARTTLS using a default tls.Config when the server advertises it — this
// is the stub implementation noted on Send; replace when wiring smtp.Client
// directly.
func (p *Provider) sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	return smtp.SendMail(addr, auth, from, to, msg)
}
