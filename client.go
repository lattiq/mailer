package mailer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lattiq/mailer/internal/core"
	"github.com/lattiq/mailer/internal/providers/mailgun"
	"github.com/lattiq/mailer/internal/providers/sendgrid"
	"github.com/lattiq/mailer/internal/providers/ses"
	"github.com/lattiq/mailer/internal/providers/smtp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Type aliases to re-export core types for the public API.
// This allows users to access types like mailer.Email instead of core.Email,
// maintaining a clean public interface while keeping implementation details internal.
type (
	Provider         = core.Provider
	ProviderSettings = core.ProviderSettings
	Email            = core.Email
	Address          = core.Address
	Priority         = core.Priority
	SendResult       = core.SendResult
	BatchResult      = core.BatchResult
	BatchFailure     = core.BatchFailure
	ValidationError  = core.ValidationError
	ProviderError    = core.ProviderError
	Attachment       = core.Attachment
	TemplateRequest  = core.TemplateRequest
	TemplateOptions  = core.TemplateOptions
)

// Priority constants
const (
	PriorityLow    = core.PriorityLow
	PriorityNormal = core.PriorityNormal
	PriorityHigh   = core.PriorityHigh
	PriorityUrgent = core.PriorityUrgent
)

// Error constructor functions
var (
	NewValidationError          = core.NewValidationError
	NewValidationErrorWithValue = core.NewValidationErrorWithValue
	NewProviderError            = core.NewProviderError
	NewRetryableProviderError   = core.NewRetryableProviderError
	NewTemporaryProviderError   = core.NewTemporaryProviderError
	IsRetryable                 = core.IsRetryable
	IsTemporary                 = core.IsTemporary
	GetRetryAfter               = core.GetRetryAfter
)

// Client implements the Mailer interface and provides email sending capabilities.
// All methods are safe for concurrent use.
type Client struct {
	config         Config
	provider       Provider
	fallback       Provider
	templateEng    TemplateEngine
	retryManager   *RetryManager
	rateLimiter    *RateLimiter
	circuitBreaker *CircuitBreaker
	tracer         trace.Tracer
	mu             sync.RWMutex
	closed         bool
}

// New creates a new email client with the given configuration.
// The client must be closed when no longer needed to release resources.
func New(config Config, opts ...Option) (*Client, error) {
	// Apply functional options
	for _, opt := range opts {
		opt(&config)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		config: config,
		tracer: otel.Tracer("github.com/lattiq/mailer"),
	}

	// Initialize provider
	provider, err := createProvider(config.Provider.Type, config.Provider.Primary)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary provider: %w", err)
	}
	client.provider = provider

	// Initialize fallback provider if configured
	if config.Provider.Fallback != nil {
		fallbackType := ProviderType(config.Provider.Fallback.Get("type"))
		if fallbackType != "" {
			fallback, err := createProvider(fallbackType, *config.Provider.Fallback)
			if err != nil {
				return nil, fmt.Errorf("failed to create fallback provider: %w", err)
			}
			client.fallback = fallback
		}
	}

	// Initialize template engine if enabled
	if config.Templates.Enabled {
		templateEng, err := NewTemplateEngine(config.Templates)
		if err != nil {
			return nil, fmt.Errorf("failed to create template engine: %w", err)
		}
		client.templateEng = templateEng
	}

	// Initialize retry manager
	if config.Retry.Enabled {
		client.retryManager = NewRetryManager(config.Retry)
	}

	// Initialize rate limiter
	if config.RateLimit.Enabled {
		client.rateLimiter = NewRateLimiter(config.RateLimit)
	}

	// Initialize circuit breaker
	if config.CircuitBreaker.Enabled {
		client.circuitBreaker = NewCircuitBreaker(config.CircuitBreaker)
	}

	return client, nil
}

// Send sends a single email.
func (c *Client) Send(ctx context.Context, email *Email) error {
	ctx, span := c.tracer.Start(ctx, "mailer.Client.Send")
	defer span.End()

	// Check if client is closed
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		span.RecordError(ErrClientClosed)
		span.SetStatus(codes.Error, ErrClientClosed.Error())
		return ErrClientClosed
	}
	c.mu.RUnlock()

	// Add attributes to span
	span.SetAttributes(
		attribute.String("mailer.to", email.To[0].Email),
		attribute.String("mailer.from", email.From.Email),
		attribute.String("mailer.subject", email.Subject),
		attribute.Int("mailer.recipients", len(email.To)),
		attribute.String("mailer.provider", c.provider.Name()),
	)

	// Validate email
	if err := email.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		return err
	}

	// Apply rate limiting
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx, email); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "rate limited")
			return err
		}
	}

	// Send with circuit breaker and retry
	var result *SendResult
	var err error

	sendFn := func() error {
		var sendErr error
		result, sendErr = c.sendWithProvider(ctx, email, c.provider)

		// Try fallback provider if primary fails and fallback is available
		if sendErr != nil && c.fallback != nil && IsRetryable(sendErr) {
			result, sendErr = c.sendWithProvider(ctx, email, c.fallback)
		}

		return sendErr
	}

	// Apply circuit breaker if enabled
	if c.circuitBreaker != nil {
		err = c.circuitBreaker.Execute(sendFn)
	} else {
		err = sendFn()
	}

	// Apply retry logic if enabled
	if err != nil && c.retryManager != nil && IsRetryable(err) {
		err = c.retryManager.Retry(ctx, sendFn)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "send failed")
		return err
	}

	// Add success attributes
	if result != nil {
		span.SetAttributes(
			attribute.String("mailer.message_id", result.MessageID),
			attribute.String("mailer.status", "sent"),
		)
	}
	span.SetStatus(codes.Ok, "email sent successfully")

	return nil
}

// SendBatch sends multiple emails efficiently.
func (c *Client) SendBatch(ctx context.Context, emails []*Email) error {
	ctx, span := c.tracer.Start(ctx, "mailer.Client.SendBatch")
	defer span.End()

	// Check if client is closed
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		span.RecordError(ErrClientClosed)
		span.SetStatus(codes.Error, ErrClientClosed.Error())
		return ErrClientClosed
	}
	c.mu.RUnlock()

	if len(emails) == 0 {
		span.SetStatus(codes.Ok, "no emails to send")
		return nil
	}

	span.SetAttributes(
		attribute.Int("mailer.batch.size", len(emails)),
		attribute.String("mailer.provider", c.provider.Name()),
	)

	// Validate all emails first
	for i, email := range emails {
		if err := email.Validate(); err != nil {
			validationErr := fmt.Errorf("email at index %d: %w", i, err)
			span.RecordError(validationErr)
			span.SetStatus(codes.Error, "validation failed")
			return validationErr
		}
	}

	// Try batch send with primary provider
	batchResult, err := c.sendBatchWithProvider(ctx, emails, c.provider)

	// If batch send fails and we have a fallback, try individual sends with fallback
	if err != nil && c.fallback != nil {
		return c.sendBatchIndividually(ctx, emails)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "batch send failed")
		return err
	}

	// Set batch results
	successCount := len(batchResult.Successful)
	failureCount := len(batchResult.Failed)

	span.SetAttributes(
		attribute.Int("mailer.batch.success_count", successCount),
		attribute.Int("mailer.batch.failure_count", failureCount),
		attribute.Float64("mailer.batch.success_rate", float64(successCount)/float64(len(emails))),
	)

	if failureCount > 0 {
		batchErr := &BatchError{
			Message: fmt.Sprintf("%d/%d emails failed", failureCount, len(emails)),
			Total:   len(emails),
			Failed:  failureCount,
		}

		// Convert batch failures to batch item errors
		for _, failure := range batchResult.Failed {
			batchErr.Errors = append(batchErr.Errors, BatchItemError{
				Index: failure.Index,
				Error: failure.Error,
			})
		}

		span.RecordError(batchErr)
		span.SetStatus(codes.Error, fmt.Sprintf("%d/%d emails failed", failureCount, len(emails)))
		return batchErr
	}

	span.SetStatus(codes.Ok, "batch send completed successfully")
	return nil
}

// SendTemplate sends an email using a template.
func (c *Client) SendTemplate(ctx context.Context, req *TemplateRequest) error {
	ctx, span := c.tracer.Start(ctx, "mailer.Client.SendTemplate")
	defer span.End()

	// Check if client is closed
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		span.RecordError(ErrClientClosed)
		span.SetStatus(codes.Error, ErrClientClosed.Error())
		return ErrClientClosed
	}
	c.mu.RUnlock()

	if c.templateEng == nil {
		err := errors.New("template engine not enabled")
		span.RecordError(err)
		span.SetStatus(codes.Error, "template engine not enabled")
		return err
	}

	span.SetAttributes(
		attribute.String("mailer.template.name", req.Template),
		attribute.Int("mailer.recipients", len(req.To)),
	)

	// Render template
	renderedSubject := req.Subject
	var renderedHTMLBody, renderedTextBody string
	var err error

	// Render subject if not provided
	if renderedSubject == "" {
		renderedSubject, err = c.templateEng.Render(req.Template+".subject", req.Data)
		if err != nil && !errors.Is(err, ErrTemplateNotFound) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "subject template render failed")
			return NewTemplateError(req.Template, "render", "failed to render subject", err)
		}
	}

	// Render HTML body
	renderedHTMLBody, err = c.templateEng.Render(req.Template+".html", req.Data)
	if err != nil && !errors.Is(err, ErrTemplateNotFound) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "HTML template render failed")
		return NewTemplateError(req.Template, "render", "failed to render HTML body", err)
	}

	// Render text body
	renderedTextBody, err = c.templateEng.Render(req.Template+".text", req.Data)
	if err != nil && !errors.Is(err, ErrTemplateNotFound) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "text template render failed")
		return NewTemplateError(req.Template, "render", "failed to render text body", err)
	}

	// Convert metadata from interface{} to string
	metadata := make(map[string]string)
	for k, v := range req.Metadata {
		metadata[k] = fmt.Sprintf("%v", v)
	}

	// Create email from template request
	email := &Email{
		From:     req.From,
		To:       req.To,
		CC:       req.CC,
		BCC:      req.BCC,
		Subject:  renderedSubject,
		HTMLBody: renderedHTMLBody,
		TextBody: renderedTextBody,
		Headers:  req.Headers,
		Priority: req.Priority,
		Metadata: metadata,
	}

	// Send the email
	return c.Send(ctx, email)
}

// Close closes the client and releases any resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Close template engine if it has a Close method
	if closer, ok := c.templateEng.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("failed to close template engine: %w", err)
		}
	}

	return nil
}

// sendWithProvider sends an email using a specific provider.
func (c *Client) sendWithProvider(ctx context.Context, email *Email, provider Provider) (*SendResult, error) {
	startTime := time.Now()

	result, err := provider.Send(ctx, email)

	duration := time.Since(startTime)

	// Add timing information to any existing span
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("mailer.provider.duration_ms", duration.Milliseconds()),
		)
	}

	return result, err
}

// sendBatchWithProvider sends multiple emails using a specific provider.
func (c *Client) sendBatchWithProvider(ctx context.Context, emails []*Email, provider Provider) (*BatchResult, error) {
	startTime := time.Now()

	result, err := provider.SendBatch(ctx, emails)

	duration := time.Since(startTime)

	// Add timing information to any existing span
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(
			attribute.Int64("mailer.provider.batch_duration_ms", duration.Milliseconds()),
		)
	}

	return result, err
}

// sendBatchIndividually sends emails individually when batch operations fail.
func (c *Client) sendBatchIndividually(ctx context.Context, emails []*Email) error {
	var successCount, failureCount int
	var batchErrors []BatchItemError

	for i, email := range emails {
		// Create child span for each email
		emailCtx, emailSpan := c.tracer.Start(ctx, "mailer.Client.SendBatch.email",
			trace.WithAttributes(
				attribute.Int("mailer.batch.index", i),
				attribute.String("mailer.to", email.To[0].Email),
			),
		)

		if err := c.Send(emailCtx, email); err != nil {
			emailSpan.RecordError(err)
			emailSpan.SetStatus(codes.Error, err.Error())
			failureCount++
			batchErrors = append(batchErrors, BatchItemError{
				Index: i,
				Error: err,
			})
		} else {
			emailSpan.SetStatus(codes.Ok, "email sent")
			successCount++
		}
		emailSpan.End()
	}

	if failureCount > 0 {
		return &BatchError{
			Message: fmt.Sprintf("%d/%d emails failed in fallback individual send", failureCount, len(emails)),
			Errors:  batchErrors,
			Total:   len(emails),
			Failed:  failureCount,
		}
	}

	return nil
}

// createProvider creates a provider instance based on type and settings.
func createProvider(providerType ProviderType, settings ProviderSettings) (Provider, error) {
	switch providerType {
	case ProviderAWSSES:
		return newSESProvider(settings)
	case ProviderSendGrid:
		return newSendGridProvider(settings)
	case ProviderMailgun:
		return newMailgunProvider(settings)
	case ProviderSMTP:
		return newSMTPProvider(settings)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

func newSESProvider(settings ProviderSettings) (Provider, error) {
	return ses.NewProvider(settings)
}

func newSendGridProvider(settings ProviderSettings) (Provider, error) {
	return sendgrid.NewProvider(settings)
}

func newMailgunProvider(settings ProviderSettings) (Provider, error) {
	return mailgun.NewProvider(settings)
}

func newSMTPProvider(settings ProviderSettings) (Provider, error) {
	return smtp.NewProvider(settings)
}
