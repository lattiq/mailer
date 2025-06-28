package mailer

import (
	"errors"
	"fmt"
	"time"
)

// Predefined sentinel errors for common cases.
var (
	// ErrInvalidEmail indicates an invalid email address format.
	ErrInvalidEmail = errors.New("invalid email address")

	// ErrTemplateNotFound indicates a requested template was not found.
	ErrTemplateNotFound = errors.New("template not found")

	// ErrProviderTimeout indicates a provider operation timed out.
	ErrProviderTimeout = errors.New("provider timeout")

	// ErrRateLimited indicates the operation was rate limited.
	ErrRateLimited = errors.New("rate limited")

	// ErrCircuitBreakerOpen indicates the circuit breaker is open.
	ErrCircuitBreakerOpen = errors.New("circuit breaker open")

	// ErrProviderUnavailable indicates the provider is unavailable.
	ErrProviderUnavailable = errors.New("provider unavailable")

	// ErrInvalidConfiguration indicates invalid configuration.
	ErrInvalidConfiguration = errors.New("invalid configuration")

	// ErrClientClosed indicates the client has been closed.
	ErrClientClosed = errors.New("client closed")
)

// TemplateError represents an error in template processing.
type TemplateError struct {
	// Template is the name of the template that caused the error.
	Template string

	// Operation is the operation that failed (e.g., "parse", "render").
	Operation string

	// Message is the error message.
	Message string

	// Cause is the underlying error.
	Cause error
}

// Error implements the error interface.
func (e *TemplateError) Error() string {
	return fmt.Sprintf("template error in %s during %s: %s", e.Template, e.Operation, e.Message)
}

// Unwrap returns the underlying error.
func (e *TemplateError) Unwrap() error {
	return e.Cause
}

// RateLimitError represents a rate limiting error with retry information.
type RateLimitError struct {
	// Message is the error message.
	Message string

	// RetryAfterDuration indicates when the operation can be retried.
	RetryAfterDuration time.Duration

	// Limit is the rate limit that was exceeded.
	Limit int

	// Window is the time window for the rate limit.
	Window time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited: %s (retry after %v)", e.Message, e.RetryAfterDuration)
}

// BatchError represents errors that occurred during batch operations.
type BatchError struct {
	// Message is the overall error message.
	Message string

	// Errors contains individual errors for each failed item.
	Errors []BatchItemError

	// Total is the total number of items in the batch.
	Total int

	// Failed is the number of items that failed.
	Failed int
}

// Error implements the error interface.
func (e *BatchError) Error() string {
	return fmt.Sprintf("batch error: %s (%d/%d failed)", e.Message, e.Failed, e.Total)
}

// BatchItemError represents an error for a specific item in a batch.
type BatchItemError struct {
	// Index is the position of the item in the batch.
	Index int

	// Error is the error that occurred for this item.
	Error error
}

// RetryableError interface indicates whether an error can be retried.
type RetryableError interface {
	Retryable() bool
}

// TemporaryError interface indicates whether an error is temporary.
type TemporaryError interface {
	Temporary() bool
}

// RateLimitErrorInterface provides rate limit information.
type RateLimitErrorInterface interface {
	RetryAfter() time.Duration
}

// NewTemplateError creates a new template error.
func NewTemplateError(template, operation, message string, cause error) *TemplateError {
	return &TemplateError{
		Template:  template,
		Operation: operation,
		Message:   message,
		Cause:     cause,
	}
}

// NewRateLimitError creates a new rate limit error.
func NewRateLimitError(message string, retryAfter time.Duration) *RateLimitError {
	return &RateLimitError{
		Message:            message,
		RetryAfterDuration: retryAfter,
	}
}
