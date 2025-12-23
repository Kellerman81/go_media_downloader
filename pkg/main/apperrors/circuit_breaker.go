package apperrors

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// StateClosed indicates the circuit is closed and requests flow normally.
	StateClosed CircuitState = iota
	// StateOpen indicates the circuit is open and requests fail fast.
	StateOpen
	// StateHalfOpen indicates the circuit is testing recovery with limited requests.
	StateHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
//
// The circuit breaker has three states:
//   - Closed: Normal operation, all requests pass through
//   - Open: Fast-fail mode, all requests fail immediately
//   - Half-Open: Testing recovery, limited requests allowed
//
// State transitions:
//   - Closed -> Open: When failure count exceeds maxFailures
//   - Open -> Half-Open: After resetTimeout duration
//   - Half-Open -> Closed: When halfOpenMaxCalls succeed
//   - Half-Open -> Open: When any call fails
type CircuitBreaker struct {
	name             string
	maxFailures      int
	resetTimeout     time.Duration
	halfOpenMaxCalls int

	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the specified configuration
//
// Parameters:
//   - name: identifier for logging and debugging
//   - maxFailures: number of failures before opening the circuit
//   - resetTimeout: duration to wait before attempting recovery
//
// Returns a configured CircuitBreaker instance.
func newCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		resetTimeout:     resetTimeout,
		halfOpenMaxCalls: 3, // Default: allow 3 test calls in half-open state
		state:            StateClosed,
		failures:         0,
		successes:        0,
	}
}

// NewCircuitBreakerWithHalfOpenCalls creates a circuit breaker with custom half-open settings
//
// Parameters:
//   - name: identifier for logging and debugging
//   - maxFailures: number of failures before opening the circuit
//   - resetTimeout: duration to wait before attempting recovery
//   - halfOpenMaxCalls: number of successful calls needed in half-open to close circuit
//
// Returns a configured CircuitBreaker instance.
func newCircuitBreakerWithHalfOpenCalls(
	name string,
	maxFailures int,
	resetTimeout time.Duration,
	halfOpenMaxCalls int,
) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		resetTimeout:     resetTimeout,
		halfOpenMaxCalls: halfOpenMaxCalls,
		state:            StateClosed,
		failures:         0,
		successes:        0,
	}
}

// Execute runs the given operation with circuit breaker protection
//
// Behavior by state:
//   - Closed: Execute operation normally, count failures
//   - Open: Return error immediately without executing operation
//   - Half-Open: Execute operation, transition based on result
//
// Parameters:
//   - operation: the function to execute with protection
//
// Returns error if circuit is open or if operation fails.
func (cb *CircuitBreaker) Execute(operation func() error) error {
	// Check current state and potentially transition
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the operation
	err := operation()

	// Update circuit breaker state based on result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks the circuit state before executing an operation
// and transitions state if necessary.
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Normal operation
		return nil

	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			log.Info().
				Str("circuit", cb.name).
				Str("from_state", "open").
				Str("to_state", "half-open").
				Dur("timeout", cb.resetTimeout).
				Msg("circuit breaker transitioning to half-open")
			cb.toHalfOpen()

			return nil
		}

		// Circuit is still open, fast-fail
		return New(ErrClassNetwork, "circuit_breaker",
			fmt.Sprintf("circuit breaker '%s' is open", cb.name))

	case StateHalfOpen:
		// Allow limited requests in half-open state
		return nil

	default:
		return New(ErrClassUnknown, "circuit_breaker",
			fmt.Sprintf("circuit breaker '%s' in unknown state", cb.name))
	}
}

// afterRequest updates the circuit state after an operation completes.
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles a successful operation.
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		if cb.failures > 0 {
			cb.failures = 0
			log.Debug().
				Str("circuit", cb.name).
				Msg("circuit breaker reset failures on success")
		}

	case StateHalfOpen:
		cb.successes++
		log.Debug().
			Str("circuit", cb.name).
			Int("successes", cb.successes).
			Int("required", cb.halfOpenMaxCalls).
			Msg("circuit breaker half-open success")

		// Transition to closed if enough successes
		if cb.successes >= cb.halfOpenMaxCalls {
			log.Info().
				Str("circuit", cb.name).
				Str("from_state", "half-open").
				Str("to_state", "closed").
				Int("successes", cb.successes).
				Msg("circuit breaker recovered")
			cb.toClosed()
		}
	}
}

// onFailure handles a failed operation.
func (cb *CircuitBreaker) onFailure() {
	cb.failures++

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		log.Debug().
			Str("circuit", cb.name).
			Int("failures", cb.failures).
			Int("max_failures", cb.maxFailures).
			Msg("circuit breaker failure")

		// Transition to open if max failures exceeded
		if cb.failures >= cb.maxFailures {
			log.Warn().
				Str("circuit", cb.name).
				Str("from_state", "closed").
				Str("to_state", "open").
				Int("failures", cb.failures).
				Dur("reset_timeout", cb.resetTimeout).
				Msg("circuit breaker opened")
			cb.toOpen()
		}

	case StateHalfOpen:
		// Any failure in half-open returns to open
		log.Warn().
			Str("circuit", cb.name).
			Str("from_state", "half-open").
			Str("to_state", "open").
			Msg("circuit breaker reopened after half-open failure")
		cb.toOpen()
	}
}

// toClosed transitions the circuit to closed state.
func (cb *CircuitBreaker) toClosed() {
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
}

// toOpen transitions the circuit to open state.
func (cb *CircuitBreaker) toOpen() {
	cb.state = StateOpen
	cb.successes = 0
}

// toHalfOpen transitions the circuit to half-open state.
func (cb *CircuitBreaker) toHalfOpen() {
	cb.state = StateHalfOpen
	cb.failures = 0
	cb.successes = 0
}

// GetState returns the current state of the circuit breaker (thread-safe).
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker to closed state
//
// This should be used carefully, typically only in testing or
// when external monitoring determines the service has recovered.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	previousState := cb.state
	cb.toClosed()

	if previousState != StateClosed {
		log.Info().
			Str("circuit", cb.name).
			Str("from_state", previousState.String()).
			Str("to_state", "closed").
			Msg("circuit breaker manually reset")
	}
}

// GetStats returns the current failure and success counts (thread-safe)
//
// Returns:
//   - failures: current number of consecutive failures
//   - successes: current number of consecutive successes (in half-open state)
func (cb *CircuitBreaker) GetStats() (failures int, successes int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures, cb.successes
}

// GetName returns the circuit breaker name.
func (cb *CircuitBreaker) GetName() string {
	return cb.name
}

// IsOpen returns true if the circuit is currently open (thread-safe).
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.GetState() == StateOpen
}

// IsClosed returns true if the circuit is currently closed (thread-safe).
func (cb *CircuitBreaker) IsClosed() bool {
	return cb.GetState() == StateClosed
}

// IsHalfOpen returns true if the circuit is currently half-open (thread-safe).
func (cb *CircuitBreaker) IsHalfOpen() bool {
	return cb.GetState() == StateHalfOpen
}
