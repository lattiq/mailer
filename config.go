package mailer

import (
	"time"
)

// Config holds the complete mailer configuration.
type Config struct {
	// Provider contains provider-specific configuration.
	Provider ProviderConfig

	// Templates contains template engine configuration.
	Templates TemplateConfig

	// Retry contains retry policy configuration.
	Retry RetryConfig

	// RateLimit contains rate limiting configuration.
	RateLimit RateLimitConfig

	// CircuitBreaker contains circuit breaker configuration.
	CircuitBreaker CircuitBreakerConfig

	// Monitoring contains observability configuration.
	Monitoring MonitoringConfig
}

// ProviderConfig contains provider-specific settings.
type ProviderConfig struct {
	// Type specifies the email provider to use.
	Type ProviderType

	// Primary contains settings for the primary provider.
	Primary ProviderSettings

	// Fallback contains settings for the fallback provider (optional).
	// If specified, the mailer will attempt to use this provider if the primary fails.
	Fallback *ProviderSettings

	// Timeout is the maximum time to wait for provider operations.
	Timeout time.Duration

	// MaxConnsPerHost limits the number of connections per host for HTTP-based providers.
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum time an idle connection will remain open.
	IdleConnTimeout time.Duration
}

// ProviderType represents the type of email provider.
type ProviderType string

const (
	// ProviderAWSSES represents Amazon Simple Email Service.
	ProviderAWSSES ProviderType = "aws_ses"

	// ProviderSendGrid represents the SendGrid email service.
	ProviderSendGrid ProviderType = "sendgrid"

	// ProviderMailgun represents the Mailgun email service.
	ProviderMailgun ProviderType = "mailgun"

	// ProviderSMTP represents a generic SMTP server.
	ProviderSMTP ProviderType = "smtp"
)

// String returns the string representation of the provider type.
func (pt ProviderType) String() string {
	return string(pt)
}

// Valid checks if the provider type is supported.
func (pt ProviderType) Valid() bool {
	switch pt {
	case ProviderAWSSES, ProviderSendGrid, ProviderMailgun, ProviderSMTP:
		return true
	default:
		return false
	}
}

// TemplateConfig contains template engine configuration.
type TemplateConfig struct {
	// Enabled indicates whether template functionality is enabled.
	Enabled bool

	// Directory is the path to the directory containing email templates.
	Directory string

	// Extension is the file extension for template files (default: ".html", ".txt").
	Extension []string

	// CacheEnabled indicates whether parsed templates should be cached.
	CacheEnabled bool

	// CacheSize is the maximum number of templates to cache (default: 100).
	CacheSize int

	// AutoReload indicates whether templates should be automatically reloaded
	// when files change (useful for development).
	AutoReload bool

	// AllowUnsafeFunctions enables unsafe template functions that bypass auto-escaping.
	// WARNING: Only enable this if you trust all template content completely.
	// These functions can lead to XSS vulnerabilities if misused.
	AllowUnsafeFunctions bool
}

// RetryConfig contains retry policy configuration.
type RetryConfig struct {
	// Enabled indicates whether retries are enabled.
	Enabled bool

	// MaxAttempts is the maximum number of retry attempts (including the initial attempt).
	MaxAttempts int

	// InitialDelay is the initial delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (should be > 1.0 for exponential backoff).
	Multiplier float64

	// Jitter indicates whether random jitter should be added to delays.
	Jitter bool

	// RetryableErrors specifies which error types should be retried.
	// If empty, all errors marked as retryable will be retried.
	RetryableErrors []string
}

// RateLimitConfig contains rate limiting configuration.
type RateLimitConfig struct {
	// Enabled indicates whether rate limiting is enabled.
	Enabled bool

	// Rate is the number of requests per period.
	Rate int

	// Period is the time period for the rate limit.
	Period time.Duration

	// Burst is the maximum number of requests that can be made immediately.
	Burst int

	// PerRecipient indicates whether rate limiting should be applied per recipient.
	// If false, rate limiting is applied globally.
	PerRecipient bool
}

// CircuitBreakerConfig contains circuit breaker configuration.
type CircuitBreakerConfig struct {
	// Enabled indicates whether the circuit breaker is enabled.
	Enabled bool

	// FailureThreshold is the number of failures that triggers the circuit breaker.
	FailureThreshold int

	// SuccessThreshold is the number of successes needed to close the circuit.
	SuccessThreshold int

	// Timeout is how long the circuit breaker waits before attempting to recover.
	Timeout time.Duration

	// ResetTimeout is how long to wait before resetting failure counts.
	ResetTimeout time.Duration
}

// MonitoringConfig contains observability configuration.
type MonitoringConfig struct {
	// Tracing contains distributed tracing configuration.
	Tracing TracingConfig

	// Metrics contains metrics collection configuration.
	Metrics MetricsConfig

	// Logging contains logging configuration.
	Logging LoggingConfig
}

// TracingConfig contains distributed tracing configuration.
type TracingConfig struct {
	// Enabled indicates whether tracing is enabled.
	Enabled bool

	// ServiceName is the service name to use in traces.
	ServiceName string

	// ServiceVersion is the service version to use in traces.
	ServiceVersion string

	// Endpoint is the OTLP endpoint for sending traces.
	Endpoint string

	// SampleRate is the sampling rate (0.0 to 1.0).
	SampleRate float64

	// Headers contains additional headers to send with traces.
	Headers map[string]string
}

// MetricsConfig contains metrics collection configuration.
type MetricsConfig struct {
	// Enabled indicates whether metrics collection is enabled.
	Enabled bool

	// Namespace is the metrics namespace/prefix.
	Namespace string

	// Endpoint is the metrics collection endpoint.
	Endpoint string

	// Interval is how often to report metrics.
	Interval time.Duration
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	// Level is the logging level (debug, info, warn, error).
	Level string

	// Format is the log format (json, text).
	Format string

	// Output is where to write logs (stdout, stderr, or file path).
	Output string

	// IncludeRequestResponse indicates whether to log request/response data.
	// This should be used carefully as it may log sensitive information.
	IncludeRequestResponse bool
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider: ProviderConfig{
			Timeout:         30 * time.Second,
			MaxConnsPerHost: 10,
			IdleConnTimeout: 90 * time.Second,
		},
		Templates: TemplateConfig{
			Enabled:              true,
			Extension:            []string{".html", ".txt"},
			CacheEnabled:         true,
			CacheSize:            100,
			AutoReload:           false,
			AllowUnsafeFunctions: false, // Secure by default
		},
		Retry: DefaultRetryConfig(),
		RateLimit: RateLimitConfig{
			Enabled:      false,
			Rate:         100,
			Period:       time.Minute,
			Burst:        10,
			PerRecipient: false,
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          false,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     300 * time.Second,
		},
		Monitoring: MonitoringConfig{
			Tracing: TracingConfig{
				Enabled:        true,
				ServiceName:    "mailer",
				ServiceVersion: "1.0.0",
				SampleRate:     1.0,
			},
			Metrics: MetricsConfig{
				Enabled:   true,
				Namespace: "mailer",
				Interval:  30 * time.Second,
			},
			Logging: LoggingConfig{
				Level:                  "info",
				Format:                 "json",
				Output:                 "stdout",
				IncludeRequestResponse: false,
			},
		},
	}
}

// DefaultRetryConfig returns default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		Enabled:      true,
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// Validate checks if the configuration is valid and complete.
func (c *Config) Validate() error {
	if !c.Provider.Type.Valid() {
		return &ValidationError{
			Field:   "provider.type",
			Message: "invalid or unsupported provider type: " + string(c.Provider.Type),
		}
	}

	if c.Provider.Timeout <= 0 {
		return &ValidationError{
			Field:   "provider.timeout",
			Message: "timeout must be greater than 0",
		}
	}

	if c.Retry.Enabled {
		if c.Retry.MaxAttempts < 1 {
			return &ValidationError{
				Field:   "retry.max_attempts",
				Message: "max attempts must be at least 1",
			}
		}
		if c.Retry.Multiplier <= 1.0 {
			return &ValidationError{
				Field:   "retry.multiplier",
				Message: "multiplier must be greater than 1.0",
			}
		}
	}

	if c.RateLimit.Enabled {
		if c.RateLimit.Rate <= 0 {
			return &ValidationError{
				Field:   "rate_limit.rate",
				Message: "rate must be greater than 0",
			}
		}
		if c.RateLimit.Period <= 0 {
			return &ValidationError{
				Field:   "rate_limit.period",
				Message: "period must be greater than 0",
			}
		}
	}

	if c.Monitoring.Tracing.Enabled {
		if c.Monitoring.Tracing.SampleRate < 0 || c.Monitoring.Tracing.SampleRate > 1 {
			return &ValidationError{
				Field:   "monitoring.tracing.sample_rate",
				Message: "sample rate must be between 0.0 and 1.0",
			}
		}
	}

	return nil
}
