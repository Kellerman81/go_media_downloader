package database

import (
	"sync"
	"testing"
	"time"
)

// TestGlobalCacheRaceConditions tests the thread safety of globalCache operations
// Run with: go test -race -run TestGlobalCacheRaceConditions
func TestGlobalCacheRaceConditions(t *testing.T) {
	// Initialize a test global cache
	testCache := &globalcache{
		defaultextension: time.Hour,
	}
	
	t.Run("ConcurrentDefaultExtensionAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 50
		iterations := 100
		
		// Test concurrent reads and writes to defaultextension
		for i := 0; i < numGoroutines; i++ {
			wg.Add(2)
			
			// Reader goroutine
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					// This should use thread-safe getexpiressql
					_ = testCache.getexpiressql(false)
					_ = testCache.getexpiressql(true)
				}
			}(i)
			
			// Writer goroutine (simulates configuration updates)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					// Simulate updating the default extension
					func() {
						testCache.mu.Lock()
						testCache.defaultextension = time.Duration(id+j) * time.Minute
						testCache.mu.Unlock()
					}()
				}
			}(i)
		}
		
		wg.Wait()
	})
	
	t.Run("ConcurrentRegexCacheAccess", func(t *testing.T) {
		// Skip this test if it requires full system initialization
		t.Skip("Skipping regex cache test - requires full database initialization")
	})
	
	t.Run("MixedCacheOperations", func(t *testing.T) {
		// Initialize cache if not already done
		NewCache(time.Hour, time.Hour)
		
		var wg sync.WaitGroup
		numOperations := 1000
		
		// Mix of different cache operations (only test safe operations)
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				switch id % 2 {
				case 0:
					// Test getexpiressql
					if globalCache != nil {
						_ = globalCache.getexpiressql(id%2 == 0)
					}
				case 1:
					// Test configuration read/write
					if globalCache != nil {
						globalCache.mu.Lock()
						old := globalCache.defaultextension
						globalCache.defaultextension = old + time.Millisecond
						globalCache.mu.Unlock()
					}
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// TestGlobalCacheStress performs stress testing under high load
func TestGlobalCacheStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	// Initialize cache
	NewCache(time.Minute, time.Hour)
	
	var wg sync.WaitGroup
	duration := 2 * time.Second
	
	// Start multiple goroutines doing different operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			operations := 0
			
			for time.Since(start) < duration && globalCache != nil {
				switch operations % 3 {
				case 0:
					_ = globalCache.getexpiressql(false)
				case 1:
					_ = globalCache.getexpiressql(true)
				case 2:
					// Configuration read/write
					globalCache.mu.Lock()
					globalCache.defaultextension = time.Duration(operations) * time.Millisecond
					globalCache.mu.Unlock()
				}
				operations++
			}
			
			t.Logf("Worker %d completed %d operations", workerID, operations)
		}(i)
	}
	
	wg.Wait()
}

// BenchmarkGlobalCacheConcurrency benchmarks concurrent access to globalCache
func BenchmarkGlobalCacheConcurrency(b *testing.B) {
	NewCache(time.Hour, time.Hour)
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() && globalCache != nil {
			switch i % 2 {
			case 0:
				_ = globalCache.getexpiressql(false)
			case 1:
				globalCache.mu.RLock()
				_ = globalCache.defaultextension
				globalCache.mu.RUnlock()
			}
			i++
		}
	})
}

// TestGlobalCacheDataRaces specifically tests for data races in field access
func TestGlobalCacheDataRaces(t *testing.T) {
	testCache := &globalcache{
		defaultextension: time.Hour,
	}
	
	var wg sync.WaitGroup
	
	// Start readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				// Safe read
				testCache.mu.RLock()
				_ = testCache.defaultextension
				testCache.mu.RUnlock()
			}
		}()
	}
	
	// Start writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				// Safe write
				testCache.mu.Lock()
				testCache.defaultextension = time.Duration(id*j) * time.Millisecond
				testCache.mu.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
}