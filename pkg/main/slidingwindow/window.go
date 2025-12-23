package slidingwindow

import (
	"context"
	"runtime"
	"sort"
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
	Violations      int64

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

	// Health metrics
	LastViolationTime time.Time
	UptimeStart       time.Time
}

const (
	defaultsleeptime   = 100  // in Milliseconds
	maxResponseSamples = 1000 // Maximum number of response time samples to keep
)

// StatusCallback is called when limiter status changes.
type StatusCallback func(limiter *Limiter, status LimiterStatus)

// AlertCallback is called when alerts are triggered.
type AlertCallback func(limiter *Limiter, alert Alert)

// MetricsCallback is called periodically with current metrics.
type MetricsCallback func(limiter *Limiter, metrics LimiterMetrics)

// LimiterStatus represents the current status of the limiter.
type LimiterStatus struct {
	Healthy         bool
	Active          int64
	Utilization     float64
	AvgResponseTime time.Duration
	LastViolation   time.Time
}

// Alert represents an alert condition.
type Alert struct {
	Type      AlertType
	Message   string
	Timestamp time.Time
	Severity  AlertSeverity
	Metrics   LimiterMetrics
}

// AlertType represents different types of alerts.
type AlertType int

const (
	AlertViolation AlertType = iota
	AlertHighUtilization
	AlertHighResponseTime
	AlertUnhealthy
	AlertRecovered
)

// AlertSeverity represents alert severity levels.
type AlertSeverity int

const (
	SeverityInfo AlertSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// String returns string representation of AlertType.
func (a AlertType) String() string {
	switch a {
	case AlertViolation:
		return "VIOLATION"
	case AlertHighUtilization:
		return "HIGH_UTILIZATION"
	case AlertHighResponseTime:
		return "HIGH_RESPONSE_TIME"
	case AlertUnhealthy:
		return "UNHEALTHY"
	case AlertRecovered:
		return "RECOVERED"
	default:
		return "UNKNOWN"
	}
}

// String returns string representation of AlertSeverity.
func (s AlertSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Limiter provides a high-performance sliding window rate limiter
// with comprehensive metrics, observability, and advanced features.
type Limiter struct {
	// Core rate limiting with segmented locking
	mu          sync.RWMutex // Main lock for structural changes
	callbacksMu sync.RWMutex // Separate lock for callbacks
	interval    time.Duration
	max         int64

	// True sliding window implementation
	requests []time.Time

	// Reservation system for advanced use cases
	reservations map[string]time.Time // client ID -> reservation time
	nextSlot     time.Time            // when the next slot becomes available

	// Performance optimizations
	lastCleanup      time.Time // Last time cleanup was performed
	cleanupInterval  time.Duration
	capacityHint     int  // Hint for initial slice capacity
	cleanupThreshold int  // Number of requests before forcing cleanup
	fastPath         bool // Use optimized fast path for common cases

	// Comprehensive metrics (atomic counters)
	totalRequests   int64
	allowedRequests int64
	deniedRequests  int64
	violations      int64
	cleanupOps      int64
	maxConcurrent   int64
	currentActive   int64

	// Response time tracking
	responseTimes     []time.Duration
	responseTimesIdx  int
	maxResponseTime   int64 // nanoseconds
	totalResponseTime int64 // nanoseconds for average calculation

	// Health and observability
	updateStart   time.Time
	lastViolation time.Time
	healthy       int32 // atomic boolean
	lastAlert     time.Time
	alertCount    int64

	// Enhanced observability
	statusCallbacks  []StatusCallback
	alertCallbacks   []AlertCallback
	metricsCallbacks []MetricsCallback
}

// NewLimiter creates a new improved sliding window rate limiter
// with comprehensive metrics and performance optimizations.
func NewLimiter(interval time.Duration, maxRequests int64) *Limiter {
	capacity := int(maxRequests)
	if capacity < 10 {
		capacity = 10
	}

	limiter := &Limiter{
		interval:         interval,
		max:              maxRequests,
		requests:         make([]time.Time, 0, capacity),
		reservations:     make(map[string]time.Time),
		cleanupInterval:  interval / 4, // Cleanup every quarter interval
		capacityHint:     capacity,
		cleanupThreshold: capacity * 2,       // Force cleanup when requests exceed 2x capacity
		fastPath:         maxRequests <= 100, // Use fast path for small limits
		responseTimes:    make([]time.Duration, 0, maxResponseSamples),
		updateStart:      time.Now(),
		healthy:          1, // Start healthy
		lastCleanup:      time.Now(),
		statusCallbacks:  make([]StatusCallback, 0),
		alertCallbacks:   make([]AlertCallback, 0),
		metricsCallbacks: make([]MetricsCallback, 0),
	}

	// Pre-allocate slice to prevent reallocations during normal operation
	if capacity > 0 {
		limiter.requests = make([]time.Time, 0, capacity*2)
	}

	return limiter
}

// cleanupOldRequests removes requests older than the window with performance optimizations.
func (l *Limiter) cleanupOldRequests(now time.Time) {
	// Advanced cleanup logic based on performance characteristics
	forceCleanup := len(l.requests) >= l.cleanupThreshold
	timeBasedCleanup := now.Sub(l.lastCleanup) >= l.cleanupInterval

	// Only cleanup if necessary
	if !forceCleanup && !timeBasedCleanup {
		return
	}

	cutoff := now.Add(-l.interval)

	// Fast path optimization for small request counts
	if l.fastPath && len(l.requests) <= int(l.max)*2 {
		// Simple linear scan for small arrays (better cache performance)
		validStart := 0
		for i, reqTime := range l.requests {
			if reqTime.After(cutoff) {
				validStart = i
				break
			}

			validStart = i + 1
		}

		// Optimized slice operation
		if validStart > 0 {
			if validStart < len(l.requests) {
				// Use copy for better performance
				n := copy(l.requests, l.requests[validStart:])

				l.requests = l.requests[:n]
			} else {
				// All requests are old - reset slice
				l.requests = l.requests[:0]
			}
		}
	} else {
		// Binary search optimization for large arrays
		validStart := l.binarySearchCleanup(cutoff)
		if validStart > 0 {
			if validStart < len(l.requests) {
				n := copy(l.requests, l.requests[validStart:])

				l.requests = l.requests[:n]
			} else {
				l.requests = l.requests[:0]
			}
		}
	}

	// Clean up old reservations efficiently
	if len(l.reservations) > 0 {
		for id, resTime := range l.reservations {
			if resTime.Before(cutoff) {
				delete(l.reservations, id)
			}
		}
	}

	// Update cleanup timestamp and metrics
	l.lastCleanup = now
	atomic.AddInt64(&l.cleanupOps, 1)

	// Trigger GC hint if we freed significant memory
	if forceCleanup && len(l.requests) < l.cleanupThreshold/4 {
		runtime.GC()
	}
}

// binarySearchCleanup performs binary search to find first valid request.
func (l *Limiter) binarySearchCleanup(cutoff time.Time) int {
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

// Allow attempts to allow a request immediately with comprehensive metrics tracking.
func (l *Limiter) Allow() (bool, time.Duration) {
	start := time.Now()

	atomic.AddInt64(&l.totalRequests, 1)

	// Update current active counter
	active := atomic.AddInt64(&l.currentActive, 1)
	defer atomic.AddInt64(&l.currentActive, -1)

	// Update max concurrent if necessary (lock-free)
	for {
		maxCurrent := atomic.LoadInt64(&l.maxConcurrent)
		if active <= maxCurrent ||
			atomic.CompareAndSwapInt64(&l.maxConcurrent, maxCurrent, active) {
			break
		}
	}

	// Fast path optimization for small limits
	if l.fastPath {
		// Quick read-only check for obviously under-limit cases
		l.mu.RLock()

		currentCount := int64(len(l.requests))
		l.mu.RUnlock()

		// If we're clearly under half the limit, try fast path
		if currentCount < l.max/2 {
			// Use full lock for actual modification
			l.mu.Lock()

			now := time.Now()
			// Double-check under full lock
			if int64(len(l.requests)) < l.max {
				l.requests = append(l.requests, now)
				atomic.AddInt64(&l.allowedRequests, 1)
				l.recordResponseTime(start, now)
				l.mu.Unlock()

				return true, 0
			}

			l.mu.Unlock()
		}
	}

	// Full path with cleanup and comprehensive checking
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cleanupOldRequests(now)

	// Check if we have capacity
	if int64(len(l.requests)) < l.max {
		l.requests = append(l.requests, now)
		atomic.AddInt64(&l.allowedRequests, 1)
		l.recordResponseTime(start, now)

		return true, 0
	}

	// Calculate when next slot becomes available
	if len(l.requests) > 0 {
		oldest := l.requests[0]

		waitTime := oldest.Add(l.interval).Sub(now)
		if waitTime > 0 {
			atomic.AddInt64(&l.deniedRequests, 1)
			l.recordResponseTime(start, now)

			return false, waitTime
		}
	}

	// If we get here, cleanup should have made space
	l.requests = append(l.requests, now)
	atomic.AddInt64(&l.allowedRequests, 1)
	l.recordResponseTime(start, now)

	return true, 0
}

// Check returns whether a request would be allowed without consuming a slot.
func (l *Limiter) Check() (bool, time.Duration) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()

	// Use read-only check without cleanup for performance
	cutoff := now.Add(-l.interval)

	activeRequests := int64(0)
	for _, reqTime := range l.requests {
		if reqTime.After(cutoff) {
			activeRequests++
		}
	}

	// Check if we have capacity
	if activeRequests < l.max {
		return true, 0
	}

	// Calculate when next slot becomes available
	if len(l.requests) > 0 {
		// Find oldest active request
		for _, reqTime := range l.requests {
			if !reqTime.After(cutoff) {
				continue
			}

			waitTime := reqTime.Add(l.interval).Sub(now)
			if waitTime > 0 {
				return false, waitTime
			}

			break
		}
	}

	return true, 0
}

// Reserve attempts to reserve a slot for future use with enhanced tracking.
func (l *Limiter) Reserve(id string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cleanupOldRequests(now)

	// Check if already reserved
	if _, exists := l.reservations[id]; exists {
		return true, 0
	}

	// Check if we have capacity (including reservations)
	totalUsed := int64(len(l.requests) + len(l.reservations))
	if totalUsed < l.max {
		l.reservations[id] = now
		return true, 0
	}

	// Calculate when next slot becomes available
	if len(l.requests) > 0 {
		oldest := l.requests[0]
		waitTime := oldest.Add(l.interval).Sub(now)
		return false, waitTime
	}

	return false, l.interval
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
	atomic.AddInt64(&l.allowedRequests, 1)
	atomic.AddInt64(&l.totalRequests, 1)

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
	for _, reqTime := range l.requests {
		if reqTime.After(cutoff) {
			activeCount++
		}
	}

	return int64(activeCount), int64(len(l.reservations)), l.nextSlot
}

// AllowForce adds a request regardless of limits (for compatibility with original).
func (l *Limiter) AllowForce() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.requests = append(l.requests, time.Now())

	return true
}

// CheckBool returns whether a request would be allowed (compatibility method).
func (l *Limiter) CheckBool() bool {
	allowed, _ := l.Check()
	return allowed
}

// Interval returns the interval duration configured for the rate limiter.
func (l *Limiter) Interval() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	minSleepTime := l.interval / time.Duration(l.max*4)
	if minSleepTime < time.Millisecond {
		minSleepTime = time.Millisecond
	}

	if minSleepTime > defaultsleeptime*time.Millisecond {
		minSleepTime = defaultsleeptime * time.Millisecond
	}

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
		sleepTime := waitFor
		if sleepTime < minSleepTime {
			sleepTime = minSleepTime
		}

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

			sleepTime = time.Duration(float64(sleepTime) * backoffMultiplier)
			if sleepTime > remainingWait {
				sleepTime = remainingWait
			}
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
	minSleepTime := l.interval / time.Duration(l.max*4)
	if minSleepTime < time.Millisecond {
		minSleepTime = time.Millisecond
	}

	if minSleepTime > defaultsleeptime*time.Millisecond {
		minSleepTime = defaultsleeptime * time.Millisecond
	}

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
		sleepTime := waitFor
		if sleepTime < minSleepTime {
			sleepTime = minSleepTime
		}

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
		maxCurrent := atomic.LoadInt64(&l.maxResponseTime)
		if nanos <= maxCurrent ||
			atomic.CompareAndSwapInt64(&l.maxResponseTime, maxCurrent, nanos) {
			break
		}
	}

	// Update total response time for average calculation
	atomic.AddInt64(&l.totalResponseTime, nanos)

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
	for _, reqTime := range l.requests {
		if reqTime.After(cutoff) {
			activeCount++
		}
	}

	utilization := float64(activeCount) / float64(l.max)

	// Calculate response time metrics
	totalReqs := atomic.LoadInt64(&l.totalRequests)

	avgResponseTime := time.Duration(0)
	if totalReqs > 0 {
		avgResponseTime = time.Duration(atomic.LoadInt64(&l.totalResponseTime) / totalReqs)
	}

	// Calculate P99 response time (optimized)
	p99ResponseTime := time.Duration(atomic.LoadInt64(&l.maxResponseTime))
	if len(l.responseTimes) > 10 {
		// Sort a copy and get 99th percentile
		sortedTimes := make([]time.Duration, len(l.responseTimes))
		copy(sortedTimes, l.responseTimes)
		// Use built-in sort for better performance
		sort.Slice(sortedTimes, func(i, j int) bool {
			return sortedTimes[i] < sortedTimes[j]
		})

		p99Index := int(float64(len(sortedTimes)) * 0.99)
		if p99Index >= len(sortedTimes) {
			p99Index = len(sortedTimes) - 1
		}

		if p99Index >= 0 {
			p99ResponseTime = sortedTimes[p99Index]
		}
	}

	return LimiterMetrics{
		TotalRequests:     atomic.LoadInt64(&l.totalRequests),
		AllowedRequests:   atomic.LoadInt64(&l.allowedRequests),
		DeniedRequests:    atomic.LoadInt64(&l.deniedRequests),
		Violations:        atomic.LoadInt64(&l.violations),
		AvgResponseTime:   avgResponseTime,
		MaxResponseTime:   time.Duration(atomic.LoadInt64(&l.maxResponseTime)),
		P99ResponseTime:   p99ResponseTime,
		MaxConcurrent:     atomic.LoadInt64(&l.maxConcurrent),
		CurrentActive:     atomic.LoadInt64(&l.currentActive),
		WindowUtilization: utilization,
		CleanupOperations: atomic.LoadInt64(&l.cleanupOps),
		LastViolationTime: l.lastViolation,
		UptimeStart:       l.updateStart,
	}
}

// HealthCheck performs a comprehensive health check of the rate limiter.
func (l *Limiter) HealthCheck() bool {
	// Check if we're healthy
	if atomic.LoadInt32(&l.healthy) == 0 {
		return false
	}

	// Check for recent violations
	if !l.lastViolation.IsZero() && time.Since(l.lastViolation) < time.Minute {
		return false
	}

	// Check if metrics look reasonable
	metrics := l.GetMetrics()
	if metrics.Violations > 0 {
		// Mark as unhealthy if we have violations
		atomic.StoreInt32(&l.healthy, 0)
		return false
	}

	return true
}

// ResetMetrics resets all metrics counters.
func (l *Limiter) ResetMetrics() {
	atomic.StoreInt64(&l.totalRequests, 0)
	atomic.StoreInt64(&l.allowedRequests, 0)
	atomic.StoreInt64(&l.deniedRequests, 0)
	atomic.StoreInt64(&l.violations, 0)
	atomic.StoreInt64(&l.cleanupOps, 0)
	atomic.StoreInt64(&l.maxConcurrent, 0)
	atomic.StoreInt64(&l.maxResponseTime, 0)
	atomic.StoreInt64(&l.totalResponseTime, 0)

	l.mu.Lock()

	l.responseTimes = l.responseTimes[:0]
	l.responseTimesIdx = 0
	l.updateStart = time.Now()
	l.lastViolation = time.Time{}
	l.mu.Unlock()

	atomic.StoreInt32(&l.healthy, 1)
}

// WaitTill sets the next available slot time to the given time.
func (l *Limiter) WaitTill(waitTime time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextSlot = waitTime
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

// AddStatusCallback adds a callback that will be called when status changes.
func (l *Limiter) AddStatusCallback(callback StatusCallback) {
	l.callbacksMu.Lock()
	defer l.callbacksMu.Unlock()

	l.statusCallbacks = append(l.statusCallbacks, callback)
}

// AddAlertCallback adds a callback that will be called when alerts are triggered.
func (l *Limiter) AddAlertCallback(callback AlertCallback) {
	l.callbacksMu.Lock()
	defer l.callbacksMu.Unlock()

	l.alertCallbacks = append(l.alertCallbacks, callback)
}

// AddMetricsCallback adds a callback that will be called periodically with metrics.
func (l *Limiter) AddMetricsCallback(callback MetricsCallback) {
	l.callbacksMu.Lock()
	defer l.callbacksMu.Unlock()

	l.metricsCallbacks = append(l.metricsCallbacks, callback)
}

// getCurrentStatus returns the current status of the limiter.
func (l *Limiter) getCurrentStatus() LimiterStatus {
	metrics := l.GetMetrics()

	return LimiterStatus{
		Healthy:         atomic.LoadInt32(&l.healthy) == 1,
		Active:          metrics.CurrentActive,
		Utilization:     metrics.WindowUtilization,
		AvgResponseTime: metrics.AvgResponseTime,
		LastViolation:   l.lastViolation,
	}
}

// GetAlertCount returns the total number of alerts triggered.
func (l *Limiter) GetAlertCount() int64 {
	return atomic.LoadInt64(&l.alertCount)
}

// GetLastAlert returns the timestamp of the last alert.
func (l *Limiter) GetLastAlert() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastAlert
}

// GetStatus returns the current status of the limiter.
func (l *Limiter) GetStatus() LimiterStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.getCurrentStatus()
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

// HealthCheck performs a health check on the rate limiter.
// Returns true if the limiter is functioning correctly.
func HealthCheck(limiter any) bool {
	switch l := limiter.(type) {
	case *Limiter:
		return l.HealthCheck()
	default:
		return false
	}
}
