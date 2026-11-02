package slidingwindow

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// LimiterMetrics provides comprehensive metrics for rate limiter monitoring.
type LimiterMetrics struct {
	// Request counters
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64

	// Performance metrics
	AvgResponseTime time.Duration
	MaxResponseTime time.Duration
	P99ResponseTime time.Duration

	// Concurrency metrics
	MaxConcurrent int64
	CurrentActive int64

	// Window metrics
	WindowUtilization float64
	CleanupOperations int64

	UptimeStart time.Time
}

const (
	defaultsleeptime   = 100  // in Milliseconds
	maxResponseSamples = 1000 // Maximum number of response time samples to keep
)

// Limiter provides a high-performance sliding window rate limiter
// with comprehensive metrics, observability, and advanced features.
type Limiter struct {
	mu       sync.RWMutex
	interval time.Duration
	max      int64

	// requests holds the timestamps of admitted requests in ascending order,
	// which lets trimLocked find the window cutoff with a binary search.
	requests []time.Time

	reservations map[string]time.Time // client ID -> reservation time
	// nextSlot is an externally imposed block (e.g. a server's Retry-After),
	// honoured by Allow/Check/Reserve until it passes.
	nextSlot time.Time

	capacityHint int

	totalRequests   atomic.Int64
	allowedRequests atomic.Int64
	deniedRequests  atomic.Int64
	cleanupOps      atomic.Int64
	maxConcurrent   atomic.Int64
	currentActive   atomic.Int64

	// Response time tracking
	responseTimes     []time.Duration
	responseTimesIdx  int
	maxResponseTime   atomic.Int64 // nanoseconds
	totalResponseTime atomic.Int64 // nanoseconds for average calculation

	updateStart time.Time
}

// NewLimiter creates a new improved sliding window rate limiter
// with comprehensive metrics and performance optimizations.
func NewLimiter(interval time.Duration, maxRequests int64) *Limiter {
	// Every current caller already guards maxRequests > 0 before calling this,
	// but that's an unenforced convention, not a type-level guarantee - a
	// maxRequests of 0 would make l.max*4 == 0 in
	// AllowWithWaitContextAndMaxWait/CheckWithWait, and dividing a
	// time.Duration by a zero divisor panics. Enforce the invariant here
	// once instead of trusting every future caller to replicate the guard.
	if maxRequests <= 0 {
		maxRequests = 1
	}

	capacity := max(int(maxRequests), 10)

	limiter := &Limiter{
		interval:      interval,
		max:           maxRequests,
		requests:      make([]time.Time, 0, capacity*2),
		reservations:  make(map[string]time.Time),
		capacityHint:  capacity,
		responseTimes: make([]time.Duration, 0, maxResponseSamples),
		updateStart:   time.Now(),
	}

	return limiter
}

// trimLocked drops requests (and reservations) that have left the window.
// It runs on every state-changing call so admission decisions are always
// made on the fresh count; requests is time-ordered, so finding the cutoff
// is a binary search. Caller must hold l.mu for writing.
func (l *Limiter) trimLocked(now time.Time) {
	cutoff := now.Add(-l.interval)

	if validStart := l.firstFresh(cutoff); validStart > 0 {
		n := copy(l.requests, l.requests[validStart:])
		l.requests = l.requests[:n]
		l.cleanupOps.Add(1)
	}

	for id, resTime := range l.reservations {
		if !resTime.After(cutoff) {
			delete(l.reservations, id)
		}
	}
}

// firstFresh returns the index of the first request newer than cutoff.
func (l *Limiter) firstFresh(cutoff time.Time) int {
	left, right := 0, len(l.requests)
	for left < right {
		mid := (left + right) / 2
		if l.requests[mid].After(cutoff) {
			right = mid
		} else {
			left = mid + 1
		}
	}

	return left
}

// waitForSlot returns how long until a new request fits, given that
// requests[start:] are inside the window and `extra` further slots are
// held (reservations). Zero means a slot is free now.
func (l *Limiter) waitForSlot(now time.Time, start, extra int) time.Duration {
	if now.Before(l.nextSlot) {
		return l.nextSlot.Sub(now)
	}

	used := int64(len(l.requests)-start) + int64(extra)
	if used < l.max {
		return 0
	}

	// The (used-max+1)-th oldest in-window request has to expire first.
	idx := start + int(used-l.max)
	if idx >= len(l.requests) {
		// Only reservations hold the remaining capacity.
		return l.interval
	}

	if wait := l.requests[idx].Add(l.interval).Sub(now); wait > 0 {
		return wait
	}

	return time.Nanosecond
}

// Allow attempts to allow a request immediately with comprehensive metrics tracking.
func (l *Limiter) Allow() (bool, time.Duration) {
	start := time.Now()

	l.totalRequests.Add(1)

	active := l.currentActive.Add(1)
	defer l.currentActive.Add(-1)

	for {
		maxCurrent := l.maxConcurrent.Load()
		if active <= maxCurrent ||
			l.maxConcurrent.CompareAndSwap(maxCurrent, active) {
			break
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.trimLocked(now)

	if wait := l.waitForSlot(now, 0, 0); wait > 0 {
		l.deniedRequests.Add(1)
		l.recordResponseTime(start, now)

		return false, wait
	}

	l.requests = append(l.requests, now)
	l.allowedRequests.Add(1)
	l.recordResponseTime(start, now)

	return true, 0
}

// Check returns whether a request would be allowed without consuming a slot.
func (l *Limiter) Check() (bool, time.Duration) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()

	wait := l.waitForSlot(now, l.firstFresh(now.Add(-l.interval)), 0)

	return wait == 0, wait
}

// Reserve attempts to reserve a slot for future use with enhanced tracking.
func (l *Limiter) Reserve(id string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.trimLocked(now)

	if _, exists := l.reservations[id]; exists {
		return true, 0
	}

	if wait := l.waitForSlot(now, 0, len(l.reservations)); wait > 0 {
		return false, wait
	}

	l.reservations[id] = now

	return true, 0
}

// UseReservation converts a reservation into an actual request with metrics tracking.
func (l *Limiter) UseReservation(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.reservations[id]; !exists {
		return false
	}

	delete(l.reservations, id)

	now := time.Now()

	l.requests = append(l.requests, now)
	l.allowedRequests.Add(1)
	l.totalRequests.Add(1)

	return true
}

// Stats returns current limiter statistics.
func (l *Limiter) Stats() (activeRequests, reservations int64, nextAvailable time.Time) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-l.interval)

	// Count active requests without modifying the slice
	activeCount := 0
	for i := range l.requests {
		if l.requests[i].After(cutoff) {
			activeCount++
		}
	}

	return int64(activeCount), int64(len(l.reservations)), l.nextSlot
}

// AllowForce adds a request regardless of limits (for compatibility with original).
func (l *Limiter) AllowForce() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.trimLocked(now)

	l.requests = append(l.requests, now)

	return true
}

// CheckBool returns whether a request would be allowed (compatibility method).
func (l *Limiter) CheckBool() bool {
	allowed, _ := l.Check()
	return allowed
}

// Interval returns the interval duration configured for the rate limiter.
// interval is set once in NewLimiter and never mutated afterward, so this
// only needs a read lock - the write lock unnecessarily serialized every
// caller against concurrent Allow()/Reserve() calls.
func (l *Limiter) Interval() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.interval
}

// AllowWithWait attempts to allow a request, waiting up to maxwait for a slot to become available.
// Returns true if the request was allowed, false if the timeout was reached.
func (l *Limiter) AllowWithWait(maxwait time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), maxwait)
	defer cancel()
	return l.AllowWithWaitContext(ctx, maxwait)
}

// AllowWithWaitContext attempts to allow a request, waiting for a slot to become available
// until the context is cancelled, times out, or maximum wait time is reached.
// Returns true if the request was allowed, false if cancelled or timed out.
func (l *Limiter) AllowWithWaitContext(ctx context.Context, maxwait time.Duration) bool {
	return l.AllowWithWaitContextAndMaxWait(ctx, maxwait) // 30 second maximum wait
}

// AllowWithWaitContextAndMaxWait attempts to allow a request with optimized waiting strategy.
// This prevents indefinite blocking by limiting the total time spent waiting.
func (l *Limiter) AllowWithWaitContextAndMaxWait(ctx context.Context, maxWait time.Duration) bool {
	startTime := time.Now()
	maxRetries := 1000 // Increased for better precision
	retryCount := 0

	// Adaptive sleep time based on window size
	minSleepTime := min(
		max(l.interval/time.Duration(l.max*4), time.Millisecond),
		defaultsleeptime*time.Millisecond,
	)

	for retryCount < maxRetries {
		// Try to get a slot immediately
		allowed, waitFor := l.Allow()
		if allowed {
			return true
		}

		// Check if we've exceeded maximum wait time
		elapsed := time.Since(startTime)
		if elapsed >= maxWait {
			return false
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// Adaptive sleep time calculation
		sleepTime := max(waitFor, minSleepTime)

		// Ensure we don't sleep past the maximum wait time
		remainingWait := maxWait - elapsed
		if sleepTime > remainingWait {
			sleepTime = remainingWait
		}

		// Don't sleep if remaining time is too small
		if sleepTime <= time.Microsecond {
			return false
		}

		// Use exponential backoff for failed attempts
		if retryCount > 10 {
			backoffMultiplier := 1.0 + float64(retryCount-10)*0.1
			if backoffMultiplier > 2.0 {
				backoffMultiplier = 2.0
			}

			sleepTime = min(time.Duration(float64(sleepTime)*backoffMultiplier), remainingWait)
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(sleepTime):
			retryCount++
			// Yield to other goroutines periodically
			if retryCount%50 == 0 {
				runtime.Gosched()
			}
		}
	}

	return false // Exceeded maximum retries
}

// CheckWithWait checks if a request would be allowed, waiting for a slot to become available
// until the context is cancelled or times out. Unlike AllowWithWaitContext, this does NOT
// consume a slot - it only checks if one would be available. Returns true if a slot would be available.
func (l *Limiter) CheckWithWait(ctx context.Context) bool {
	retryCount := 0
	maxRetries := 1000

	// Adaptive sleep time based on window size
	minSleepTime := min(
		max(l.interval/time.Duration(l.max*4), time.Millisecond),
		defaultsleeptime*time.Millisecond,
	)

	for retryCount < maxRetries {
		// Try to check if a slot is available immediately
		allowed, waitFor := l.Check()
		if allowed {
			return true
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// Adaptive sleep time calculation
		sleepTime := max(waitFor, minSleepTime)

		// Use exponential backoff for failed attempts
		if retryCount > 10 {
			backoffMultiplier := 1.0 + float64(retryCount-10)*0.1
			if backoffMultiplier > 2.0 {
				backoffMultiplier = 2.0
			}

			sleepTime = time.Duration(float64(sleepTime) * backoffMultiplier)
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(sleepTime):
			retryCount++
			// Yield to other goroutines periodically
			if retryCount%50 == 0 {
				runtime.Gosched()
			}
		}
	}

	return false // Exceeded maximum retries
}

// recordResponseTime records response time for metrics tracking.
func (l *Limiter) recordResponseTime(start, end time.Time) {
	duration := end.Sub(start)
	nanos := duration.Nanoseconds()

	// Update max response time atomically
	for {
		maxCurrent := l.maxResponseTime.Load()
		if nanos <= maxCurrent ||
			l.maxResponseTime.CompareAndSwap(maxCurrent, nanos) {
			break
		}
	}

	// Update total response time for average calculation
	l.totalResponseTime.Add(nanos)

	// Store in circular buffer for percentile calculations
	if len(l.responseTimes) < maxResponseSamples {
		l.responseTimes = append(l.responseTimes, duration)
	} else {
		l.responseTimes[l.responseTimesIdx] = duration
		l.responseTimesIdx = (l.responseTimesIdx + 1) % maxResponseSamples
	}
}

// GetMetrics returns comprehensive metrics about the rate limiter's performance.
func (l *Limiter) GetMetrics() LimiterMetrics {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-l.interval)

	// Calculate window utilization
	activeCount := 0
	for i := range l.requests {
		if l.requests[i].After(cutoff) {
			activeCount++
		}
	}

	utilization := float64(activeCount) / float64(l.max)

	// Calculate response time metrics
	totalReqs := l.totalRequests.Load()

	avgResponseTime := time.Duration(0)
	if totalReqs > 0 {
		avgResponseTime = time.Duration(l.totalResponseTime.Load() / totalReqs)
	}

	// Calculate P99 response time (optimized)
	p99ResponseTime := time.Duration(l.maxResponseTime.Load())
	if len(l.responseTimes) > 10 {
		// Sort a copy and get 99th percentile
		sortedTimes := make([]time.Duration, len(l.responseTimes))
		copy(sortedTimes, l.responseTimes)
		// Use built-in sort for better performance
		slices.Sort(sortedTimes)

		p99Index := int(float64(len(sortedTimes)) * 0.99)
		if p99Index >= len(sortedTimes) {
			p99Index = len(sortedTimes) - 1
		}

		if p99Index >= 0 {
			p99ResponseTime = sortedTimes[p99Index]
		}
	}

	return LimiterMetrics{
		TotalRequests:     l.totalRequests.Load(),
		AllowedRequests:   l.allowedRequests.Load(),
		DeniedRequests:    l.deniedRequests.Load(),
		AvgResponseTime:   avgResponseTime,
		MaxResponseTime:   time.Duration(l.maxResponseTime.Load()),
		P99ResponseTime:   p99ResponseTime,
		MaxConcurrent:     l.maxConcurrent.Load(),
		CurrentActive:     l.currentActive.Load(),
		WindowUtilization: utilization,
		CleanupOperations: l.cleanupOps.Load(),
		UptimeStart:       l.updateStart,
	}
}

// ResetMetrics resets all metrics counters.
func (l *Limiter) ResetMetrics() {
	l.totalRequests.Store(0)
	l.allowedRequests.Store(0)
	l.deniedRequests.Store(0)
	l.cleanupOps.Store(0)
	l.maxConcurrent.Store(0)
	l.maxResponseTime.Store(0)
	l.totalResponseTime.Store(0)

	l.mu.Lock()

	l.responseTimes = l.responseTimes[:0]
	l.responseTimesIdx = 0
	l.updateStart = time.Now()
	l.mu.Unlock()
}

// WaitTill blocks Allow, Check and Reserve until waitTime, e.g. to honour a
// server's Retry-After. An earlier time than an existing block is ignored.
func (l *Limiter) WaitTill(waitTime time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if waitTime.After(l.nextSlot) {
		l.nextSlot = waitTime
	}
}

// GetCapacityHint returns the current capacity hint for performance optimization.
func (l *Limiter) GetCapacityHint() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.capacityHint
}

// SetCapacityHint sets a new capacity hint for performance optimization.
func (l *Limiter) SetCapacityHint(hint int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.capacityHint = hint
}

// GetMetrics returns a snapshot of current limiter metrics.
// This function provides observability into rate limiter performance.
func GetMetrics(limiter any) LimiterMetrics {
	switch l := limiter.(type) {
	case *Limiter:
		return l.GetMetrics()
	default:
		// Return empty metrics for unsupported types
		return LimiterMetrics{UptimeStart: time.Now()}
	}
}
