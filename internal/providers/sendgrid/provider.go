package sendgrid

import (
	"context"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"github.com/lattiq/mailer/internal/core"
)

// Provider implements the core.Provider interface for SendGrid.
type Provider struct {
	client *sendgrid.Client
	config core.ProviderSettings
}

// NewProvider creates a new SendGrid provider.
func NewProvider(settings core.ProviderSettings) (core.Provider, error) {
	apiKey := settings.Get("api_key")
	if apiKey == "" {
		return nil, core.NewValidationError("api_key", "SendGrid API key is required")
	}

	client := sendgrid.NewSendClient(apiKey)

	provider := &Provider{
		client: client,
		config: settings,
	}

	return provider, nil
}

// Send sends a single email using SendGrid.
func (p *Provider) Send(ctx context.Context, email *core.Email) (*core.SendResult, error) {
	// Convert from address
	from := mail.NewEmail(email.From.Name, email.From.Email)

	// Convert to addresses
	if len(email.To) == 0 {
		return nil, core.NewValidationError("to", "at least one recipient is required")
	}

	// For simplicity, we'll send to the first recipient and add others as personalizations
	to := mail.NewEmail(email.To[0].Name, email.To[0].Email)

	// Create the message
	message := mail.NewSingleEmail(from, email.Subject, to, email.TextBody, email.HTMLBody)

	// Add additional recipients if any
	if len(email.To) > 1 || len(email.CC) > 0 || len(email.BCC) > 0 {
		personalization := mail.NewPersonalization()

		// Add all TO recipients
		for _, recipient := range email.To {
			personalization.AddTos(mail.NewEmail(recipient.Name, recipient.Email))
		}

		// Add CC recipients
		for _, recipient := range email.CC {
			personalization.AddCCs(mail.NewEmail(recipient.Name, recipient.Email))
		}

		// Add BCC recipients
		for _, recipient := range email.BCC {
			personalization.AddBCCs(mail.NewEmail(recipient.Name, recipient.Email))
		}

		message.Personalizations = []*mail.Personalization{personalization}
	}

	// Add custom headers
	if len(email.Headers) > 0 {
		if message.Headers == nil {
			message.Headers = make(map[string]string)
		}
		for key, value := range email.Headers {
			message.Headers[key] = value
		}
	}

	// Send the email
	response, err := p.client.Send(message)
	if err != nil {
		return nil, core.NewProviderError("sendgrid", "send_error", "failed to send email: "+err.Error())
	}

	// Check response status
	if response.StatusCode >= 400 {
		return nil, core.NewProviderError("sendgrid", "api_error", "SendGrid API error: "+response.Body)
	}

	// Extract message ID from headers (SendGrid provides X-Message-Id)
	messageID := response.Headers["X-Message-Id"]
	if len(messageID) == 0 {
		messageID = []string{"unknown"}
	}

	return &core.SendResult{
		MessageID: messageID[0],
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
	if p.config.Get("api_key") == "" {
		return core.NewValidationError("api_key", "SendGrid API key is required")
	}
	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "sendgrid"
}
