// Package mailer provides a high-performance, provider-agnostic Go library for sending emails
// with a focus on transactional email use cases.
//
// The library provides a clean, idiomatic Go API with support for multiple email providers,
// template management, and built-in reliability features including retries, rate limiting,
// and circuit breaker patterns.
//
// # Basic Usage
//
//	config := mailer.Config{
//		Provider: mailer.ProviderConfig{
//			Type: mailer.ProviderAWSSES,
//			Primary: mailer.ProviderSettings{
//				"region": "us-east-1",
//			},
//		},
//	}
//
//	client, err := mailer.New(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	email := &mailer.Email{
//		From:     mailer.Address{Email: "noreply@example.com"},
//		To:       []mailer.Address{{Email: "user@example.com"}},
//		Subject:  "Welcome",
//		HTMLBody: "<h1>Welcome!</h1>",
//		TextBody: "Welcome!",
//	}
//
//	err = client.Send(context.Background(), email)
//
// # Supported Providers
//
//   - AWS SES
//   - SendGrid
//   - Mailgun
//   - Generic SMTP
//
// # Features
//
//   - Provider-agnostic interface
//   - Template management with HTML/text templates
//   - Automatic retries with exponential backoff
//   - Rate limiting and circuit breaker
//   - Batch operations
//   - Distributed tracing with OpenTelemetry
//   - Context-aware operations
//   - Thread-safe operations
package mailer
