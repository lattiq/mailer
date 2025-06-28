package mailgun

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mailgun/mailgun-go/v4"

	"github.com/lattiq/mailer/internal/core"
)

// Provider implements the core.Provider interface for Mailgun.
type Provider struct {
	client mailgun.Mailgun
	config core.ProviderSettings
}

// NewProvider creates a new Mailgun provider.
func NewProvider(settings core.ProviderSettings) (core.Provider, error) {
	apiKey := settings.Get("api_key")
	if apiKey == "" {
		return nil, core.NewValidationError("api_key", "Mailgun API key is required")
	}

	domain := settings.Get("domain")
	if domain == "" {
		return nil, core.NewValidationError("domain", "Mailgun domain is required")
	}

	// Create Mailgun client
	client := mailgun.NewMailgun(domain, apiKey)

	// Set base URL if provided (for EU customers)
	if baseURL := settings.Get("base_url"); baseURL != "" {
		client.SetAPIBase(baseURL)
	}

	provider := &Provider{
		client: client,
		config: settings,
	}

	return provider, nil
}

// Send sends a single email using Mailgun.
func (p *Provider) Send(ctx context.Context, email *core.Email) (*core.SendResult, error) {
	// Create message - note: v4 API uses NewMessage as a standalone function
	message := mailgun.NewMessage(email.From.String(), email.Subject, email.TextBody, email.To[0].String())

	// Add additional recipients
	for i := 1; i < len(email.To); i++ {
		if err := message.AddRecipient(email.To[i].String()); err != nil {
			return nil, core.NewProviderError("mailgun", "recipient_add_failed", fmt.Sprintf("failed to add recipient %s: %v", email.To[i].String(), err))
		}
	}

	// Add CC recipients
	for _, cc := range email.CC {
		message.AddCC(cc.String())
	}

	// Add BCC recipients
	for _, bcc := range email.BCC {
		message.AddBCC(bcc.String())
	}

	// Set HTML body if provided
	if email.HTMLBody != "" {
		message.SetHTML(email.HTMLBody)
	}

	// Add custom headers
	for key, value := range email.Headers {
		message.AddHeader(key, value)
	}

	// Set priority headers if specified
	switch email.Priority {
	case core.PriorityHigh:
		message.AddHeader("X-Priority", "2")
		message.AddHeader("Importance", "high")
	case core.PriorityUrgent:
		message.AddHeader("X-Priority", "1")
		message.AddHeader("Importance", "high")
	case core.PriorityLow:
		message.AddHeader("X-Priority", "4")
		message.AddHeader("Importance", "low")
	}

	// Add attachments
	for _, attachment := range email.Attachments {
		if attachment.Data != nil {
			// Read the data into a byte slice
			data, err := io.ReadAll(attachment.Data)
			if err != nil {
				return nil, core.NewProviderError("mailgun", "attachment_read_failed", err.Error())
			}
			message.AddBufferAttachment(attachment.Filename, data)
		}
	}

	// Send the email - Mailgun v4 returns 3 values: mes, id, err
	mes, id, err := p.client.Send(ctx, message)
	if err != nil {
		return nil, core.NewProviderError("mailgun", "send_failed", err.Error())
	}

	return &core.SendResult{
		MessageID: id,
		Provider:  p.Name(),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"message": mes,
		},
	}, nil
}

// SendBatch sends multiple emails using Mailgun.
func (p *Provider) SendBatch(ctx context.Context, emails []*core.Email) (*core.BatchResult, error) {
	var successful []*core.SendResult
	var failed []core.BatchFailure

	for i, email := range emails {
		result, err := p.Send(ctx, email)
		if err != nil {
			failed = append(failed, core.BatchFailure{
				Index: i,
				Email: email,
				Error: err,
			})
		} else {
			successful = append(successful, result)
		}
	}

	return &core.BatchResult{
		Total:      len(emails),
		Successful: successful,
		Failed:     failed,
		Provider:   p.Name(),
	}, nil
}

// ValidateConfig validates the Mailgun provider configuration.
func (p *Provider) ValidateConfig() error {
	if p.config.Get("api_key") == "" {
		return core.NewValidationError("api_key", "Mailgun API key is required")
	}
	if p.config.Get("domain") == "" {
		return core.NewValidationError("domain", "Mailgun domain is required")
	}
	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "mailgun"
}
