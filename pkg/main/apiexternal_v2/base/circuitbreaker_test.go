package base

import (
	"testing"
	"time"
)

func TestCircuitBreakerLifecycleAndReset(t *testing.T) {
	cb := newCircuitBreaker(CircuitBreakerConfig{Threshold: 2, Timeout: time.Hour, HalfOpenMax: 1, MaxOpenTime: 24 * time.Hour}, "test")

	cb.RecordFailure()
	cb.RecordSuccess() // a success while closed clears the count

	if cb.GetFailureCount() != 0 || cb.GetState() != string(StateClosed) {
		t.Fatalf("after success: %d failures, state %s", cb.GetFailureCount(), cb.GetState())
	}

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.GetState() != string(StateOpen) || cb.Allow() || cb.CanMakeRequest() {
		t.Fatalf("threshold reached but state %s, allow %t", cb.GetState(), cb.Allow())
	}

	cb.RecordSuccess() // ignored while open

	if cb.GetState() != string(StateOpen) {
		t.Fatal("success while open closed the breaker")
	}

	cb.Reset()

	if cb.GetState() != string(StateClosed) || cb.GetFailureCount() != 0 || !cb.Allow() {
		t.Errorf("after Reset: state %s, failures %d", cb.GetState(), cb.GetFailureCount())
	}
}

func TestCircuitBreakerAllowDoesNotConsume(t *testing.T) {
	cb := newCircuitBreaker(CircuitBreakerConfig{
		Threshold:   1,
		Timeout:     time.Millisecond,
		HalfOpenMax: 1,
	}, "t")

	cb.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	for range 3 {
		if !cb.Allow() {
			t.Fatal("Allow should report a probe is possible")
		}
	}

	if cb.GetState() != string(StateOpen) {
		t.Fatalf("Allow changed state to %s", cb.GetState())
	}

	ok, probe := cb.acquire()
	if !ok || !probe {
		t.Fatal("expected a half-open probe slot")
	}

	if cb.Allow() {
		t.Fatal("probe budget exhausted, Allow should be false")
	}

	cb.ReleaseProbe()

	if !cb.Allow() {
		t.Fatal("released probe slot should be available again")
	}
}
