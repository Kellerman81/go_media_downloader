package slidingwindow

import (
	"sync"
	"time"
)

// Source: https://github.com/RussellLuo/slidingwindow

// LocalWindow represents a window that ignores sync behavior entirely
// and only stores counters in memory.
type Limiter struct {
	// The start boundary (timestamp in nanoseconds) of the window.
	start int64

	// The last call
	last int64

	interval int64

	// The total count of events happened in the window.
	count int64

	// The total count of events happened in the window.
	max int64
	mu  sync.Mutex
}

// add increments the count and updates the last and start timestamps if
// the rate limit has not been reached. If the rate limit has been reached,
// the start timestamp is updated if enough time has passed since the first
// event in the window.
func (lim *Limiter) add() {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	now := time.Now().UnixNano()
	if now < lim.last {
		// Moved Time to Future for Blocking
		return
	}

	if lim.count < lim.max {
		// Queue not full
		lim.count++
		lim.last = now
		return
	}

	timeSinceStart := now - lim.start
	timeSinceLast := now - lim.last

	if timeSinceLast > lim.interval || timeSinceStart > lim.interval {
		// Last Call long ago

		lim.count = 1
		lim.last = now
		lim.start = now
		return
	}

	lim.last = now
}

// Allow checks if the rate limit would be exceeded by calling add. If the
// limit would be exceeded, Allow returns false and the remaining duration
// until the next event can happen. If the event can happen immediately,
// Allow calls add to increment the count and returns true.
// This method is atomic to prevent race conditions between check and add operations.
func (lim *Limiter) Allow() (bool, time.Duration) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	
	now := time.Now().UnixNano()
	if now < lim.last {
		// Date set to future for blocking
		return false, time.Duration(lim.last - now)
	}
	if lim.count < lim.max {
		// Queue not full - can allow and increment
		lim.count++
		lim.last = now
		return true, 0
	}

	timeSinceStart := now - lim.start
	timeSinceLast := now - lim.last

	if timeSinceLast > lim.interval || timeSinceStart > lim.interval {
		// Last Call long ago - reset and allow
		lim.count = 1
		lim.last = now
		lim.start = now
		return true, 0
	}
	
	// Rate limit exceeded
	remainingTime := lim.interval - timeSinceStart
	lim.last = now
	return false, time.Duration(remainingTime)
}

// AllowForce unconditionally increments the rate limiter count and returns
// true, without checking if the rate limit would be exceeded. This allows
// forcing an event through even if the rate limit has been reached.
func (lim *Limiter) AllowForce() bool {
	lim.add()
	return true
}

// Check returns whether the rate limit would be exceeded if an event is added
// now. It returns a bool indicating if the limit would be exceeded, and a
// time.Duration for the remaining time until the next event can happen without
// exceeding the rate limit.
func (lim *Limiter) Check() (bool, time.Duration) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	now := time.Now().UnixNano()
	if now < lim.last {
		// Date set to future for blocking
		return false, time.Duration(lim.last - now)
	}
	if lim.count < lim.max {
		// Queue not full
		return true, 0 * time.Second
	}

	timeSinceStart := now - lim.start
	timeSinceLast := now - lim.last

	if timeSinceLast > lim.interval || timeSinceStart > lim.interval {
		// Last Call long ago
		return true, 0
	}
	remainingTime := lim.interval - timeSinceStart
	return false, time.Duration(remainingTime)
}

// CheckBool checks if the rate limit would be exceeded by calling add. It returns
// a boolean indicating whether the rate limit would be exceeded or not.
func (lim *Limiter) CheckBool() bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	now := time.Now().UnixNano()
	if now < lim.last {
		// Date set to future for blocking
		return false
	}
	if lim.count < lim.max {
		// Queue not full
		return true
	}

	timeSinceStart := now - lim.start
	timeSinceLast := now - lim.last

	return timeSinceLast > lim.interval || timeSinceStart > lim.interval
}

// Interval returns the interval duration configured for the rate limiter.
func (lim *Limiter) Interval() time.Duration {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	return time.Duration(lim.interval)
}

// WaitTill sets the last time to the given time. This overrides
// the rate limiting and forces the last time to be the given time.
func (lim *Limiter) WaitTill(now time.Time) {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	lim.last = now.UnixNano()
}

// NewLimiter returns a new Limiter that limits events to max
// events per interval duration.
func NewLimiter(interval time.Duration, maxevents int64) Limiter {
	now := time.Now().UnixNano()
	return Limiter{interval: int64(interval), max: maxevents, start: now, last: now}
}
