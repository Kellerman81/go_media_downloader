package slidingwindow

import (
	"time"
)

//Source: https://github.com/RussellLuo/slidingwindow

// LocalWindow represents a window that ignores sync behavior entirely
// and only stores counters in memory.
type Limiter struct {
	// The start boundary (timestamp in nanoseconds) of the window.
	start time.Time

	//The last call
	last time.Time

	// The total count of events happened in the window.
	count int64

	// The total count of events happened in the window.
	max int64

	interval time.Duration
}

// NewLimiter returns a new Limiter that limits events to max
// events per interval duration.
func NewLimiter(interval time.Duration, max int64) *Limiter {
	return &Limiter{interval: interval, max: max, start: time.Now(), last: time.Now()}
}

// add increments the count and updates the last and start timestamps if
// the rate limit has not been reached. If the rate limit has been reached,
// the start timestamp is updated if enough time has passed since the first
// event in the window.
func (lim *Limiter) add() {
	set := time.Now()
	if lim.last.After(time.Now()) {
		//Moved Time to Future for Blocking
		return
	}

	if lim.count < lim.max {
		//Queue not full
		lim.count++
		lim.last = set
		return
	}

	if time.Since(lim.last) > lim.interval {
		//Last Call long ago

		lim.count = 1
		lim.last = set
		lim.start = set
		return
	}

	if time.Since(lim.start) > lim.interval {
		//First Call long ago
		lim.count = 1
		lim.last = set
		lim.start = set
		return
	}

	lim.last = set
}

// Allow checks if the rate limit would be exceeded by calling add. If the
// limit would be exceeded, Allow returns false and the remaining duration
// until the next event can happen. If the event can happen immediately,
// Allow calls add to increment the count and returns true.
func (lim *Limiter) Allow() (bool, time.Duration) {
	ok, wait := lim.Check()
	if !ok {
		return false, wait
	}
	lim.add()
	return true, 0 * time.Minute
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
	if lim.last.After(time.Now()) {
		//Date set to future for blocking
		return false, time.Until(lim.last)
	}
	if lim.count < lim.max {
		// Queue not full
		return true, 0 * time.Second
	}

	if time.Since(lim.last) > lim.interval {
		//Last Call long ago
		return true, 0 * time.Second
	}
	if time.Since(lim.start) > lim.interval {
		//First Call long ago
		return true, 0 * time.Second
	}
	return false, lim.interval - time.Since(lim.start)
}

// Interval returns the interval duration configured for the rate limiter.
func (lim *Limiter) Interval() time.Duration {
	return lim.interval
}

// WaitTill sets the last time to the given time. This overrides
// the rate limiting and forces the last time to be the given time.
func (lim *Limiter) WaitTill(now time.Time) {
	lim.last = now
}
