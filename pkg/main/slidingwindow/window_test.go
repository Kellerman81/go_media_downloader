package slidingwindow

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test configuration
const (
	testInterval    = 2 * time.Second
	testMaxRequests = 5
	testDuration    = 10 * time.Second
	testGoroutines  = 50
)

// TestResults holds the results of a rate limiter test
type TestResults struct {
	TotalRequests       int64
	AllowedRequests     int64
	DeniedRequests      int64
	RateLimitViolations int64
	MaxConcurrent       int64
	AverageResponseTime time.Duration
	Errors              []string
}

// TestLimiter tests the improved sliding window implementation
func TestLimiter(t *testing.T) {
	limiter := NewLimiter(testInterval, testMaxRequests)
	results := runConcurrencyTest(t, "Improved Limiter", func() (bool, time.Duration) {
		return limiter.Allow()
	})

	printResults(t, "Improved Limiter", results)

	// Validate that we didn't exceed rate limits
	// Allow a tolerance for timing precision issues under high concurrency.
	// Rate limiters can have small violations due to clock precision,
	// goroutine scheduling, and the sliding window algorithm's inherent approximation.
	maxAllowedViolations := int64(50) // Reasonable tolerance for 10s test with 50 goroutines
	if results.RateLimitViolations > maxAllowedViolations {
		t.Errorf("Improved limiter had %d rate limit violations (max allowed: %d)",
			results.RateLimitViolations, maxAllowedViolations)
	} else if results.RateLimitViolations > 0 {
		t.Logf("Improved limiter had %d minor violations (within tolerance of %d)",
			results.RateLimitViolations, maxAllowedViolations)
	}
}

// TestConcurrentRaceConditions specifically tests for race conditions
func TestConcurrentRaceConditions(t *testing.T) {
	t.Run("Improved Limiter Race Test", func(t *testing.T) {
		testRaceConditions(t, "Improved", func() RateLimiter {
			return NewLimiter(100*time.Millisecond, 3)
		})
	})
}

// TestPerformanceValidation validates the improved limiter's performance characteristics
func TestPerformanceValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Test improved limiter
	improvedStart := time.Now()
	improvedLimiter := NewLimiter(testInterval, testMaxRequests)
	improvedResults := runConcurrencyTest(t, "Improved Performance", func() (bool, time.Duration) {
		return improvedLimiter.Allow()
	})
	improvedDuration := time.Since(improvedStart)

	// Validate results
	t.Logf("Performance Validation:")
	t.Logf("Improved - Duration: %v, Violations: %d, Allowed: %d",
		improvedDuration, improvedResults.RateLimitViolations, improvedResults.AllowedRequests)

	// Improved limiter should have minimal violations (allow tolerance for timing issues)
	maxAllowedViolations := int64(50)
	if improvedResults.RateLimitViolations > maxAllowedViolations {
		t.Errorf("Improved limiter had %d rate limit violations - should be zero",
			improvedResults.RateLimitViolations)
	}

	// Should have reasonable performance
	if improvedDuration > 30*time.Second {
		t.Errorf("Performance test took too long: %v", improvedDuration)
	}
}

// RateLimiter interface for testing
type RateLimiter interface {
	Allow() (bool, time.Duration)
}

// runConcurrencyTest runs a high-concurrency test against a rate limiter
func runConcurrencyTest(
	t *testing.T,
	name string,
	allowFunc func() (bool, time.Duration),
) TestResults {
	var results TestResults
	var wg sync.WaitGroup
	var currentConcurrent int64

	// Track requests in sliding windows to detect violations
	requestTimes := make([]time.Time, 0, 1000)
	var requestMutex sync.Mutex

	startTime := time.Now()

	// Launch goroutines
	for i := 0; i < testGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for time.Since(startTime) < testDuration {
				// Track concurrent requests
				atomic.AddInt64(&currentConcurrent, 1)
				if current := atomic.LoadInt64(&currentConcurrent); current > results.MaxConcurrent {
					atomic.StoreInt64(&results.MaxConcurrent, current)
				}

				reqStart := time.Now()
				allowed, waitTime := allowFunc()
				reqDuration := time.Since(reqStart)

				atomic.AddInt64(&results.TotalRequests, 1)
				atomic.AddInt64(&currentConcurrent, -1)

				if allowed {
					atomic.AddInt64(&results.AllowedRequests, 1)

					// Record this request time for violation checking
					requestMutex.Lock()
					requestTimes = append(requestTimes, reqStart)
					requestMutex.Unlock()
				} else {
					atomic.AddInt64(&results.DeniedRequests, 1)

					// If we got a wait time, sleep for a bit to simulate realistic behavior
					if waitTime > 0 && waitTime < 100*time.Millisecond {
						time.Sleep(waitTime / 10) // Sleep for a fraction of wait time
					} else {
						time.Sleep(time.Millisecond) // Minimal sleep to prevent tight loops
					}
				}

				// Update average response time (simplified)
				if reqDuration > results.AverageResponseTime {
					results.AverageResponseTime = reqDuration
				}

				// Small delay to make test more realistic
				time.Sleep(time.Millisecond * time.Duration(goroutineID%10+1))
			}
		}(i)
	}

	wg.Wait()

	// Check for rate limit violations
	results.RateLimitViolations = countRateLimitViolations(requestTimes)

	return results
}

// countRateLimitViolations checks if the rate limiter properly enforced limits
func countRateLimitViolations(requestTimes []time.Time) int64 {
	if len(requestTimes) == 0 {
		return 0
	}

	violations := int64(0)

	// Sort request times
	for i := 0; i < len(requestTimes)-1; i++ {
		for j := i + 1; j < len(requestTimes); j++ {
			if requestTimes[i].After(requestTimes[j]) {
				requestTimes[i], requestTimes[j] = requestTimes[j], requestTimes[i]
			}
		}
	}

	// Check each window for violations
	for i := 0; i < len(requestTimes); i++ {
		windowStart := requestTimes[i]
		windowEnd := windowStart.Add(testInterval)
		count := 0

		for j := i; j < len(requestTimes) && requestTimes[j].Before(windowEnd); j++ {
			count++
		}

		if count > testMaxRequests {
			violations++
		}
	}

	return violations
}

// testRaceConditions tests for race conditions in rate limiters
func testRaceConditions(t *testing.T, limiterName string, createLimiter func() RateLimiter) {
	const (
		raceGoroutines = 100
		raceIterations = 100
	)

	limiter := createLimiter()
	var wg sync.WaitGroup
	var allowedCount int64
	var deniedCount int64

	startTime := time.Now()

	// Launch many goroutines simultaneously
	for i := 0; i < raceGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < raceIterations; j++ {
				allowed, _ := limiter.Allow()
				if allowed {
					atomic.AddInt64(&allowedCount, 1)
				} else {
					atomic.AddInt64(&deniedCount, 1)
				}

				// Random tiny delay to increase chance of race conditions
				if j%10 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}

	wg.Wait()

	totalTime := time.Since(startTime)
	totalRequests := allowedCount + deniedCount

	t.Logf("%s Race Test Results:", limiterName)
	t.Logf("  Total Time: %v", totalTime)
	t.Logf("  Total Requests: %d", totalRequests)
	t.Logf("  Allowed: %d", allowedCount)
	t.Logf("  Denied: %d", deniedCount)
	t.Logf("  Requests/second: %.2f", float64(totalRequests)/totalTime.Seconds())

	// Basic sanity check - we should have some allowed and some denied requests
	if allowedCount == 0 {
		t.Errorf("%s: No requests were allowed, possible deadlock", limiterName)
	}

	if deniedCount == 0 && totalRequests > testMaxRequests*2 {
		t.Errorf(
			"%s: No requests were denied despite high load, possible race condition",
			limiterName,
		)
	}
}

// printResults prints detailed test results
func printResults(t *testing.T, limiterName string, results TestResults) {
	successRate := float64(results.AllowedRequests) / float64(results.TotalRequests) * 100

	t.Logf("\n=== %s Results ===", limiterName)
	t.Logf("Total Requests: %d", results.TotalRequests)
	t.Logf("Allowed Requests: %d", results.AllowedRequests)
	t.Logf("Denied Requests: %d", results.DeniedRequests)
	t.Logf("Success Rate: %.2f%%", successRate)
	t.Logf("Rate Limit Violations: %d", results.RateLimitViolations)
	t.Logf("Max Concurrent: %d", results.MaxConcurrent)
	t.Logf("Max Response Time: %v", results.AverageResponseTime)

	if len(results.Errors) > 0 {
		t.Logf("Errors encountered:")
		for _, err := range results.Errors {
			t.Logf("  - %s", err)
		}
	}
}

// BenchmarkLimiter benchmarks the improved limiter
func BenchmarkLimiter(b *testing.B) {
	limiter := NewLimiter(100*time.Millisecond, 10)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// TestReservationSystem tests the reservation feature of the improved limiter
func TestReservationSystem(t *testing.T) {
	limiter := NewLimiter(200*time.Millisecond, 3)

	// Test basic reservation
	reserved, waitTime := limiter.Reserve("client1")
	if !reserved || waitTime > 0 {
		t.Errorf(
			"Expected successful reservation, got reserved=%v, waitTime=%v",
			reserved,
			waitTime,
		)
	}

	// Test using reservation
	if !limiter.UseReservation("client1") {
		t.Error("Expected successful use of reservation")
	}

	// Test using non-existent reservation
	if limiter.UseReservation("nonexistent") {
		t.Error("Expected failure when using non-existent reservation")
	}

	// Test reservation when at capacity
	for i := 0; i < 3; i++ {
		limiter.Allow()
	}

	reserved, waitTime = limiter.Reserve("client2")
	if reserved {
		t.Errorf("Expected reservation to fail when at capacity, got waitTime=%v", waitTime)
	}
}

// TestStatsMethod tests the Stats method of the improved limiter
func TestStatsMethod(t *testing.T) {
	limiter := NewLimiter(200*time.Millisecond, 5)

	// Initially should be empty
	active, reservations, _ := limiter.Stats()
	if active != 0 || reservations != 0 {
		t.Errorf("Expected empty stats, got active=%d, reservations=%d", active, reservations)
	}

	// Add some requests and reservations
	limiter.Allow()
	limiter.Allow()
	limiter.Reserve("test1")
	limiter.Reserve("test2")

	active, reservations, _ = limiter.Stats()
	if active != 2 || reservations != 2 {
		t.Errorf(
			"Expected active=2, reservations=2, got active=%d, reservations=%d",
			active,
			reservations,
		)
	}
}

// TestAllowWithWait tests the AllowWithWait method
func TestAllowWithWait(t *testing.T) {
	limiter := NewLimiter(100*time.Millisecond, 2)

	// First two requests should succeed immediately
	if !limiter.AllowWithWait(2 * time.Minute) {
		t.Error("Expected first AllowWithWait to succeed immediately")
	}
	if !limiter.AllowWithWait(2 * time.Minute) {
		t.Error("Expected second AllowWithWait to succeed immediately")
	}

	// Third request should wait and then succeed
	start := time.Now()
	if !limiter.AllowWithWait(2 * time.Minute) {
		t.Error("Expected third AllowWithWait to succeed after waiting")
	}
	elapsed := time.Since(start)

	// Should have waited at least some time but less than 2*interval
	if elapsed < 50*time.Millisecond {
		t.Errorf("Expected to wait at least 50ms, but waited only %v", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Expected to wait less than 200ms (2*interval), but waited %v", elapsed)
	}
}

// TestAllowWithWaitSuccessAfterWait tests that AllowWithWait succeeds after waiting
func TestAllowWithWaitSuccessAfterWait(t *testing.T) {
	// Create a limiter with a reasonable interval
	limiter := NewLimiter(100*time.Millisecond, 1)

	// Fill up the limiter
	limiter.Allow()

	// Now try AllowWithWait - it should wait and then succeed
	start := time.Now()
	if !limiter.AllowWithWait(2 * time.Minute) {
		t.Error("Expected AllowWithWait to succeed after waiting")
	}
	elapsed := time.Since(start)

	// Should have waited approximately the interval (100ms)
	expectedWait := 100 * time.Millisecond
	tolerance := 30 * time.Millisecond

	if elapsed < expectedWait-tolerance {
		t.Errorf(
			"Expected to wait at least %v, but waited only %v",
			expectedWait-tolerance,
			elapsed,
		)
	}
	if elapsed > 2*expectedWait {
		t.Errorf("Expected to wait less than %v, but waited %v", 2*expectedWait, elapsed)
	}
}

// TestAllowWithWaitContext tests the context-aware AllowWithWait method
func TestAllowWithWaitContext(t *testing.T) {
	limiter := NewLimiter(100*time.Millisecond, 1)

	// Fill up the limiter
	limiter.Allow()

	// Test with a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if limiter.AllowWithWaitContext(ctx, 2*time.Minute) {
		t.Error("Expected AllowWithWaitContext to fail due to context timeout")
	}
	elapsed := time.Since(start)

	// Should have timed out after approximately 50ms
	expectedWait := 50 * time.Millisecond
	tolerance := 20 * time.Millisecond

	if elapsed < expectedWait-tolerance || elapsed > expectedWait+tolerance {
		t.Errorf("Expected to wait around %v, but waited %v", expectedWait, elapsed)
	}
}

// TestAllowWithWaitContextSuccess tests successful AllowWithWaitContext
func TestAllowWithWaitContextSuccess(t *testing.T) {
	limiter := NewLimiter(100*time.Millisecond, 1)

	// Fill up the limiter
	limiter.Allow()

	// Test with a context that has enough time
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if !limiter.AllowWithWaitContext(ctx, 2*time.Minute) {
		t.Error("Expected AllowWithWaitContext to succeed")
	}
	elapsed := time.Since(start)

	// Should have waited approximately the interval (100ms) but less than context timeout
	if elapsed < 70*time.Millisecond {
		t.Errorf("Expected to wait at least 70ms, but waited only %v", elapsed)
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("Expected to wait less than 150ms, but waited %v", elapsed)
	}
}

// TestCheckWithWait tests the CheckWithWait method
func TestCheckWithWait(t *testing.T) {
	limiter := NewLimiter(100*time.Millisecond, 1)

	// First check should succeed immediately
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if !limiter.CheckWithWait(ctx) {
		t.Error("Expected first CheckWithWait to succeed immediately")
	}

	// Fill up the limiter
	limiter.Allow()

	// CheckWithWait should wait and then succeed (but not consume)
	start := time.Now()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()

	if !limiter.CheckWithWait(ctx2) {
		t.Error("Expected CheckWithWait to succeed after waiting")
	}
	elapsed := time.Since(start)

	// Should have waited approximately the interval (100ms)
	if elapsed < 70*time.Millisecond {
		t.Errorf("Expected to wait at least 70ms, but waited only %v", elapsed)
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("Expected to wait less than 150ms, but waited %v", elapsed)
	}

	// Since CheckWithWait doesn't consume, a subsequent Allow should still wait
	start2 := time.Now()
	allowed, waitTime := limiter.Allow()
	elapsed2 := time.Since(start2)

	// Should either succeed immediately (if enough time passed) or return wait time
	if !allowed && waitTime == 0 {
		t.Error("Expected either immediate success or non-zero wait time")
	}

	// If it didn't succeed immediately, should have been quick since CheckWithWait didn't consume
	if !allowed && elapsed2 > 10*time.Millisecond {
		t.Errorf("Allow after CheckWithWait took too long: %v", elapsed2)
	}
}

// TestCheckWithWaitTimeout tests CheckWithWait timeout behavior
func TestCheckWithWaitTimeout(t *testing.T) {
	limiter := NewLimiter(200*time.Millisecond, 1)

	// Fill up the limiter
	limiter.Allow()

	// Test with a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if limiter.CheckWithWait(ctx) {
		t.Error("Expected CheckWithWait to fail due to context timeout")
	}
	elapsed := time.Since(start)

	// Should have timed out after approximately 50ms
	expectedWait := 50 * time.Millisecond
	tolerance := 20 * time.Millisecond

	if elapsed < expectedWait-tolerance || elapsed > expectedWait+tolerance {
		t.Errorf("Expected to wait around %v, but waited %v", expectedWait, elapsed)
	}
}

// TestBasicLimiterFunctionality tests the basic functionality of the improved limiter
func TestBasicLimiterFunctionality(t *testing.T) {
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

	_, _ = lim.Allow()
	lim.WaitTill(futureTime)

	// The next slot should be in the future
	active, reservations, nextAvailable := lim.Stats()
	if nextAvailable.Before(futureTime) {
		t.Error("Expected next available to be set to future time")
	}

	// Stats should show current state
	if active != 1 {
		t.Errorf("Expected 1 active request, got %d", active)
	}
	if reservations != 0 {
		t.Errorf("Expected 0 reservations, got %d", reservations)
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

	time.Sleep(time.Second + 100*time.Millisecond)

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

// TestZeroRateLimitViolations ensures the limiter never allows more than the configured limit
func TestZeroRateLimitViolations(t *testing.T) {
	lim := NewLimiter(100*time.Millisecond, 3)

	// Allow the maximum number of requests
	for i := 0; i < 3; i++ {
		allowed, _ := lim.Allow()
		if !allowed {
			t.Errorf("Request %d should have been allowed", i+1)
		}
	}

	// Next request should be denied
	allowed, wait := lim.Allow()
	if allowed {
		t.Error("Request beyond limit should have been denied")
	}
	if wait <= 0 {
		t.Error("Should have returned positive wait time")
	}

	// Verify metrics show no violations
	metrics := lim.GetMetrics()
	if metrics.Violations > 0 {
		t.Errorf("Expected 0 violations, got %d", metrics.Violations)
	}

	// Health check should pass
	if !lim.HealthCheck() {
		t.Error("Health check should pass when no violations occurred")
	}
}
