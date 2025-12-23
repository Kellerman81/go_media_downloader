package apperrors

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestnewCircuitBreaker verifies circuit breaker constructor
func TestNewCircuitBreaker(t *testing.T) {
	cb := newCircuitBreaker("test-service", 5, 30*time.Second)

	assert.NotNil(t, cb)
	assert.Equal(t, "test-service", cb.GetName())
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, 5, cb.maxFailures)
	assert.Equal(t, 30*time.Second, cb.resetTimeout)
	assert.Equal(t, 3, cb.halfOpenMaxCalls) // default

	failures, successes := cb.GetStats()
	assert.Equal(t, 0, failures)
	assert.Equal(t, 0, successes)
}

// TestnewCircuitBreakerWithHalfOpenCalls verifies custom half-open configuration
func TestNewCircuitBreakerWithHalfOpenCalls(t *testing.T) {
	cb := newCircuitBreakerWithHalfOpenCalls("test-service", 3, 10*time.Second, 5)

	assert.NotNil(t, cb)
	assert.Equal(t, 5, cb.halfOpenMaxCalls)
}

// TestCircuitBreakerClosed verifies normal operation in closed state
func TestCircuitBreakerClosed(t *testing.T) {
	cb := newCircuitBreaker("test-service", 3, 1*time.Second)

	// Successful operations should work normally
	callCount := 0
	for i := 0; i < 10; i++ {
		err := cb.Execute(func() error {
			callCount++
			return nil
		})
		assert.NoError(t, err)
	}

	assert.Equal(t, 10, callCount)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())
}

// TestCircuitBreakerOpens verifies circuit opens after max failures
func TestCircuitBreakerOpens(t *testing.T) {
	maxFailures := 3
	cb := newCircuitBreaker("test-service", maxFailures, 1*time.Second)

	// Trigger failures to open circuit
	for i := 0; i < maxFailures; i++ {
		err := cb.Execute(func() error {
			return fmt.Errorf("operation failed")
		})
		assert.Error(t, err)
		assert.Equal(t, "operation failed", err.Error())
	}

	// Circuit should now be open
	assert.Equal(t, StateOpen, cb.GetState())
	assert.True(t, cb.IsOpen())

	failures, _ := cb.GetStats()
	assert.Equal(t, maxFailures, failures)

	// Subsequent calls should fail fast without executing
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})

	assert.Error(t, err)
	assert.Equal(t, 0, callCount, "operation should not execute when circuit is open")

	// Error should be a classified retryable error
	classifiedErr, ok := err.(*ClassifiedError)
	require.True(t, ok)
	assert.Equal(t, ErrClassNetwork, classifiedErr.Class)
	assert.Contains(t, classifiedErr.Error(), "circuit breaker 'test-service' is open")
}

// TestCircuitBreakerHalfOpen verifies transition to half-open state
func TestCircuitBreakerHalfOpen(t *testing.T) {
	maxFailures := 2
	resetTimeout := 100 * time.Millisecond
	cb := newCircuitBreaker("test-service", maxFailures, resetTimeout)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for reset timeout
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// Next request should transition to half-open
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil // Success
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "operation should execute in half-open state")
	assert.Equal(t, StateHalfOpen, cb.GetState())
	assert.True(t, cb.IsHalfOpen())
}

// TestCircuitBreakerHalfOpenToClose verifies recovery from half-open to closed
func TestCircuitBreakerHalfOpenToClose(t *testing.T) {
	maxFailures := 2
	resetTimeout := 100 * time.Millisecond
	halfOpenMaxCalls := 3
	cb := newCircuitBreakerWithHalfOpenCalls("test-service", maxFailures, resetTimeout, halfOpenMaxCalls)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for reset timeout
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// Execute enough successful operations to close the circuit
	for i := 0; i < halfOpenMaxCalls; i++ {
		err := cb.Execute(func() error {
			return nil
		})
		assert.NoError(t, err)

		if i < halfOpenMaxCalls-1 {
			assert.Equal(t, StateHalfOpen, cb.GetState(), "should remain half-open until enough successes")
		}
	}

	// Circuit should now be closed
	assert.Equal(t, StateClosed, cb.GetState())
	assert.True(t, cb.IsClosed())

	failures, successes := cb.GetStats()
	assert.Equal(t, 0, failures, "failures should be reset")
	assert.Equal(t, 0, successes, "successes should be reset")
}

// TestCircuitBreakerHalfOpenToOpen verifies failure in half-open reopens circuit
func TestCircuitBreakerHalfOpenToOpen(t *testing.T) {
	maxFailures := 2
	resetTimeout := 100 * time.Millisecond
	cb := newCircuitBreaker("test-service", maxFailures, resetTimeout)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for reset timeout
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// First successful call transitions to half-open
	err := cb.Execute(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Failure in half-open should reopen circuit
	err = cb.Execute(func() error {
		return fmt.Errorf("failure in half-open")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
	assert.True(t, cb.IsOpen())
}

// TestCircuitBreakerReset verifies manual reset functionality
func TestCircuitBreakerReset(t *testing.T) {
	maxFailures := 2
	cb := newCircuitBreaker("test-service", maxFailures, 1*time.Second)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Manual reset
	cb.Reset()

	assert.Equal(t, StateClosed, cb.GetState())
	failures, successes := cb.GetStats()
	assert.Equal(t, 0, failures)
	assert.Equal(t, 0, successes)

	// Should work normally after reset
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// TestCircuitBreakerResetFromClosed verifies reset from closed state is safe
func TestCircuitBreakerResetFromClosed(t *testing.T) {
	cb := newCircuitBreaker("test-service", 3, 1*time.Second)

	assert.Equal(t, StateClosed, cb.GetState())

	// Reset should be safe even when already closed
	cb.Reset()

	assert.Equal(t, StateClosed, cb.GetState())
}

// TestCircuitBreakerConcurrency verifies thread safety
func TestCircuitBreakerConcurrency(t *testing.T) {
	maxFailures := 50
	cb := newCircuitBreaker("test-service", maxFailures, 100*time.Millisecond)

	var wg sync.WaitGroup
	goroutines := 100
	operationsPerGoroutine := 10

	var successCount atomic.Int32
	var failureCount atomic.Int32

	// Launch concurrent operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				err := cb.Execute(func() error {
					// Simulate some work
					time.Sleep(1 * time.Millisecond)

					// Fail every 3rd operation
					if (id+j)%3 == 0 {
						return fmt.Errorf("simulated failure")
					}
					return nil
				})

				if err != nil {
					failureCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify no race conditions occurred
	totalOps := successCount.Load() + failureCount.Load()
	assert.Greater(t, int(totalOps), 0, "should have executed operations")

	// Circuit breaker state should be consistent
	state := cb.GetState()
	assert.Contains(t, []CircuitState{StateClosed, StateOpen, StateHalfOpen}, state)
}

// TestCircuitBreakerGetStats verifies statistics retrieval
func TestCircuitBreakerGetStats(t *testing.T) {
	cb := newCircuitBreaker("test-service", 5, 1*time.Second)

	// Initial stats
	failures, successes := cb.GetStats()
	assert.Equal(t, 0, failures)
	assert.Equal(t, 0, successes)

	// Add some failures
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}

	failures, successes = cb.GetStats()
	assert.Equal(t, 3, failures)
	assert.Equal(t, 0, successes)

	// Add a success (should reset failures in closed state)
	cb.Execute(func() error {
		return nil
	})

	failures, successes = cb.GetStats()
	assert.Equal(t, 0, failures, "success should reset failures in closed state")
}

// TestCircuitBreakerStateTransitions verifies full state machine
func TestCircuitBreakerStateTransitions(t *testing.T) {
	maxFailures := 2
	resetTimeout := 100 * time.Millisecond
	halfOpenMaxCalls := 2
	cb := newCircuitBreakerWithHalfOpenCalls("test-service", maxFailures, resetTimeout, halfOpenMaxCalls)

	// Initial state: Closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Transition 1: Closed -> Open (after max failures)
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure %d", i)
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Verify fast-fail in open state
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, 0, callCount)

	// Wait for reset timeout
	time.Sleep(resetTimeout + 50*time.Millisecond)

	// Transition 2: Open -> Half-Open (after timeout)
	err = cb.Execute(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Transition 3: Half-Open -> Closed (after successful calls)
	for i := 0; i < halfOpenMaxCalls-1; i++ {
		err = cb.Execute(func() error {
			return nil
		})
		assert.NoError(t, err)
	}
	assert.Equal(t, StateClosed, cb.GetState())

	// Circuit is now closed and healthy
	err = cb.Execute(func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

// TestCircuitBreakerStateTransitionHalfOpenToOpen verifies half-open failure path
func TestCircuitBreakerStateTransitionHalfOpenToOpen(t *testing.T) {
	maxFailures := 2
	resetTimeout := 100 * time.Millisecond
	cb := newCircuitBreaker("test-service", maxFailures, resetTimeout)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for reset timeout and transition to half-open
	time.Sleep(resetTimeout + 50*time.Millisecond)
	cb.Execute(func() error {
		return nil
	})
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Transition 4: Half-Open -> Open (on failure)
	err := cb.Execute(func() error {
		return fmt.Errorf("failed in half-open")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

// TestCircuitBreakerPartialFailures verifies partial failure handling
func TestCircuitBreakerPartialFailures(t *testing.T) {
	maxFailures := 5
	cb := newCircuitBreaker("test-service", maxFailures, 1*time.Second)

	// Mix of successes and failures (not enough to open)
	for i := 0; i < 4; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateClosed, cb.GetState())

	failures, _ := cb.GetStats()
	assert.Equal(t, 4, failures)

	// Success should reset failure count in closed state
	cb.Execute(func() error {
		return nil
	})

	failures, _ = cb.GetStats()
	assert.Equal(t, 0, failures, "success should reset failure count")
	assert.Equal(t, StateClosed, cb.GetState())
}

// TestCircuitBreakerStateString verifies state string representation
func TestCircuitBreakerStateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// TestCircuitBreakerMultipleResets verifies multiple reset operations
func TestCircuitBreakerMultipleResets(t *testing.T) {
	cb := newCircuitBreaker("test-service", 2, 1*time.Second)

	// Open circuit
	cb.Execute(func() error { return fmt.Errorf("fail") })
	cb.Execute(func() error { return fmt.Errorf("fail") })
	assert.Equal(t, StateOpen, cb.GetState())

	// Reset multiple times
	for i := 0; i < 5; i++ {
		cb.Reset()
		assert.Equal(t, StateClosed, cb.GetState())
		failures, successes := cb.GetStats()
		assert.Equal(t, 0, failures)
		assert.Equal(t, 0, successes)
	}
}

// TestCircuitBreakerTimeoutPrecision verifies timeout handling
func TestCircuitBreakerTimeoutPrecision(t *testing.T) {
	maxFailures := 2
	resetTimeout := 200 * time.Millisecond
	cb := newCircuitBreaker("test-service", maxFailures, resetTimeout)

	// Open the circuit
	for i := 0; i < maxFailures; i++ {
		cb.Execute(func() error {
			return fmt.Errorf("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Try before timeout - should still be open
	time.Sleep(100 * time.Millisecond)
	callCount := 0
	err := cb.Execute(func() error {
		callCount++
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, 0, callCount, "should fast-fail before timeout")
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for remaining timeout
	time.Sleep(150 * time.Millisecond)

	// Should now allow request (transition to half-open)
	err = cb.Execute(func() error {
		callCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, StateHalfOpen, cb.GetState())
}

// TestCircuitBreakerConcurrentStateReads verifies concurrent read safety
func TestCircuitBreakerConcurrentStateReads(t *testing.T) {
	cb := newCircuitBreaker("test-service", 3, 100*time.Millisecond)

	var wg sync.WaitGroup
	readers := 100

	// Concurrent state reads
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = cb.GetState()
				_ = cb.IsOpen()
				_ = cb.IsClosed()
				_ = cb.IsHalfOpen()
				_, _ = cb.GetStats()
			}
		}()
	}

	// Concurrent writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			cb.Execute(func() error {
				if i%2 == 0 {
					return fmt.Errorf("failure")
				}
				return nil
			})
		}
	}()

	// Should not race
	wg.Wait()
}

// TestCircuitBreakerZeroMaxFailures verifies edge case handling
func TestCircuitBreakerZeroMaxFailures(t *testing.T) {
	// Edge case: circuit that opens on first failure
	cb := newCircuitBreaker("test-service", 0, 1*time.Second)

	// Single failure should open circuit immediately
	err := cb.Execute(func() error {
		return fmt.Errorf("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

// TestCircuitBreakerHelperMethods verifies convenience methods
func TestCircuitBreakerHelperMethods(t *testing.T) {
	cb := newCircuitBreaker("test-service", 2, 100*time.Millisecond)

	// Closed state
	assert.True(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())

	// Open circuit
	cb.Execute(func() error { return fmt.Errorf("fail") })
	cb.Execute(func() error { return fmt.Errorf("fail") })

	assert.False(t, cb.IsClosed())
	assert.True(t, cb.IsOpen())
	assert.False(t, cb.IsHalfOpen())

	// Transition to half-open
	time.Sleep(150 * time.Millisecond)
	cb.Execute(func() error { return nil })

	assert.False(t, cb.IsClosed())
	assert.False(t, cb.IsOpen())
	assert.True(t, cb.IsHalfOpen())
}
