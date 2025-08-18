package worker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// TestData represents test data for syncMapUint race tests
type TestData struct {
	ID    uint32
	Name  string
	Value int64
}

// TestSyncMapUintRaceConditions tests the thread safety of syncMapUint operations
// Run with: go test -race -run TestSyncMapUintRaceConditions
func TestSyncMapUintRaceConditions(t *testing.T) {
	t.Run("ConcurrentAddGetDelete", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](100)

		var wg sync.WaitGroup
		numGoroutines := 50
		iterations := 100

		// Writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					key := uint32(id*iterations + j)
					data := TestData{
						ID:    key,
						Name:  fmt.Sprintf("item_%d_%d", id, j),
						Value: int64(id + j),
					}
					sm.Add(key, data)
				}
			}(i)
		}

		// Readers
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations*2; j++ {
					key := uint32(j % (numGoroutines * iterations))
					_ = sm.GetVal(key)
					_ = sm.Check(key)
				}
			}(i)
		}

		// Deleters
		for i := 0; i < numGoroutines/4; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					key := uint32((id + j) % (numGoroutines * iterations))
					sm.Delete(key)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("ConcurrentUpdateVal", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](50)

		// Pre-populate with some data
		for i := uint32(0); i < 50; i++ {
			sm.Add(i, TestData{ID: i, Name: "initial", Value: 0})
		}

		var wg sync.WaitGroup
		var updates int64

		// Multiple updaters
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					key := uint32(j % 50)
					data := TestData{
						ID:    key,
						Name:  fmt.Sprintf("updated_by_%d_%d", id, j),
						Value: int64(id*1000 + j),
					}
					sm.UpdateVal(key, data)
					atomic.AddInt64(&updates, 1)
				}
			}(i)
		}

		// Concurrent readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 200; j++ {
					key := uint32(j % 50)
					data := sm.GetVal(key)
					_ = data.Name // Use the data
				}
			}()
		}

		wg.Wait()
		t.Logf("Completed %d updates", updates)
	})

	t.Run("ConcurrentGetMap", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := uint32(0); i < 100; i++ {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i)})
		}

		var wg sync.WaitGroup

		// Multiple GetMap calls
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					mapCopy := sm.GetMap()
					// Map can grow beyond 100 due to concurrent additions
					if len(mapCopy) > 200 {
						t.Errorf("Map copy size unexpected: %d", len(mapCopy))
					}
				}
			}(i)
		}

		// Concurrent modifications
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 25; j++ {
					key := uint32(100 + id*25 + j)
					sm.Add(key, TestData{ID: key, Name: "new_item", Value: int64(key)})
					sm.Delete(key)
				}
			}(i)
		}

		wg.Wait()
	})
}

// TestSyncMapUintAdvancedOperations tests the new advanced methods
func TestSyncMapUintAdvancedOperations(t *testing.T) {
	t.Run("ConcurrentForEach", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](50)

		// Pre-populate
		for i := uint32(0); i < 50; i++ {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i)})
		}

		var wg sync.WaitGroup
		var forEachCalls int64

		// Multiple ForEach operations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					sm.ForEach(func(key uint32, data TestData) {
						atomic.AddInt64(&forEachCalls, 1)
						_ = data.Name // Use the data
					})
				}
			}()
		}

		// Concurrent modifications
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					key := uint32(50 + id*10 + j)
					sm.Add(key, TestData{ID: key, Name: "concurrent", Value: int64(key)})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("ForEach called %d times", forEachCalls)
	})

	t.Run("ConcurrentDeleteIf", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := uint32(0); i < 100; i++ {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i % 10)})
		}

		var wg sync.WaitGroup

		// Multiple DeleteIf operations
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Delete items with specific criteria
				sm.DeleteIf(func(key uint32, data TestData) bool {
					return data.Value == int64(id)
				})
			}(i)
		}

		// Concurrent readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					key := uint32(j % 100)
					_ = sm.Check(key)
				}
			}()
		}

		wg.Wait()

		// Check final state
		finalMap := sm.GetMap()
		t.Logf("Final map size: %d", len(finalMap))
	})

	t.Run("ConcurrentFindFirst", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := uint32(0); i < 100; i++ {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i % 20)})
		}

		var wg sync.WaitGroup
		var foundCount int64

		// Multiple FindFirst operations
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					searchValue := int64(j % 20)
					_, _, found := sm.FindFirst(func(key uint32, data TestData) bool {
						return data.Value == searchValue
					})
					if found {
						atomic.AddInt64(&foundCount, 1)
					}
				}
			}(i)
		}

		// Concurrent modifications
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					key := uint32(100 + id*20 + j)
					sm.Add(key, TestData{ID: key, Name: "new", Value: int64(j % 5)})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Found items %d times", foundCount)
	})

	t.Run("ConcurrentExists", func(t *testing.T) {
		sm := syncops.NewSyncMapUint[TestData](50)

		// Pre-populate
		for i := uint32(0); i < 50; i++ {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("special_%d", i%5), Value: int64(i)})
		}

		var wg sync.WaitGroup
		var existsCount int64

		// Multiple Exists operations
		for i := 0; i < 15; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					searchName := fmt.Sprintf("special_%d", j%5)
					exists := sm.Exists(func(key uint32, data TestData) bool {
						return data.Name == searchName
					})
					if exists {
						atomic.AddInt64(&existsCount, 1)
					}
				}
			}(i)
		}

		// Concurrent deletions
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					key := uint32(id*10 + j)
					sm.Delete(key)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Exists returned true %d times", existsCount)
	})
}

// TestSyncMapUintStress performs stress testing under high load
func TestSyncMapUintStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	sm := syncops.NewSyncMapUint[TestData](1000)

	var wg sync.WaitGroup
	duration := 5 * time.Second

	// High-volume readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			operations := 0

			for time.Since(start) < duration {
				key := uint32(operations % 1000)
				switch operations % 4 {
				case 0:
					_ = sm.Check(key)
				case 1:
					_ = sm.GetVal(key)
				case 2:
					_ = sm.GetMap()
				case 3:
					_ = sm.Exists(func(k uint32, d TestData) bool {
						return k == key
					})
				}
				operations++
			}

			t.Logf("Reader %d completed %d operations", workerID, operations)
		}(i)
	}

	// High-volume writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			operations := 0

			for time.Since(start) < duration {
				key := uint32(workerID*1000 + operations%1000)
				data := TestData{
					ID:    key,
					Name:  fmt.Sprintf("stress_%d_%d", workerID, operations),
					Value: int64(operations),
				}

				switch operations % 3 {
				case 0:
					sm.Add(key, data)
				case 1:
					sm.UpdateVal(key, data)
				case 2:
					sm.Delete(key)
				}
				operations++
			}

			t.Logf("Writer %d completed %d operations", workerID, operations)
		}(i)
	}

	wg.Wait()
}

// BenchmarkSyncMapUintConcurrency benchmarks concurrent access to syncMapUint
func BenchmarkSyncMapUintConcurrency(b *testing.B) {
	sm := syncops.NewSyncMapUint[TestData](1000)

	// Pre-populate
	for i := uint32(0); i < 1000; i++ {
		sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("bench_%d", i), Value: int64(i)})
	}

	b.RunParallel(func(pb *testing.PB) {
		i := uint32(0)
		for pb.Next() {
			key := i % 1000
			switch i % 5 {
			case 0:
				_ = sm.Check(key)
			case 1:
				_ = sm.GetVal(key)
			case 2:
				data := TestData{ID: key, Name: "bench", Value: int64(i)}
				sm.UpdateVal(key, data)
			case 3:
				_ = sm.Exists(func(k uint32, d TestData) bool {
					return d.Value%10 == 0
				})
			case 4:
				_ = sm.GetMap()
			}
			i++
		}
	})
}
