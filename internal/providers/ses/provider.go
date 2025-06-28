package ses

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"

	"github.com/lattiq/mailer/internal/core"
)

// Provider implements the core.Provider interface for AWS SES.
type Provider struct {
	client *ses.Client
	config core.ProviderSettings
}

// NewProvider creates a new AWS SES provider.
func NewProvider(settings core.ProviderSettings) (core.Provider, error) {
	region := settings.Get("region")
	if region == "" {
		return nil, core.NewValidationError("region", "AWS region is required")
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, core.NewProviderError("aws_ses", "config_error", "failed to load AWS config: "+err.Error())
	}

	// Override with explicit credentials if provided
	if accessKey := settings.Get("access_key"); accessKey != "" {
		secretKey := settings.Get("secret_key")
		if secretKey == "" {
			return nil, core.NewValidationError("secret_key", "secret key is required when access key is provided")
		}

		// Create credentials using static credentials provider
		cfg.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
				SessionToken:    settings.Get("session_token"),
			}, nil
		})
	}

	client := ses.NewFromConfig(cfg)

	provider := &Provider{
		client: client,
		config: settings,
	}

	return provider, nil
}

// Send sends a single email using AWS SES.
func (p *Provider) Send(ctx context.Context, email *core.Email) (*core.SendResult, error) {
	input := &ses.SendEmailInput{
		Source: aws.String(email.From.String()),
		Destination: &types.Destination{
			ToAddresses: p.convertAddresses(email.To),
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(email.Subject),
			},
			Body: &types.Body{},
		},
	}

	// Add CC addresses if present
	if len(email.CC) > 0 {
		input.Destination.CcAddresses = p.convertAddresses(email.CC)
	}

	// Add BCC addresses if present
	if len(email.BCC) > 0 {
		input.Destination.BccAddresses = p.convertAddresses(email.BCC)
	}

	// Set email body
	if email.TextBody != "" {
		input.Message.Body.Text = &types.Content{
			Data: aws.String(email.TextBody),
		}
	}

	if email.HTMLBody != "" {
		input.Message.Body.Html = &types.Content{
			Data: aws.String(email.HTMLBody),
		}
	}

	// Add configuration set if specified
	if configSet := p.config.Get("configuration_set"); configSet != "" {
		input.ConfigurationSetName = aws.String(configSet)
	}

	// Send the email
	output, err := p.client.SendEmail(ctx, input)
	if err != nil {
		return nil, core.NewProviderError("aws_ses", "send_error", "failed to send email: "+err.Error())
	}

	return &core.SendResult{
		MessageID: aws.ToString(output.MessageId),
		Provider:  p.Name(),
		Timestamp: time.Now(),
	}, nil
}

// SendBatch sends multiple emails. AWS SES doesn't have a native batch API,
// so we send emails individually.
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
	if p.config.Get("region") == "" {
		return core.NewValidationError("region", "AWS region is required")
	}
	return nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "aws_ses"
}

// convertAddresses converts core.Address slice to string slice.
func (p *Provider) convertAddresses(addresses []core.Address) []string {
	result := make([]string, len(addresses))
	for i, addr := range addresses {
		result[i] = addr.String()
	}
	return result
}
