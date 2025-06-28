package providers

import (
	"github.com/lattiq/mailer"
	"github.com/lattiq/mailer/internal/providers/mailgun"
	"github.com/lattiq/mailer/internal/providers/sendgrid"
	"github.com/lattiq/mailer/internal/providers/ses"
	"github.com/lattiq/mailer/internal/providers/smtp"
)

// NewSESProvider creates a new AWS SES provider.
func NewSESProvider(settings mailer.ProviderSettings) (mailer.Provider, error) {
	return ses.NewProvider(settings)
}

// NewSendGridProvider creates a new SendGrid provider.
func NewSendGridProvider(settings mailer.ProviderSettings) (mailer.Provider, error) {
	return sendgrid.NewProvider(settings)
}

// NewMailgunProvider creates a new Mailgun provider.
func NewMailgunProvider(settings mailer.ProviderSettings) (mailer.Provider, error) {
	return mailgun.NewProvider(settings)
}

// NewSMTPProvider creates a new SMTP provider.
func NewSMTPProvider(settings mailer.ProviderSettings) (mailer.Provider, error) {
	return smtp.NewProvider(settings)
}
