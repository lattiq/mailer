package mailer

import (
	"time"
)

// Option is a functional option for configuring the mailer client.
type Option func(*Config)

// WithProvider sets the email provider type and its settings.
func WithProvider(providerType ProviderType, settings ProviderSettings) Option {
	return func(c *Config) {
		c.Provider.Type = providerType
		c.Provider.Primary = settings
	}
}

// WithFallbackProvider sets a fallback provider for redundancy.
func WithFallbackProvider(providerType ProviderType, settings ProviderSettings) Option {
	return func(c *Config) {
		fallbackSettings := settings
		fallbackSettings["type"] = string(providerType)
		c.Provider.Fallback = &fallbackSettings
	}
}

// WithTimeout sets the provider operation timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Provider.Timeout = timeout
	}
}

// WithMaxConnsPerHost sets the maximum number of connections per host.
func WithMaxConnsPerHost(maxConns int) Option {
	return func(c *Config) {
		c.Provider.MaxConnsPerHost = maxConns
	}
}

// WithTemplates enables template functionality and sets the template directory.
func WithTemplates(directory string) Option {
	return func(c *Config) {
		c.Templates.Enabled = true
		c.Templates.Directory = directory
	}
}

// WithTemplateCache configures template caching.
func WithTemplateCache(enabled bool, cacheSize int) Option {
	return func(c *Config) {
		c.Templates.CacheEnabled = enabled
		c.Templates.CacheSize = cacheSize
	}
}

// WithTemplateAutoReload enables automatic template reloading for development.
func WithTemplateAutoReload(enabled bool) Option {
	return func(c *Config) {
		c.Templates.AutoReload = enabled
	}
}

// WithRetry configures retry behavior.
func WithRetry(maxAttempts int, initialDelay, maxDelay time.Duration, multiplier float64) Option {
	return func(c *Config) {
		c.Retry.Enabled = true
		c.Retry.MaxAttempts = maxAttempts
		c.Retry.InitialDelay = initialDelay
		c.Retry.MaxDelay = maxDelay
		c.Retry.Multiplier = multiplier
	}
}

// WithJitter enables or disables jitter in retry delays.
func WithJitter(enabled bool) Option {
	return func(c *Config) {
		c.Retry.Jitter = enabled
	}
}

// WithoutRetry disables retry functionality.
func WithoutRetry() Option {
	return func(c *Config) {
		c.Retry.Enabled = false
	}
}

// WithRateLimit configures rate limiting.
func WithRateLimit(rate int, period time.Duration, burst int) Option {
	return func(c *Config) {
		c.RateLimit.Enabled = true
		c.RateLimit.Rate = rate
		c.RateLimit.Period = period
		c.RateLimit.Burst = burst
	}
}

// WithPerRecipientRateLimit enables per-recipient rate limiting.
func WithPerRecipientRateLimit(enabled bool) Option {
	return func(c *Config) {
		c.RateLimit.PerRecipient = enabled
	}
}

// WithCircuitBreaker configures circuit breaker behavior.
func WithCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) Option {
	return func(c *Config) {
		c.CircuitBreaker.Enabled = true
		c.CircuitBreaker.FailureThreshold = failureThreshold
		c.CircuitBreaker.SuccessThreshold = successThreshold
		c.CircuitBreaker.Timeout = timeout
	}
}

// WithoutCircuitBreaker disables circuit breaker functionality.
func WithoutCircuitBreaker() Option {
	return func(c *Config) {
		c.CircuitBreaker.Enabled = false
	}
}

// WithTracing configures distributed tracing.
func WithTracing(serviceName, serviceVersion string, sampleRate float64) Option {
	return func(c *Config) {
		c.Monitoring.Tracing.Enabled = true
		c.Monitoring.Tracing.ServiceName = serviceName
		c.Monitoring.Tracing.ServiceVersion = serviceVersion
		c.Monitoring.Tracing.SampleRate = sampleRate
	}
}

// WithTracingEndpoint sets the OTLP endpoint for traces.
func WithTracingEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.Monitoring.Tracing.Endpoint = endpoint
	}
}

// WithoutTracing disables distributed tracing.
func WithoutTracing() Option {
	return func(c *Config) {
		c.Monitoring.Tracing.Enabled = false
	}
}

// WithMetrics configures metrics collection.
func WithMetrics(namespace string, interval time.Duration) Option {
	return func(c *Config) {
		c.Monitoring.Metrics.Enabled = true
		c.Monitoring.Metrics.Namespace = namespace
		c.Monitoring.Metrics.Interval = interval
	}
}

// WithMetricsEndpoint sets the metrics collection endpoint.
func WithMetricsEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.Monitoring.Metrics.Endpoint = endpoint
	}
}

// WithoutMetrics disables metrics collection.
func WithoutMetrics() Option {
	return func(c *Config) {
		c.Monitoring.Metrics.Enabled = false
	}
}

// WithLogging configures logging.
func WithLogging(level, format, output string) Option {
	return func(c *Config) {
		c.Monitoring.Logging.Level = level
		c.Monitoring.Logging.Format = format
		c.Monitoring.Logging.Output = output
	}
}

// WithRequestResponseLogging enables logging of request/response data.
// Use with caution as this may log sensitive information.
func WithRequestResponseLogging(enabled bool) Option {
	return func(c *Config) {
		c.Monitoring.Logging.IncludeRequestResponse = enabled
	}
}

// WithAWSSES creates an AWS SES provider configuration.
func WithAWSSES(region string) Option {
	return WithProvider(ProviderAWSSES, ProviderSettings{
		"region": region,
	})
}

// WithAWSSESCredentials creates an AWS SES provider configuration with explicit credentials.
func WithAWSSESCredentials(region, accessKey, secretKey string) Option {
	return WithProvider(ProviderAWSSES, ProviderSettings{
		"region":     region,
		"access_key": accessKey,
		"secret_key": secretKey,
	})
}

// WithSendGrid creates a SendGrid provider configuration.
func WithSendGrid(apiKey string) Option {
	return WithProvider(ProviderSendGrid, ProviderSettings{
		"api_key": apiKey,
	})
}

// WithMailgun creates a Mailgun provider configuration.
func WithMailgun(apiKey, domain string) Option {
	return WithProvider(ProviderMailgun, ProviderSettings{
		"api_key": apiKey,
		"domain":  domain,
	})
}

// WithMailgunEU creates a Mailgun provider configuration for EU region.
func WithMailgunEU(apiKey, domain string) Option {
	return WithProvider(ProviderMailgun, ProviderSettings{
		"api_key":  apiKey,
		"domain":   domain,
		"base_url": "https://api.eu.mailgun.net",
	})
}

// WithSMTP creates an SMTP provider configuration.
func WithSMTP(host, port string) Option {
	return WithProvider(ProviderSMTP, ProviderSettings{
		"host": host,
		"port": port,
	})
}

// WithSMTPAuth creates an SMTP provider configuration with authentication.
func WithSMTPAuth(host, port, username, password string) Option {
	return WithProvider(ProviderSMTP, ProviderSettings{
		"host":     host,
		"port":     port,
		"username": username,
		"password": password,
	})
}

// WithSMTPTLS creates an SMTP provider configuration with TLS enabled.
func WithSMTPTLS(host, port, username, password string, skipVerify bool) Option {
	return WithProvider(ProviderSMTP, ProviderSettings{
		"host":     host,
		"port":     port,
		"username": username,
		"password": password,
		"tls":      "true",
		"tls_skip_verify": func() string {
			if skipVerify {
				return "true"
			}
			return "false"
		}(),
	})
}
