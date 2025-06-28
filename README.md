# Mailer - Go Email Library

[![Go Reference](https://pkg.go.dev/badge/github.com/lattiq/mailer.svg)](https://pkg.go.dev/github.com/lattiq/mailer)
[![Go Report Card](https://goreportcard.com/badge/github.com/lattiq/mailer)](https://goreportcard.com/report/github.com/lattiq/mailer)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**mailer** is a high-performance, provider-agnostic Go library for sending emails with a focus on transactional email use cases. It provides a clean, idiomatic Go API with support for multiple email providers, template management, and built-in reliability features.

## Features

- 🚀 **Provider Agnostic**: Support for AWS SES, SendGrid, Mailgun, and SMTP
- 📧 **Template Management**: HTML/text templates with helper functions
- 🔄 **Automatic Retries**: Exponential backoff with jitter
- 🚦 **Rate Limiting**: Configurable rate limiting per provider or recipient
- 🔌 **Circuit Breaker**: Fault tolerance with circuit breaker pattern
- 📊 **Observability**: Built-in distributed tracing with OpenTelemetry
- 🏗️ **Batch Operations**: Efficient batch email sending
- ⚡ **Context Support**: Full context.Context propagation
- 🔒 **Thread Safe**: All operations are safe for concurrent use
- 🎯 **Type Safe**: Leverages Go's type system for compile-time safety

## Installation

```bash
go get github.com/lattiq/mailer
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/lattiq/mailer"
)

func main() {
    // Create client with SMTP provider
    client, err := mailer.New(
        mailer.DefaultConfig(),
        mailer.WithSMTP("localhost", "1025"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create and send email
    email := &mailer.Email{
        From:     mailer.Address{Email: "sender@example.com", Name: "Sender"},
        To:       []mailer.Address{{Email: "recipient@example.com", Name: "Recipient"}},
        Subject:  "Welcome!",
        HTMLBody: "<h1>Welcome to our service!</h1>",
        TextBody: "Welcome to our service!",
    }

    err = client.Send(context.Background(), email)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Email sent successfully!")
}
```

## Providers

### AWS SES

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithAWSSES("us-east-1"),
)
```

With explicit credentials:

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithAWSSESCredentials("us-east-1", "access-key", "secret-key"),
)
```

### SendGrid

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithSendGrid("your-api-key"),
)
```

### Mailgun

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithMailgun("your-api-key", "your-domain.com"),
)
```

For Mailgun EU:

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithMailgunEU("your-api-key", "your-domain.com"),
)
```

### SMTP

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithSMTPAuth("smtp.gmail.com", "587", "username", "password"),
)
```

With TLS:

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithSMTPTLS("smtp.gmail.com", "465", "username", "password", false),
)
```

## Advanced Configuration

### Retry Logic

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithRetry(3, 100*time.Millisecond, 5*time.Second, 2.0),
    mailer.WithJitter(true),
)
```

### Rate Limiting

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithRateLimit(100, time.Minute, 10), // 100 emails per minute, burst of 10
    mailer.WithPerRecipientRateLimit(true),
)
```

### Circuit Breaker

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithCircuitBreaker(5, 3, 60*time.Second), // 5 failures trigger, 3 successes to close, 60s timeout
)
```

### Fallback Provider

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithAWSSES("us-east-1"),
    mailer.WithFallbackProvider(mailer.ProviderSendGrid, mailer.ProviderSettings{
        "api_key": "your-sendgrid-key",
    }),
)
```

## Template Support

### Setup Templates

```go
config := mailer.DefaultConfig()
config.Templates.Enabled = true
config.Templates.Directory = "templates"
config.Templates.Extension = []string{".html", ".text"}

client, err := mailer.New(config, mailer.WithAWSSES("us-east-1"))
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Template Structure

Templates use double extensions to specify both the template name and format:

```
templates/
├── welcome.html.html      # HTML template for welcome emails
├── welcome.text.text      # Text template for welcome emails
├── otp.html.html         # HTML template for OTP emails
└── otp.text.text         # Text template for OTP emails
```

**Important**: Template files must use double extensions (e.g., `name.html.html`, `name.text.text`) so that when the file extension is removed during loading, the templates are registered with names like `name.html` and `name.text`, which is what `SendTemplate` expects.

### Send Template Email

```go
templateRequest := &mailer.TemplateRequest{
    Template: "otp", // Template name (without extension)
    From:     mailer.Address{Email: "noreply@example.com", Name: "Example"},
    To:       []mailer.Address{{Email: "user@example.com", Name: "User"}},
    Subject:  "Your OTP Code",
    Data: map[string]interface{}{
        "UserName": "John Doe",
        "OTP":      "123456",
        "ExpiryTime": "10 minutes",
    },
    Headers: map[string]string{
        "X-Category": "authentication",
    },
}

err := client.SendTemplate(context.Background(), templateRequest)
```

## Batch Operations

```go
emails := []*mailer.Email{
    {
        From:    mailer.Address{Email: "noreply@example.com"},
        To:      []mailer.Address{{Email: "user1@example.com"}},
        Subject: "Batch Email 1",
        TextBody: "First email in batch",
    },
    {
        From:    mailer.Address{Email: "noreply@example.com"},
        To:      []mailer.Address{{Email: "user2@example.com"}},
        Subject: "Batch Email 2",
        TextBody: "Second email in batch",
    },
}

err := client.SendBatch(context.Background(), emails)
```

## Build Information

### Getting Build Information

```go
// Get build information
info := mailer.GetVersionInfo()
fmt.Printf("Version: %s\n", info.Version)
fmt.Printf("Git Commit: %s\n", info.GitCommit)
fmt.Printf("Build Date: %s\n", info.BuildDate)
fmt.Printf("Platform: %s\n", info.Platform)
```

### Building

Use the provided Makefile for common build tasks:

```bash
# Build the project
make build

# Create a release build
make release

# Build for multiple platforms
make build-all

# Run all checks and tests
make check
```

## Observability

### Distributed Tracing

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithTracing("my-service", "1.0.0", 1.0),
    mailer.WithTracingEndpoint("http://jaeger:14268/api/traces"),
)
```

### Metrics

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithMetrics("myapp_mailer", 30*time.Second),
)
```

### Logging

```go
client, err := mailer.New(
    mailer.DefaultConfig(),
    mailer.WithLogging("info", "json", "stdout"),
    mailer.WithRequestResponseLogging(false), // Be careful with sensitive data
)
```

## Error Handling

The library provides rich error types for different scenarios:

```go
err := client.Send(ctx, email)
if err != nil {
    switch {
    case mailer.IsRetryable(err):
        log.Println("Error is retryable:", err)
    case mailer.IsTemporary(err):
        log.Println("Error is temporary:", err)
    default:
        log.Println("Permanent error:", err)
    }

    // Check for specific error types
    var validationErr *mailer.ValidationError
    if errors.As(err, &validationErr) {
        log.Printf("Validation error in field %s: %s", validationErr.Field, validationErr.Message)
    }

    var providerErr *mailer.ProviderError
    if errors.As(err, &providerErr) {
        log.Printf("Provider %s error [%s]: %s", providerErr.Provider, providerErr.Code, providerErr.Message)
    }
}
```

## Email Validation

```go
email := &mailer.Email{
    From:     mailer.Address{Email: "sender@example.com"},
    To:       []mailer.Address{{Email: "recipient@example.com"}},
    Subject:  "Test",
    TextBody: "Hello, World!",
}

if err := email.Validate(); err != nil {
    log.Printf("Email validation failed: %v", err)
}
```

## Performance Considerations

- **Connection Pooling**: HTTP connections are pooled automatically
- **Memory Usage**: ~1KB per email (excluding attachments)
- **Throughput**: <10ms processing time per email (excluding network)
- **Template Caching**: Templates are cached for improved performance
- **Batch Operations**: ~1ms per email in batch operations

## Security

- **Input Validation**: All email addresses and headers are validated
- **Header Injection**: Protection against email header injection attacks
- **TLS Support**: Full TLS/SSL support for SMTP connections
- **Credential Management**: Support for environment variables and IAM roles

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Examples

For more examples, see the [examples/](examples/) directory:

- **[OTP Email with Templates](examples/otp/)** - Complete example showing how to send OTP emails using templates with AWS SES
