package mailer

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"sync"
	"time"
)

// RetryManager handles retry logic for failed operations.
type RetryManager struct {
	config RetryConfig
}

// NewRetryManager creates a new retry manager with the given configuration.
func NewRetryManager(config RetryConfig) *RetryManager {
	return &RetryManager{
		config: config,
	}
}

// Retry executes the given function with retry logic.
func (r *RetryManager) Retry(ctx context.Context, fn func() error) error {
	if !r.config.Enabled {
		return fn()
	}

	var lastErr error
	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay for next attempt
		delay := r.calculateDelay(attempt)

		// Check if we should retry after rate limit
		if retryAfter := GetRetryAfter(err); retryAfter > 0 {
			delay = retryAfter
		}

		// Wait for the delay or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateDelay calculates the delay for the given attempt number.
func (r *RetryManager) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff delay
	delay := time.Duration(float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1)))

	// Cap the delay at MaxDelay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	// Add jitter if enabled
	if r.config.Jitter {
		// Add up to 10% jitter using cryptographically secure random
		jitterRange := float64(delay) * 0.1
		maxJitter := int64(jitterRange)
		if maxJitter > 0 {
			jitterBig, err := rand.Int(rand.Reader, big.NewInt(maxJitter))
			if err == nil {
				jitter := time.Duration(jitterBig.Int64())
				delay += jitter
			}
		}
	}

	return delay
}

// RateLimiter provides rate limiting functionality.
type RateLimiter struct {
	config     RateLimitConfig
	tokens     chan struct{}
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:     config,
		tokens:     make(chan struct{}, config.Burst),
		lastRefill: time.Now(),
	}

	// Fill initial tokens
	for i := 0; i < config.Burst; i++ {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Channel is full, stop filling
			goto tokensFilled
		}
	}
tokensFilled:

	// Start token refill goroutine
	go rl.refillTokens()

	return rl
}

// Wait waits until the rate limit allows the operation to proceed.
func (rl *RateLimiter) Wait(ctx context.Context, email *Email) error {
	if !rl.config.Enabled {
		return nil
	}

	// For per-recipient rate limiting, we need to account for all recipients
	tokensNeeded := 1
	if rl.config.PerRecipient {
		tokensNeeded = email.TotalRecipients()
	}

	// Acquire the needed tokens
	for i := 0; i < tokensNeeded; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rl.tokens:
			// Token acquired, continue
		default:
			// No tokens available, calculate retry after
			retryAfter := rl.config.Period / time.Duration(rl.config.Rate)
			return NewRateLimitError("rate limit exceeded", retryAfter)
		}
	}

	return nil
}

// refillTokens periodically refills the token bucket.
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(rl.config.Period / time.Duration(rl.config.Rate))
	defer ticker.Stop()

	for range ticker.C {
		select {
		case rl.tokens <- struct{}{}:
			// Token added
		default:
			// Bucket is full
		}
	}
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitBreakerClosed indicates the circuit breaker is closed (normal operation).
	CircuitBreakerClosed CircuitBreakerState = iota

	// CircuitBreakerOpen indicates the circuit breaker is open (blocking requests).
	CircuitBreakerOpen

	// CircuitBreakerHalfOpen indicates the circuit breaker is half-open (testing recovery).
	CircuitBreakerHalfOpen
)

// String returns the string representation of the circuit breaker state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
type CircuitBreaker struct {
	config       CircuitBreakerConfig
	state        CircuitBreakerState
	failureCount int
	successCount int
	lastFailTime time.Time
	mutex        sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
	}
}

// Execute executes the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.config.Enabled {
		return fn()
	}

	// Check if we can execute
	if !cb.canExecute() {
		return ErrCircuitBreakerOpen
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.recordResult(err)

	return err
}

// canExecute checks if the circuit breaker allows execution.
func (cb *CircuitBreaker) canExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if enough time has passed to try half-open
		if time.Since(cb.lastFailTime) >= cb.config.Timeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			// Double check after acquiring write lock
			if cb.state == CircuitBreakerOpen && time.Since(cb.lastFailTime) >= cb.config.Timeout {
				cb.state = CircuitBreakerHalfOpen
				cb.successCount = 0
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return cb.state == CircuitBreakerHalfOpen
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an operation.
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailTime = time.Now()

		// Check if we should open the circuit
		if cb.state == CircuitBreakerClosed && cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitBreakerOpen
		} else if cb.state == CircuitBreakerHalfOpen {
			// Return to open state on failure in half-open
			cb.state = CircuitBreakerOpen
		}
	} else {
		cb.successCount++

		// Check if we should close the circuit
		if cb.state == CircuitBreakerHalfOpen && cb.successCount >= cb.config.SuccessThreshold {
			cb.state = CircuitBreakerClosed
			cb.failureCount = 0
		}

		// Reset failure count after successful operations in closed state
		if cb.state == CircuitBreakerClosed && time.Since(cb.lastFailTime) >= cb.config.ResetTimeout {
			cb.failureCount = 0
		}
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// FailureCount returns the current failure count.
func (cb *CircuitBreaker) FailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}

// SuccessCount returns the current success count.
func (cb *CircuitBreaker) SuccessCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.successCount
}
