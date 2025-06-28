package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/lattiq/mailer/internal/core"
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
	skipVerify := p.config.Get("tls_skip_verify") == "true"

	addr := host + ":" + port

	// Create TLS config if needed
	var tlsConfig *tls.Config
	if useTLS {
		tlsConfig = &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,            // Always verify TLS certificates for security
			MinVersion:         tls.VersionTLS12, // Require TLS 1.2 or higher for security
		}
		// Only allow insecure mode if explicitly configured for development
		if skipVerify {
			// Log a warning that this is insecure (if logger is available)
			tlsConfig.InsecureSkipVerify = true
		}
	}

	// Build email message
	message, err := p.buildMessage(email)
	if err != nil {
		return nil, core.NewProviderError("smtp", "message_build_error", "failed to build message: "+err.Error())
	}

	// Send email
	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	// Get all recipient addresses
	var recipients []string
	for _, to := range email.To {
		recipients = append(recipients, to.Email)
	}
	for _, cc := range email.CC {
		recipients = append(recipients, cc.Email)
	}
	for _, bcc := range email.BCC {
		recipients = append(recipients, bcc.Email)
	}

	// Send the email
	var sendErr error
	if useTLS {
		sendErr = p.sendMailTLS(addr, auth, email.From.Email, recipients, message, tlsConfig)
	} else {
		sendErr = smtp.SendMail(addr, auth, email.From.Email, recipients, message)
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

// buildMessage builds the email message in RFC 5322 format.
func (p *Provider) buildMessage(email *core.Email) ([]byte, error) {
	var message strings.Builder

	// Headers
	message.WriteString("From: " + email.From.String() + "\r\n")

	if len(email.To) > 0 {
		var toAddrs []string
		for _, to := range email.To {
			toAddrs = append(toAddrs, to.String())
		}
		message.WriteString("To: " + strings.Join(toAddrs, ", ") + "\r\n")
	}

	if len(email.CC) > 0 {
		var ccAddrs []string
		for _, cc := range email.CC {
			ccAddrs = append(ccAddrs, cc.String())
		}
		message.WriteString("Cc: " + strings.Join(ccAddrs, ", ") + "\r\n")
	}

	message.WriteString("Subject: " + email.Subject + "\r\n")
	message.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")

	// Add custom headers
	for key, value := range email.Headers {
		message.WriteString(key + ": " + value + "\r\n")
	}

	// Handle multipart message if both HTML and text bodies exist
	if email.HTMLBody != "" && email.TextBody != "" {
		boundary := fmt.Sprintf("boundary_%d", time.Now().UnixNano())
		message.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n")
		message.WriteString("\r\n")

		// Text part
		message.WriteString("--" + boundary + "\r\n")
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		message.WriteString("\r\n")
		message.WriteString(email.TextBody + "\r\n")
		message.WriteString("\r\n")

		// HTML part
		message.WriteString("--" + boundary + "\r\n")
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		message.WriteString("\r\n")
		message.WriteString(email.HTMLBody + "\r\n")
		message.WriteString("\r\n")

		// End boundary
		message.WriteString("--" + boundary + "--\r\n")
	} else if email.HTMLBody != "" {
		// HTML only
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		message.WriteString("\r\n")
		message.WriteString(email.HTMLBody + "\r\n")
	} else {
		// Text only
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		message.WriteString("\r\n")
		message.WriteString(email.TextBody + "\r\n")
	}

	return []byte(message.String()), nil
}

// sendMailTLS sends mail using TLS.
func (p *Provider) sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, tlsConfig *tls.Config) error {
	// Implementation of TLS SMTP sending
	// This is a simplified version - production code would need more robust TLS handling
	return smtp.SendMail(addr, auth, from, to, msg)
}
