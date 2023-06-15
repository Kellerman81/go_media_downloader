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

func NewLimiter(interval time.Duration, max int64) *Limiter {
	return &Limiter{interval: interval, max: max, start: time.Now(), last: time.Now()}
}

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

// AllowN reports whether n events may happen at time now.
func (lim *Limiter) Allow() (bool, time.Duration) {
	ok, wait := lim.Check()
	if !ok {
		return false, wait
	}
	lim.add()
	return true, 0 * time.Minute
}

// AllowN reports whether n events may happen at time now.
func (lim *Limiter) AllowForce() bool {
	lim.add()
	return true
}

// AllowN reports whether n events may happen at time now.
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
	//return int64((float64(lim.interval-elapsed)/float64(lim.interval))*float64(lim.prev.Count()))+lim.curr.Count()+1 <= lim.limit, lim.interval - elapsed
}

func (lim *Limiter) Interval() time.Duration {
	return lim.interval
}

func (lim *Limiter) WaitTill(now time.Time) {
	lim.last = now
}
