package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Provider defines the interface for email service providers.
// Implementations handle provider-specific logic for sending emails.
type Provider interface {
	// Send sends a single email using the provider's API.
	Send(ctx context.Context, email *Email) (*SendResult, error)

	// SendBatch sends multiple emails using the provider's batch API if available.
	// If batch API is not available, it should send emails individually.
	SendBatch(ctx context.Context, emails []*Email) (*BatchResult, error)

	// ValidateConfig validates the provider configuration.
	// Returns an error if the configuration is invalid or incomplete.
	ValidateConfig() error

	// Name returns the provider's name for identification and logging.
	Name() string
}

// ProviderSettings represents configuration settings for email providers.
type ProviderSettings map[string]string

// Get retrieves a configuration value by key.
func (ps ProviderSettings) Get(key string) string {
	return ps[key]
}

// Set sets a configuration value.
func (ps ProviderSettings) Set(key, value string) {
	ps[key] = value
}

// Address represents an email address with optional display name.
type Address struct {
	Name  string `json:"name"`  // Display name (optional)
	Email string `json:"email"` // Email address (required)
}

// String returns the formatted email address.
// If Name is provided, returns "Name <email@domain.com>"
// Otherwise returns just "email@domain.com"
func (a Address) String() string {
	if a.Name != "" {
		return mime.QEncoding.Encode("UTF-8", a.Name) + " <" + a.Email + ">"
	}
	return a.Email
}

// Valid checks if the address has a valid email format.
func (a Address) Valid() bool {
	if a.Email == "" {
		return false
	}
	_, err := mail.ParseAddress(a.String())
	return err == nil
}

// Attachment represents a file attachment to be included with the email.
type Attachment struct {
	// Filename is the name of the file as it will appear in the email.
	Filename string

	// ContentType is the MIME content type of the file.
	// If empty, it will be detected from the filename extension.
	ContentType string

	// Data contains the file content.
	Data io.Reader

	// Size is the size of the attachment in bytes (optional).
	// If provided, it may be used for validation and progress tracking.
	Size int64

	// Inline indicates whether the attachment should be displayed inline.
	// Inline attachments can be referenced in HTML content using cid:<ContentID>.
	Inline bool

	// ContentID is used for inline attachments to reference them in HTML.
	// Only used when Inline is true.
	ContentID string
}

// DetectContentType attempts to detect the content type from the filename.
func (a *Attachment) DetectContentType() string {
	if a.ContentType != "" {
		return a.ContentType
	}

	ext := strings.ToLower(filepath.Ext(a.Filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".csv":
		return "text/csv"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// Email represents an email message.
type Email struct {
	From        Address           `json:"from"`        // Sender address
	To          []Address         `json:"to"`          // Primary recipients
	CC          []Address         `json:"cc"`          // Carbon copy recipients
	BCC         []Address         `json:"bcc"`         // Blind carbon copy recipients
	Subject     string            `json:"subject"`     // Email subject
	HTMLBody    string            `json:"html_body"`   // HTML body content
	TextBody    string            `json:"text_body"`   // Plain text body content
	Attachments []Attachment      `json:"attachments"` // File attachments
	Headers     map[string]string `json:"headers"`     // Custom headers
	Priority    Priority          `json:"priority"`    // Email priority
	Metadata    map[string]string `json:"metadata"`    // Provider-specific metadata
}

// Validate checks if the email has valid structure and required fields.
func (e *Email) Validate() error {
	if !e.From.Valid() {
		return &ValidationError{Field: "from", Message: "invalid or missing sender address"}
	}

	if len(e.To) == 0 {
		return &ValidationError{Field: "to", Message: "at least one recipient required"}
	}

	for i, to := range e.To {
		if !to.Valid() {
			return &ValidationError{
				Field:   "to",
				Message: "invalid recipient address at index " + strconv.Itoa(i),
			}
		}
	}

	for i, cc := range e.CC {
		if !cc.Valid() {
			return &ValidationError{
				Field:   "cc",
				Message: "invalid CC address at index " + strconv.Itoa(i),
			}
		}
	}

	for i, bcc := range e.BCC {
		if !bcc.Valid() {
			return &ValidationError{
				Field:   "bcc",
				Message: "invalid BCC address at index " + strconv.Itoa(i),
			}
		}
	}

	if strings.TrimSpace(e.Subject) == "" {
		return &ValidationError{Field: "subject", Message: "subject is required"}
	}

	if strings.TrimSpace(e.TextBody) == "" && strings.TrimSpace(e.HTMLBody) == "" {
		return &ValidationError{Field: "body", Message: "either text or HTML body is required"}
	}

	return nil
}

// HasAttachments returns true if the email has any attachments.
func (e *Email) HasAttachments() bool {
	return len(e.Attachments) > 0
}

// HasInlineAttachments returns true if the email has any inline attachments.
func (e *Email) HasInlineAttachments() bool {
	for _, att := range e.Attachments {
		if att.Inline {
			return true
		}
	}
	return false
}

// TotalRecipients returns the total number of recipients (To + CC + BCC).
func (e *Email) TotalRecipients() int {
	return len(e.To) + len(e.CC) + len(e.BCC)
}

// AllRecipients returns all recipients combined into a single slice.
func (e *Email) AllRecipients() []Address {
	all := make([]Address, 0, e.TotalRecipients())
	all = append(all, e.To...)
	all = append(all, e.CC...)
	all = append(all, e.BCC...)
	return all
}

// TemplateRequest represents a request to send an email using a template.
type TemplateRequest struct {
	// Template is the name of the template to use.
	Template string

	// To contains the recipients for this email.
	To []Address

	// From is the sender's address.
	From Address

	// CC contains carbon copy recipients (optional).
	CC []Address

	// BCC contains blind carbon copy recipients (optional).
	BCC []Address

	// Subject is the email subject. If empty, the template should provide it.
	Subject string

	// Data contains the data to be merged with the template.
	Data interface{}

	// Options provides additional template rendering options.
	Options *TemplateOptions

	// Priority indicates the email priority level.
	Priority Priority

	// Headers contains custom email headers.
	Headers map[string]string

	// Metadata contains arbitrary data for tracking and analytics.
	Metadata map[string]interface{}
}

// TemplateOptions provides additional options for template rendering.
type TemplateOptions struct {
	// Locale specifies the locale for internationalization (optional).
	Locale string

	// Timezone specifies the timezone for date/time formatting (optional).
	Timezone string

	// Partials contains additional template partials that can be included.
	Partials map[string]string

	// Helpers contains custom template helper functions.
	Helpers map[string]interface{}
}

// Priority defines the priority level of an email.
type Priority int

const (
	// PriorityLow indicates low priority email (marketing, newsletters).
	PriorityLow Priority = iota

	// PriorityNormal indicates normal priority email (default).
	PriorityNormal

	// PriorityHigh indicates high priority email (alerts, notifications).
	PriorityHigh

	// PriorityUrgent indicates urgent email (security alerts, critical notifications).
	PriorityUrgent
)

// String returns the string representation of the priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

// SendResult contains the result of sending a single email.
type SendResult struct {
	// MessageID is the unique identifier assigned by the provider.
	MessageID string

	// Provider is the name of the provider that sent the email.
	Provider string

	// Timestamp when the email was accepted by the provider.
	Timestamp time.Time

	// Metadata contains provider-specific information.
	Metadata map[string]interface{}
}

// BatchResult contains the results of sending multiple emails.
type BatchResult struct {
	// Total number of emails that were attempted.
	Total int

	// Successful contains results for successfully sent emails.
	Successful []*SendResult

	// Failed contains errors for emails that failed to send.
	Failed []BatchFailure

	// Provider is the name of the provider used for the batch.
	Provider string
}

// BatchFailure represents a failed email in a batch operation.
type BatchFailure struct {
	// Index is the position of the failed email in the original batch.
	Index int

	// Email is the email that failed to send.
	Email *Email

	// Error is the reason for the failure.
	Error error
}

// ValidationError represents a validation error with specific field information.
type ValidationError struct {
	// Field is the name of the field that failed validation.
	Field string

	// Message is the validation error message.
	Message string

	// Value is the invalid value (optional).
	Value interface{}
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation error in %s: %s (value: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// Is implements error matching for errors.Is.
func (e *ValidationError) Is(target error) bool {
	_, ok := target.(*ValidationError)
	return ok
}

// ProviderError represents an error from an email provider.
type ProviderError struct {
	// Provider is the name of the provider that generated the error.
	Provider string

	// Code is the provider-specific error code.
	Code string

	// Message is the error message from the provider.
	Message string

	// StatusCode is the HTTP status code (for HTTP-based providers).
	StatusCode int

	// IsRetryable indicates whether the error can be retried.
	IsRetryable bool

	// IsTemporary indicates whether the error is temporary.
	IsTemporary bool

	// Cause is the underlying error that caused this provider error.
	Cause error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("provider %s error [%s] (status: %d): %s",
			e.Provider, e.Code, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("provider %s error [%s]: %s", e.Provider, e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// Is implements error matching for errors.Is.
func (e *ProviderError) Is(target error) bool {
	pe, ok := target.(*ProviderError)
	if !ok {
		return false
	}
	return e.Provider == pe.Provider && e.Code == pe.Code
}

// Retryable implements RetryableError for ProviderError.
func (e *ProviderError) Retryable() bool {
	return e.IsRetryable
}

// Temporary implements TemporaryError for ProviderError.
func (e *ProviderError) Temporary() bool {
	return e.IsTemporary
}

// RetryableError interface indicates whether an error can be retried.
type RetryableError interface {
	Retryable() bool
}

// TemporaryError interface indicates whether an error is temporary.
type TemporaryError interface {
	Temporary() bool
}

// Constructor functions for errors

// NewProviderError creates a new provider error.
func NewProviderError(provider, code, message string) *ProviderError {
	return &ProviderError{
		Provider:    provider,
		Code:        code,
		Message:     message,
		IsRetryable: false,
		IsTemporary: false,
	}
}

// NewRetryableProviderError creates a new retryable provider error.
func NewRetryableProviderError(provider, code, message string) *ProviderError {
	return &ProviderError{
		Provider:    provider,
		Code:        code,
		Message:     message,
		IsRetryable: true,
		IsTemporary: false,
	}
}

// NewTemporaryProviderError creates a new temporary provider error.
func NewTemporaryProviderError(provider, code, message string) *ProviderError {
	return &ProviderError{
		Provider:    provider,
		Code:        code,
		Message:     message,
		IsRetryable: true,
		IsTemporary: true,
	}
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithValue creates a new validation error with a value.
func NewValidationErrorWithValue(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements RetryableError interface
	if re, ok := err.(RetryableError); ok {
		return re.Retryable()
	}

	// Check for specific error types that are retryable
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.IsRetryable
	}

	return false
}

// IsTemporary checks if an error is temporary.
func IsTemporary(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements TemporaryError interface
	if te, ok := err.(TemporaryError); ok {
		return te.Temporary()
	}

	// Check for specific error types that are temporary
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.IsTemporary
	}

	return false
}

// GetRetryAfter extracts retry delay from an error if available.
func GetRetryAfter(err error) time.Duration {
	if err == nil {
		return 0
	}

	// Check if the error implements a RetryAfter interface
	if rateLimited, ok := err.(interface{ RetryAfter() time.Duration }); ok {
		return rateLimited.RetryAfter()
	}

	return 0
}
