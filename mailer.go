package mailer

import (
	"context"
)

// Public interfaces for the mailer library
type (
	// Mailer defines the core email sending interface.
	// All methods are safe for concurrent use.
	Mailer interface {
		// Send sends a single email.
		// Returns an error if the email cannot be sent or if validation fails.
		Send(ctx context.Context, email *Email) error

		// SendBatch sends multiple emails efficiently.
		// If any email fails, the operation continues and returns a BatchError
		// containing details about failed emails.
		SendBatch(ctx context.Context, emails []*Email) error

		// SendTemplate sends an email using a template.
		// The template is rendered with the provided data before sending.
		SendTemplate(ctx context.Context, req *TemplateRequest) error

		// Close closes the mailer and releases any resources.
		// After calling Close, the mailer should not be used.
		Close() error
	}

	// TemplateEngine defines the interface for template rendering.
	TemplateEngine interface {
		// Render renders a template with the provided data.
		Render(templateName string, data interface{}) (string, error)

		// RegisterTemplate registers a template with the given name and content.
		RegisterTemplate(name string, content string) error

		// LoadTemplatesFromDir loads all templates from the specified directory.
		// Templates should follow the naming convention: <name>.<type>.<ext>
		// where type is 'subject', 'html', or 'text'.
		LoadTemplatesFromDir(dir string) error
	}
)
