package database

import (
	"sync"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
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
		for i := range numGoroutines {
			wg.Add(2)

			// Reader goroutine
			go func(id int) {
				defer wg.Done()
				for range iterations {
					// This should use thread-safe getexpiressql
					_ = testCache.getexpiressql(false)
					_ = testCache.getexpiressql(true)
				}
			}(i)

			// Writer goroutine (simulates configuration updates)
			go func(id int) {
				defer wg.Done()
				for j := range iterations {
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
		for i := range numOperations {
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
	for i := range 10 {
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
	for range 10 {
		wg.Go(func() {
			for range 1000 {
				// Safe read
				testCache.mu.RLock()
				_ = testCache.defaultextension
				testCache.mu.RUnlock()
			}
		})
	}

	// Start writers
	for i := range 5 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 500 {
				// Safe write
				testCache.mu.Lock()
				testCache.defaultextension = time.Duration(id*j) * time.Millisecond
				testCache.mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
}

func TestClearCacheTypeOnlyClearsTargetKey(t *testing.T) {
	NewCache(time.Hour, time.Hour)

	cache.itemstwoint.Add(logger.CacheMovie, []syncops.DbstaticOneStringTwoInt{{Str: "l", Num1: 1, Num2: 2}}, 0, false, 0)
	cache.itemsstring.Add(logger.CacheFilesMovie, []string{"/a"}, 0, false, 0)

	t.Cleanup(func() {
		cache.itemstwoint.Delete(logger.CacheMovie)
		cache.itemsstring.Delete(logger.CacheFilesMovie)
	})

	ClearCacheType(logger.CacheMovie)

	if cache.itemstwoint.Check(logger.CacheMovie) {
		t.Error("CacheMovie still cached after ClearCacheType(CacheMovie)")
	}

	if !cache.itemsstring.Check(logger.CacheFilesMovie) {
		t.Error("ClearCacheType(CacheMovie) also wiped CacheFilesMovie")
	}
}

func TestDropOrphanedIndexes(t *testing.T) {
	NewCache(time.Hour, time.Hour)

	const live, gone = "zzLiveKey", "zzGoneKey"

	cache.itemsstring.Add(live, []string{"a"}, 0, false, 0)
	cache.indexStringSet.Add(live, map[string]struct{}{"a": {}}, 0, false, 0)
	cache.indexStringSet.Add(gone, map[string]struct{}{"b": {}}, 0, false, 0)

	t.Cleanup(func() {
		cache.itemsstring.Delete(live)
		cache.indexStringSet.Delete(live)
		cache.indexStringSet.Delete(gone)
	})

	dropOrphanedIndexes()

	if !cache.indexStringSet.Check(live) {
		t.Error("index of a live cache entry was dropped")
	}

	if cache.indexStringSet.Check(gone) {
		t.Error("index without cache entry survived")
	}
}
