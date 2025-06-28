# Mailer - Go Email Library Technical Documentation

## Overview

**mailer** is a high-performance, provider-agnostic Go library for sending emails with a focus on transactional email use cases. It provides a clean, idiomatic Go API with support for multiple email providers, template management, and built-in reliability features.

## Design Principles

### Core Principles

- **Provider Agnostic**: Abstract away provider-specific implementations
- **Minimal Dependencies**: Only essential external dependencies
- **Context-Aware**: Full support for `context.Context` propagation
- **Type Safety**: Leverage Go's type system for compile-time safety
- **Progressive Complexity**: Simple use cases remain simple, complex use cases possible
- **No Hidden Costs**: Predictable performance characteristics

### Library Scope

- **Primary**: Transactional email sending (OTP, notifications, alerts)
- **Secondary**: Template management and rendering
- **Out of Scope**: Marketing email campaigns, bulk mailing, email parsing

## Architecture Overview

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Application   │───▶│     Mailer       │───▶│   Provider      │
│                 │    │   (Interface)    │    │ (SES/SendGrid)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │   Template       │
                       │   Engine         │
                       └──────────────────┘
```

### Package Structure

```
mailer/
├── doc.go                    # Package documentation
├── mailer.go                 # Main client interface
├── client.go                 # Client implementation
├── config.go                 # Configuration types
├── options.go                # Functional options
├── template.go               # Template management
├── errors.go                 # Custom error types
├── retry.go                  # Retry logic
├── version.go                # Version information
├── docs/                     # Documentation
│   └── TECHNICAL.md         # Technical documentation
├── examples/                 # Usage examples
│   └── otp/                 # OTP email example with templates
│       ├── main.go          # Example using AWS SES with templates
│       ├── README.md        # Example documentation
│       └── templates/       # Template files for OTP emails
│           ├── otp.html.html # HTML template (double extension)
│           └── otp.text.text # Text template (double extension)
├── internal/                 # Private implementation
│   ├── core/               # Core types and interfaces
│   │   └── types.go        # Core type definitions
│   └── providers/          # Provider implementations
│       ├── provider.go     # Provider interface
│       ├── ses/            # AWS SES provider
│       │   └── provider.go
│       ├── sendgrid/       # SendGrid provider
│       │   └── provider.go
│       ├── mailgun/        # Mailgun provider
│       │   └── provider.go
│       └── smtp/           # Generic SMTP provider
│           └── provider.go
├── Makefile                  # Build and development tasks
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

## Core Interfaces

### Primary Interface

```go
// Mailer defines the core email sending interface
type Mailer interface {
    // Send sends a single email
    Send(ctx context.Context, email *Email) error

    // SendBatch sends multiple emails efficiently
    SendBatch(ctx context.Context, emails []*Email) error

    // SendTemplate sends an email using a template
    SendTemplate(ctx context.Context, req *TemplateRequest) error

    // Close closes the mailer and releases resources
    Close() error
}

// Provider defines the interface for email service providers
type Provider interface {
    Send(ctx context.Context, email *Email) (*SendResult, error)
    SendBatch(ctx context.Context, emails []*Email) (*BatchResult, error)
    ValidateConfig() error
    Name() string
}

// TemplateEngine defines the interface for template rendering
type TemplateEngine interface {
    Render(templateName string, data interface{}) (string, error)
    RegisterTemplate(name string, content string) error
    LoadTemplatesFromDir(dir string) error
}
```

### Configuration Types

```go
// Config holds the complete mailer configuration
type Config struct {
    Provider       ProviderConfig
    Templates      TemplateConfig
    Retry          RetryConfig
    RateLimit      RateLimitConfig
    Monitoring     MonitoringConfig
}

// ProviderConfig contains provider-specific settings
type ProviderConfig struct {
    Type       ProviderType
    Primary    ProviderSettings
    Fallback   *ProviderSettings
    Timeout    time.Duration
}

// Email represents an email message
type Email struct {
    From        Address
    To          []Address
    CC          []Address
    BCC         []Address
    Subject     string
    TextBody    string
    HTMLBody    string
    Attachments []Attachment
    Headers     map[string]string
    Priority    Priority
    Metadata    map[string]interface{}
}
```

## Provider Architecture

### Provider Types

```go
type ProviderType string

const (
    ProviderAWSSES    ProviderType = "aws_ses"
    ProviderSendGrid  ProviderType = "sendgrid"
    ProviderMailgun   ProviderType = "mailgun"
    ProviderSMTP      ProviderType = "smtp"
)
```

### Provider Implementation Pattern

```go
// Each provider implements the Provider interface
type sesProvider struct {
    client    *ses.Client
    config    *SESConfig
    validator *validation.Validator
}

func (p *sesProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
    // Validate input
    if err := p.validator.ValidateEmail(email); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Convert to provider format
    input := p.buildSESInput(email)

    // Send with retries
    output, err := p.sendWithRetry(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("send failed: %w", err)
    }

    return &SendResult{
        MessageID: aws.ToString(output.MessageId),
        Provider:  p.Name(),
    }, nil
}
```

## Template System

### Template Engine Design

```go
// TemplateRequest represents a template-based email request
type TemplateRequest struct {
    Template    string
    To          []Address
    From        Address
    Subject     string  // Optional: can be in template
    Data        interface{}
    Options     *TemplateOptions
}

// TemplateOptions provides template rendering options
type TemplateOptions struct {
    Locale      string
    Timezone    *time.Location
    Partials    map[string]string
    Helpers     map[string]interface{}
}
```

### Template File Structure

```
templates/
├── otp/
│   ├── subject.txt
│   ├── body.html
│   └── body.txt
├── notification/
│   ├── subject.txt
│   ├── body.html
│   └── body.txt
└── partials/
    ├── header.html
    └── footer.html
```

## Error Handling

### Error Types

```go
// Error types for different failure scenarios
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

type ProviderError struct {
    Provider string
    Code     string
    Message  string
    Retryable bool
    Temporary bool
}

func (e *ProviderError) Error() string {
    return fmt.Sprintf("provider %s error [%s]: %s", e.Provider, e.Code, e.Message)
}

// Sentinel errors for common cases
var (
    ErrInvalidEmail     = errors.New("invalid email address")
    ErrTemplateNotFound = errors.New("template not found")
    ErrProviderTimeout  = errors.New("provider timeout")
    ErrRateLimited      = errors.New("rate limited")
)
```

### Error Classification Interfaces

```go
// RetryableError indicates if an error can be retried
type RetryableError interface {
    Retryable() bool
}

// TemporaryError indicates if an error is temporary
type TemporaryError interface {
    Temporary() bool
}

// RateLimitError provides rate limit information
type RateLimitError interface {
    RetryAfter() time.Duration
}
```

## Reliability Features

### Retry Logic

```go
type RetryConfig struct {
    MaxAttempts   int
    InitialDelay  time.Duration
    MaxDelay      time.Duration
    Multiplier    float64
    Jitter        bool
}

// Default retry configuration
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts:  3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay:     5 * time.Second,
        Multiplier:   2.0,
        Jitter:       true,
    }
}
```

### Rate Limiting

```go
type RateLimitConfig struct {
    Enabled     bool
    Rate        int           // requests per period
    Period      time.Duration
    Burst       int           // burst capacity
}
```

### Circuit Breaker

```go
type CircuitBreakerConfig struct {
    Enabled             bool
    FailureThreshold    int
    SuccessThreshold    int
    Timeout             time.Duration
    ResetTimeout        time.Duration
}
```

## Performance Characteristics

### Memory Usage

- **Email Objects**: ~1KB per email (without attachments)
- **Template Cache**: ~10KB per cached template
- **Connection Pools**: Configurable, default 10 connections per provider
- **Buffer Pools**: Automatic pooling for temporary buffers

### Throughput Expectations

- **Single Email**: <10ms processing time (excluding network)
- **Batch Operations**: ~1ms per email in batch
- **Template Rendering**: <1ms per template (cached)
- **Provider Latency**: 50-500ms depending on provider

### Concurrency Model

- **Thread-Safe**: All public methods are safe for concurrent use
- **Connection Pooling**: Automatic HTTP connection pooling
- **Context Cancellation**: All operations respect context cancellation
- **No Goroutine Leaks**: All internal goroutines are properly managed

## Configuration Examples

### Basic Configuration

```go
config := mailer.Config{
    Provider: mailer.ProviderConfig{
        Type: mailer.ProviderAWSSES,
        Primary: mailer.ProviderSettings{
            "region":     "us-east-1",
            "access_key": "your-access-key",
            "secret_key": "your-secret-key",
        },
        Timeout: 30 * time.Second,
    },
}
```

### Multi-Provider with Fallback

```go
config := mailer.Config{
    Provider: mailer.ProviderConfig{
        Type: mailer.ProviderAWSSES,
        Primary: mailer.ProviderSettings{
            "region": "us-east-1",
            // SES configuration
        },
        Fallback: &mailer.ProviderSettings{
            "type": "sendgrid",
            "api_key": "your-sendgrid-key",
        },
    },
}
```

## Usage Patterns

### Simple Email Sending

```go
// Create client
client, err := mailer.New(config)
if err != nil {
    return err
}
defer client.Close()

// Send email
email := &mailer.Email{
    From:     mailer.Address{Email: "noreply@lattiq.com"},
    To:       []mailer.Address{{Email: "user@example.com"}},
    Subject:  "Welcome",
    HTMLBody: "<h1>Welcome to our service!</h1>",
    TextBody: "Welcome to our service!",
}

err = client.Send(ctx, email)
```

### Template-Based Sending

```go
// Send using template
req := &mailer.TemplateRequest{
    Template: "otp",
    To:       []mailer.Address{{Email: "user@example.com"}},
    From:     mailer.Address{Email: "auth@lattiq.com"},
    Data: map[string]interface{}{
        "Code":    "123456",
        "Expires": time.Now().Add(5 * time.Minute),
        "User":    "John Doe",
    },
}

err = client.SendTemplate(ctx, req)
```

### Batch Operations

```go
emails := []*mailer.Email{
    // ... multiple emails
}

err = client.SendBatch(ctx, emails)
```

## Testing Strategy

### Unit Tests

- Mock provider implementations for unit testing
- Table-driven tests for validation logic
- Fuzz tests for email parsing and validation
- Benchmark tests for performance critical paths

### Integration Tests

- Real provider testing in CI (with test credentials)
- Template rendering integration tests
- End-to-end email delivery tests
- Provider failover testing

### Testing Approach

**Unit Testing with Mocks**

```go
// Use dependency injection for testing
type MockProvider struct {
    sent []Email
}

func (m *MockProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
    m.sent = append(m.sent, *email)
    return &SendResult{MessageID: "mock-123", Provider: "mock"}, nil
}

func TestEmailSending(t *testing.T) {
    mock := &MockProvider{}
    client := &Client{provider: mock}

    email := &Email{
        From: Address{Email: "test@example.com"},
        To:   []Address{{Email: "user@example.com"}},
        Subject: "Test Email",
        TextBody: "This is a test email",
    }

    err := client.Send(context.Background(), email)
    assert.NoError(t, err)
    assert.Len(t, mock.sent, 1)
    assert.Equal(t, "Test Email", mock.sent[0].Subject)
}
```

**Integration Testing**
For integration tests, use test-specific provider credentials or containerized test environments (e.g., MailHog in Docker) rather than requiring localhost setup.

## Security Considerations

### Input Validation

- Email address format validation (RFC 5322 compliant)
- Header injection prevention
- Template injection protection
- File attachment validation

### Credential Management

- Support for environment variables
- Integration with AWS IAM roles
- Secure credential storage recommendations
- Rotation support for long-lived credentials

### Rate Limiting & Abuse Prevention

- Per-sender rate limiting
- IP-based rate limiting
- Template-based limits
- Monitoring hooks for abuse detection

## Monitoring & Observability

### Distributed Tracing

**OpenTelemetry Integration**
All public methods must include tracing spans for complete observability. Follow this pattern for consistent tracing:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

// Tracer instance for the mailer package
var tracer = otel.Tracer("github.com/lattiq/mailer")

// Example implementation for all public methods
func (c *Client) Send(ctx context.Context, email *Email) error {
    // Start span with operation name
    span, ctx := tracer.Start(ctx, "mailer.Client.Send")
    defer span.End()

    // Add relevant attributes
    span.SetAttributes(
        attribute.String("mailer.to", email.To[0].Email),
        attribute.String("mailer.from", email.From.Email),
        attribute.String("mailer.subject", email.Subject),
        attribute.String("mailer.provider", c.provider.Name()),
        attribute.Int("mailer.recipients", len(email.To)),
    )

    // Perform operation
    result, err := c.provider.Send(ctx, email)
    if err != nil {
        // Record error in span
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return fmt.Errorf("send failed: %w", err)
    }

    // Add success attributes
    span.SetAttributes(
        attribute.String("mailer.message_id", result.MessageID),
        attribute.String("mailer.status", "sent"),
    )
    span.SetStatus(codes.Ok, "email sent successfully")

    return nil
}
```

**Required Tracing for All Public Methods:**

```go
// Core interface methods
func (c *Client) Send(ctx context.Context, email *Email) error
func (c *Client) SendBatch(ctx context.Context, emails []*Email) error
func (c *Client) SendTemplate(ctx context.Context, req *TemplateRequest) error

// Provider methods
func (p *sesProvider) Send(ctx context.Context, email *Email) (*SendResult, error)
func (p *smtpProvider) Send(ctx context.Context, email *Email) (*SendResult, error)

// Template methods
func (t *TemplateEngine) Render(ctx context.Context, templateName string, data interface{}) (string, error)
func (t *TemplateEngine) LoadTemplatesFromDir(ctx context.Context, dir string) error
```

**Span Naming Convention:**

- **Package Level**: `mailer.<interface>.<method>`
- **Provider Level**: `mailer.provider.<provider_name>.<operation>`
- **Template Level**: `mailer.template.<operation>`
- **Internal Level**: `mailer.internal.<component>.<operation>`

**Standard Attributes:**

```go
// Email-specific attributes
attribute.String("mailer.to", email.To[0].Email)
attribute.String("mailer.from", email.From.Email)
attribute.String("mailer.subject", email.Subject)
attribute.Int("mailer.recipients", len(email.To))
attribute.String("mailer.template", templateName)

// Provider-specific attributes
attribute.String("mailer.provider", providerName)
attribute.String("mailer.provider.region", region)
attribute.String("mailer.message_id", messageID)

// Performance attributes
attribute.Int64("mailer.duration_ms", durationMs)
attribute.Int("mailer.retry_count", retryCount)
attribute.String("mailer.status", "sent|failed|retrying")

// Error attributes (when applicable)
attribute.String("mailer.error.type", errorType)
attribute.String("mailer.error.code", errorCode)
attribute.Bool("mailer.error.retryable", isRetryable)
```

**Provider Implementation Example:**

```go
func (p *sesProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
    span, ctx := tracer.Start(ctx, "mailer.provider.ses.send")
    defer span.End()

    span.SetAttributes(
        attribute.String("mailer.provider", "aws_ses"),
        attribute.String("mailer.provider.region", p.config.Region),
        attribute.String("mailer.to", email.To[0].Email),
    )

    // Validation span
    validationSpan, ctx := tracer.Start(ctx, "mailer.provider.ses.validate")
    if err := p.validator.ValidateEmail(email); err != nil {
        validationSpan.RecordError(err)
        validationSpan.SetStatus(codes.Error, "validation failed")
        validationSpan.End()

        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    validationSpan.SetStatus(codes.Ok, "validation passed")
    validationSpan.End()

    // AWS SES API call span
    apiSpan, ctx := tracer.Start(ctx, "mailer.provider.ses.api_call")
    input := p.buildSESInput(email)

    startTime := time.Now()
    output, err := p.client.SendEmail(ctx, input)
    duration := time.Since(startTime)

    apiSpan.SetAttributes(
        attribute.Int64("mailer.duration_ms", duration.Milliseconds()),
    )

    if err != nil {
        apiSpan.RecordError(err)
        apiSpan.SetStatus(codes.Error, err.Error())
        apiSpan.End()

        span.RecordError(err)
        span.SetStatus(codes.Error, "aws ses api call failed")
        return nil, fmt.Errorf("ses send failed: %w", err)
    }

    messageID := aws.ToString(output.MessageId)
    apiSpan.SetAttributes(
        attribute.String("mailer.message_id", messageID),
    )
    apiSpan.SetStatus(codes.Ok, "aws ses api call successful")
    apiSpan.End()

    // Set success attributes on main span
    span.SetAttributes(
        attribute.String("mailer.message_id", messageID),
        attribute.String("mailer.status", "sent"),
        attribute.Int64("mailer.total_duration_ms", duration.Milliseconds()),
    )
    span.SetStatus(codes.Ok, "email sent successfully")

    return &SendResult{
        MessageID: messageID,
        Provider:  p.Name(),
    }, nil
}
```

**Template Rendering Tracing:**

```go
func (t *TemplateEngine) Render(ctx context.Context, templateName string, data interface{}) (string, error) {
    span, ctx := tracer.Start(ctx, "mailer.template.render")
    defer span.End()

    span.SetAttributes(
        attribute.String("mailer.template.name", templateName),
        attribute.String("mailer.template.engine", "html/template"),
    )

    startTime := time.Now()
    result, err := t.renderTemplate(templateName, data)
    duration := time.Since(startTime)

    span.SetAttributes(
        attribute.Int64("mailer.template.duration_ms", duration.Milliseconds()),
        attribute.Int("mailer.template.output_size", len(result)),
    )

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return "", fmt.Errorf("template render failed: %w", err)
    }

    span.SetStatus(codes.Ok, "template rendered successfully")
    return result, nil
}
```

**Batch Operation Tracing:**

```go
func (c *Client) SendBatch(ctx context.Context, emails []*Email) error {
    span, ctx := tracer.Start(ctx, "mailer.Client.SendBatch")
    defer span.End()

    span.SetAttributes(
        attribute.Int("mailer.batch.size", len(emails)),
        attribute.String("mailer.provider", c.provider.Name()),
    )

    var successCount, failureCount int

    for i, email := range emails {
        // Create child span for each email
        emailSpan, emailCtx := tracer.Start(ctx, "mailer.Client.SendBatch.email",
            trace.WithAttributes(
                attribute.Int("mailer.batch.index", i),
                attribute.String("mailer.to", email.To[0].Email),
            ),
        )

        if err := c.Send(emailCtx, email); err != nil {
            emailSpan.RecordError(err)
            emailSpan.SetStatus(codes.Error, err.Error())
            failureCount++
        } else {
            emailSpan.SetStatus(codes.Ok, "email sent")
            successCount++
        }
        emailSpan.End()
    }

    // Set batch results
    span.SetAttributes(
        attribute.Int("mailer.batch.success_count", successCount),
        attribute.Int("mailer.batch.failure_count", failureCount),
        attribute.Float64("mailer.batch.success_rate", float64(successCount)/float64(len(emails))),
    )

    if failureCount > 0 {
        span.SetStatus(codes.Error, fmt.Sprintf("%d/%d emails failed", failureCount, len(emails)))
        return fmt.Errorf("batch send completed with %d failures", failureCount)
    }

    span.SetStatus(codes.Ok, "batch send completed successfully")
    return nil
}
```

**Configuration for Tracing:**

```go
type TracingConfig struct {
    Enabled        bool
    ServiceName    string
    ServiceVersion string
    Endpoint       string // OTLP endpoint
    SampleRate     float64
    Headers        map[string]string
}

func DefaultTracingConfig() TracingConfig {
    return TracingConfig{
        Enabled:        true,
        ServiceName:    "mailer",
        ServiceVersion: "1.0.0",
        SampleRate:     1.0, // 100% sampling for development
    }
}
```

### Metrics

- Send success/failure rates by provider
- Provider response times (P50, P95, P99)
- Template rendering times
- Queue depths and processing times
- Error rates by type and provider
- Retry attempt counts and success rates

### Logging

- Structured logging with configurable levels
- Provider-specific log events with correlation IDs
- Template rendering events and errors
- Error context preservation with stack traces
- Request/response logging (sanitized)

## Migration & Compatibility

### Versioning Strategy

- Semantic versioning (SemVer)
- Backward compatibility within major versions
- Clear migration guides for major version upgrades
- Deprecation notices with alternatives

### API Evolution

- Additive changes only in minor versions
- Optional parameters using functional options
- Interface extension through embedding
- Provider plugin system for extensibility

This technical documentation serves as the foundation for implementing the **mailer** library following Go best practices and the established development guidelines.
