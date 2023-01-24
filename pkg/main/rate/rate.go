package rate

import (
	"container/list"
	"sync"
	"time"
)

// A RateLimiter limits the rate at which an action can be performed.  It
// applies neither smoothing (like one could achieve in a token bucket system)
// nor does it offer any conception of warmup, wherein the rate of actions
// granted are steadily increased until a steady throughput equilibrium is
// reached.
type Limiter struct {
	limit      int
	dailylimit int
	interval   time.Duration
	mtx        sync.Mutex
	times      list.List
	timesdaily list.List
}

// New creates a new rate limiter for the limit and interval.
func New(limit int, dailylimt int, interval time.Duration) *Limiter {
	lim := Limiter{
		limit:      limit,
		dailylimit: dailylimt,
		interval:   interval,
	}
	lim.times.Init()
	lim.timesdaily.Init()
	return &lim
}

//daily = rate.New(limitercallsdaily, 24*time.Hour)

// Try returns true if under the rate limit, or false if over and the
// remaining time before the rate limit expires.
func (r *Limiter) Allow() (ok bool, remaining time.Duration) {
	ok, _, left := r.Check(true, true)
	if ok {
		now := time.Now()
		if r.times.Len() < r.limit && (r.timesdaily.Len() < r.dailylimit || r.dailylimit == 0) {
			r.times.PushBack(now)
			if r.dailylimit != 0 {
				r.timesdaily.PushBack(now)
			}
			return ok, left
		}

		frnt := r.times.Front()
		frnt.Value = now
		r.times.MoveToBack(frnt)

		if r.dailylimit != 0 {
			frntdaily := r.timesdaily.Front()
			frntdaily.Value = now
			r.timesdaily.MoveToBack(frntdaily)
		}
	}
	return ok, left
	// r.mtx.Lock()
	// defer r.mtx.Unlock()
	// now := time.Now()
	// if r.times.Len() < r.limit && (r.timesdaily.Len() < r.dailylimit || r.dailylimit == 0) {
	// 	r.times.PushBack(now)
	// 	if r.dailylimit != 0 {
	// 		r.timesdaily.PushBack(now)
	// 	}
	// 	return true, 0
	// }
	// frnt := r.times.Front()

	// if diff := now.Sub(frnt.Value.(time.Time)); diff < r.interval {
	// 	return false, r.interval - diff
	// }
	// if r.dailylimit != 0 {
	// 	frntdaily := r.timesdaily.Front()
	// 	if diff := now.Sub(frntdaily.Value.(time.Time)); diff < (24 * time.Hour) {
	// 		return false, (24 * time.Hour) - diff
	// 	}
	// 	frntdaily.Value = now
	// 	r.timesdaily.MoveToBack(frntdaily)
	// }
	// frnt.Value = now
	// r.times.MoveToBack(frnt)
	// return true, 0
}
func (r *Limiter) AllowForce() {
	now := time.Now()
	if r.times.Len() < r.limit && (r.timesdaily.Len() < r.dailylimit || r.dailylimit == 0) {
		r.times.PushBack(now)
		if r.dailylimit != 0 {
			r.timesdaily.PushBack(now)
		}
		return
	}

	frnt := r.times.Front()
	if frnt == nil {
		r.times.PushBack(now)
	} else {
		frnt.Value = now
		r.times.MoveToBack(frnt)
	}

	if r.dailylimit != 0 {
		frntdaily := r.timesdaily.Front()
		if frntdaily == nil {
			r.timesdaily.PushBack(now)
		} else {
			frntdaily.Value = now
			r.timesdaily.MoveToBack(frntdaily)
		}
	}
}

func (r *Limiter) Check(interval bool, daily bool) (ok bool, hitdaily bool, remaining time.Duration) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	now := time.Now()

	if interval {
		frnt := r.times.Front()
		if frnt == nil {
			return true, true, 0
		}
		if diff := now.Sub(frnt.Value.(time.Time)); diff < r.interval && r.times.Len() >= r.limit {
			return false, true, r.interval - diff
		}
	}
	if !daily {
		return true, true, 0
	}
	if r.dailylimit != 0 {
		frntdaily := r.timesdaily.Front()
		if frntdaily == nil {
			return true, true, 0
		}
		if diff := now.Sub(frntdaily.Value.(time.Time)); diff < (24*time.Hour) && r.timesdaily.Len() >= r.dailylimit {
			return false, false, (24 * time.Hour) - diff
		}
	}
	return true, true, 0
}

// Try returns true if under the rate limit, or false if over and the
// remaining time before the rate limit expires.
func (r *Limiter) WaitTill(settime time.Time) {
	r.mtx.Lock()

	for e := r.times.Front(); e != nil; e = e.Next() {
		e.Value = settime
	}
	r.mtx.Unlock()
}
