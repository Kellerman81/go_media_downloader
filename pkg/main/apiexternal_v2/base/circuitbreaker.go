package base

import (
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// CircuitBreaker - Prevents cascading failures
// States: Closed -> Open -> Half-Open -> Closed
//

type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig holds circuit breaker configuration.
type CircuitBreakerConfig struct {
	Threshold   int           // Number of failures to open
	Timeout     time.Duration // How long to stay open before trying half-open
	HalfOpenMax int           // Max requests in half-open state
	MaxOpenTime time.Duration // Max time to stay open before forcing reset (prevents infinite open state)
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu         sync.RWMutex
	config     CircuitBreakerConfig
	clientName string // For logging

	state            CircuitBreakerState
	failures         int
	lastFailureTime  time.Time
	firstOpenTime    time.Time // When circuit first opened (for max open time tracking)
	halfOpenSuccess  int
	halfOpenAttempts int
}

// NewCircuitBreaker creates a new circuit breaker.
func newCircuitBreaker(config CircuitBreakerConfig, clientName string) *CircuitBreaker {
	// Set defaults
	if config.Threshold == 0 {
		config.Threshold = 5
	}

	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	if config.HalfOpenMax == 0 {
		config.HalfOpenMax = 3
	}

	if config.MaxOpenTime == 0 {
		config.MaxOpenTime = 4 * time.Hour
	}

	return &CircuitBreaker{
		config:     config,
		clientName: clientName,
		state:      StateClosed,
	}
}

// CanMakeRequest checks if a request can be made.
func (cb *CircuitBreaker) CanMakeRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if max open time exceeded - force reset to closed
		if !cb.firstOpenTime.IsZero() && time.Since(cb.firstOpenTime) > cb.config.MaxOpenTime {
			openDuration := time.Since(cb.firstOpenTime)
			logger.Logtype(logger.StatusWarning, 0).
				Str("client", cb.clientName).
				Dur("open_duration", openDuration).
				Dur("max_open_time", cb.config.MaxOpenTime).
				Int("failures", cb.failures).
				Msg("Circuit breaker FORCE RESET - max open time exceeded, resetting to closed")

			cb.state = StateClosed
			cb.failures = 0
			cb.firstOpenTime = time.Time{}

			return true
		}

		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenAttempts = 0
			cb.halfOpenSuccess = 0
			return true
		}

		return false

	case StateHalfOpen:
		// Check if max open time exceeded - force reset to closed
		if !cb.firstOpenTime.IsZero() && time.Since(cb.firstOpenTime) > cb.config.MaxOpenTime {
			openDuration := time.Since(cb.firstOpenTime)
			logger.Logtype(logger.StatusWarning, 0).
				Str("client", cb.clientName).
				Dur("open_duration", openDuration).
				Dur("max_open_time", cb.config.MaxOpenTime).
				Int("failures", cb.failures).
				Msg("Circuit breaker FORCE RESET - max open time exceeded in half-open state, resetting to closed")

			cb.state = StateClosed
			cb.failures = 0
			cb.firstOpenTime = time.Time{}
			cb.halfOpenSuccess = 0
			cb.halfOpenAttempts = 0

			return true
		}

		// Allow limited requests in half-open state
		if cb.halfOpenAttempts < cb.config.HalfOpenMax {
			cb.halfOpenAttempts++
			return true
		}

		return false

	default:
		return false
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		cb.halfOpenSuccess++
		// If all half-open requests succeeded, close the circuit
		if cb.halfOpenSuccess >= cb.config.HalfOpenMax {
			cb.state = StateClosed
			cb.failures = 0
			cb.halfOpenSuccess = 0
			cb.halfOpenAttempts = 0
			cb.firstOpenTime = time.Time{} // Clear open time tracker
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Open circuit if threshold is reached
		if cb.failures >= cb.config.Threshold {
			cb.state = StateOpen
			cb.firstOpenTime = time.Now() // Track when circuit first opened
		}

	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = StateOpen
		cb.halfOpenSuccess = 0
		cb.halfOpenAttempts = 0
		// Don't reset firstOpenTime - keep tracking total open duration
	}
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return string(cb.state)
}

// GetFailureCount returns the current failure count.
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenSuccess = 0
	cb.halfOpenAttempts = 0
	cb.firstOpenTime = time.Time{}
}
