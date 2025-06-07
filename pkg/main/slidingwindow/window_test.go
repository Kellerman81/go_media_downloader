package slidingwindow

import (
	"testing"
	"time"
)

func TestLimiterAllowBasic(t *testing.T) {
	lim := NewLimiter(time.Second, 3)

	allowed, wait := lim.Allow()
	if !allowed {
		t.Error("Expected first call to be allowed")
	}
	if wait != 0 {
		t.Error("Expected no wait time for first call")
	}

	allowed, wait = lim.Allow()
	if !allowed {
		t.Error("Expected second call to be allowed")
	}

	allowed, wait = lim.Allow()
	if !allowed {
		t.Error("Expected third call to be allowed")
	}

	allowed, wait = lim.Allow()
	if allowed {
		t.Error("Expected fourth call to be denied")
	}
	if wait == 0 {
		t.Error("Expected non-zero wait time after limit exceeded")
	}
}

func TestLimiterAllowForce(t *testing.T) {
	lim := NewLimiter(time.Second, 1)

	if !lim.AllowForce() {
		t.Error("Expected AllowForce to return true")
	}

	if !lim.AllowForce() {
		t.Error("Expected AllowForce to return true even after limit")
	}
}

func TestLimiterWaitTill(t *testing.T) {
	lim := NewLimiter(time.Second, 1)
	futureTime := time.Now().Add(time.Hour)

	allowed, _ := lim.Allow()
	lim.WaitTill(futureTime)

	allowed, _ = lim.Allow()
	if allowed {
		t.Error("Expected call to be denied after WaitTill")
	}
}

func TestLimiterInterval(t *testing.T) {
	interval := 2 * time.Second
	lim := NewLimiter(interval, 1)

	if lim.Interval() != interval {
		t.Errorf("Expected interval %v, got %v", interval, lim.Interval())
	}
}

func TestLimiterCheckBool(t *testing.T) {
	lim := NewLimiter(time.Second, 2)

	if !lim.CheckBool() {
		t.Error("Expected first check to return true")
	}

	lim.Allow()
	lim.Allow()

	if lim.CheckBool() {
		t.Error("Expected check to return false after limit reached")
	}

	time.Sleep(time.Second)

	if !lim.CheckBool() {
		t.Error("Expected check to return true after interval")
	}
}

func TestLimiterWindowReset(t *testing.T) {
	lim := NewLimiter(100*time.Millisecond, 2)

	lim.Allow()
	lim.Allow()

	time.Sleep(200 * time.Millisecond)

	allowed, wait := lim.Allow()
	if !allowed {
		t.Error("Expected allow after window reset")
	}
	if wait != 0 {
		t.Error("Expected no wait time after window reset")
	}
}
