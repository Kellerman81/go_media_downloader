package pool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPoolobjRaceConditions tests the thread safety of Poolobj operations
// Run with: go test -race -run TestPoolobjRaceConditions
func TestPoolobjRaceConditions(t *testing.T) {
	t.Run("ConcurrentGetPut", func(t *testing.T) {
		pool := NewPool(10, 5, 
			func(s *string) { *s = "initialized" },
			func(s *string) bool { *s = ""; return false },
		)
		
		var wg sync.WaitGroup
		numGoroutines := 50
		iterations := 100
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					// Get object from pool
					obj := pool.Get()
					if obj == nil {
						t.Errorf("Got nil object from pool")
						continue
					}
					
					// Use the object (simulate work)
					*obj = "worker_" + string(rune(id+'A'))
					
					// Return object to pool
					pool.Put(obj)
				}
			}(i)
		}
		
		wg.Wait()
	})
	
	t.Run("ConcurrentNewObjAndPut", func(t *testing.T) {
		pool := NewPool(5, 0, // Small pool, no initial objects
			func(s *string) { *s = "new_object" },
			nil,
		)
		
		var wg sync.WaitGroup
		var createdObjects int64
		var putObjects int64
		
		// Goroutines creating new objects
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					obj := pool.NewObj()
					atomic.AddInt64(&createdObjects, 1)
					
					// Try to put it back (may fail if pool is full)
					if pool.Put(obj) {
						atomic.AddInt64(&putObjects, 1)
					}
				}
			}()
		}
		
		// Goroutines getting and putting objects
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					obj := pool.Get()
					if obj != nil {
						// Simulate some work
						*obj = "processed"
						pool.Put(obj)
					}
				}
			}()
		}
		
		wg.Wait()
		
		t.Logf("Created %d objects, put back %d objects", createdObjects, putObjects)
	})
	
	t.Run("ConcurrentInitialization", func(t *testing.T) {
		var wg sync.WaitGroup
		numPools := 20
		
		// Create multiple pools concurrently
		for i := 0; i < numPools; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				pool := NewPool(5, 2,
					func(s *string) { *s = "pool_" + string(rune(id+'A')) },
					func(s *string) bool { return false },
				)
				
				// Immediately start using the pool
				for j := 0; j < 10; j++ {
					obj := pool.Get()
					if obj != nil {
						pool.Put(obj)
					}
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// TestSizedWaitGroupRaceConditions tests the thread safety of SizedWaitGroup
func TestSizedWaitGroupRaceConditions(t *testing.T) {
	t.Run("ConcurrentAddDone", func(t *testing.T) {
		swg := NewSizedGroup(10)
		var wg sync.WaitGroup
		numGoroutines := 50
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Add to sized wait group
				swg.Add()
				
				// Simulate work
				time.Sleep(time.Millisecond * time.Duration(id%10))
				
				// Mark as done
				swg.Done()
			}(i)
		}
		
		// Wait for all goroutines to complete
		wg.Wait()
		
		// Now wait for sized wait group to complete (all workers are finished)
		swg.Wait()
	})
	
	t.Run("ConcurrentMultipleSizedGroups", func(t *testing.T) {
		var wg sync.WaitGroup
		numGroups := 10
		
		for i := 0; i < numGroups; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				swg := NewSizedGroup(5)
				var localWg sync.WaitGroup
				
				// Start workers for this group
				for j := 0; j < 20; j++ {
					localWg.Add(1)
					go func(workerID int) {
						defer localWg.Done()
						swg.Add()
						defer swg.Done()
						
						// Simulate work
						time.Sleep(time.Microsecond * time.Duration(workerID%100))
					}(j)
				}
				
				// Wait for all workers to start
				localWg.Wait()
				
				// Wait for this group to complete
				swg.Wait()
			}(i)
		}
		
		wg.Wait()
	})
	
	t.Run("StressTestSizedWaitGroup", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping stress test in short mode")
		}
		
		swg := NewSizedGroup(20)
		var completed int64
		
		// Start many workers
		for i := 0; i < 1000; i++ {
			go func(id int) {
				swg.Add()
				defer swg.Done()
				
				// Simulate variable work
				time.Sleep(time.Microsecond * time.Duration(id%50))
				atomic.AddInt64(&completed, 1)
			}(i)
		}
		
		// Wait for all to complete
		swg.Wait()
		
		if completed != 1000 {
			t.Errorf("Expected 1000 completed workers, got %d", completed)
		}
	})
}

// TestPoolStressTest performs stress testing under high contention
func TestPoolStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	
	t.Run("HighContentionPool", func(t *testing.T) {
		pool := NewPool(5, 2, // Small pool, high contention
			func(s *string) { *s = "stress_test" },
			func(s *string) bool { *s = ""; return false },
		)
		
		var wg sync.WaitGroup
		var operations int64
		duration := 3 * time.Second
		
		// Start many workers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				start := time.Now()
				ops := int64(0)
				
				for time.Since(start) < duration {
					obj := pool.Get()
					if obj != nil {
						*obj = "worker_data"
						pool.Put(obj)
					}
					ops++
				}
				
				atomic.AddInt64(&operations, ops)
			}(i)
		}
		
		wg.Wait()
		t.Logf("Completed %d operations under high contention", operations)
	})
}

// BenchmarkPoolConcurrency benchmarks concurrent pool operations
func BenchmarkPoolConcurrency(b *testing.B) {
	pool := NewPool(10, 5,
		func(s *string) { *s = "benchmark" },
		func(s *string) bool { return false },
	)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			obj := pool.Get()
			if obj != nil {
				*obj = "test_data"
				pool.Put(obj)
			}
		}
	})
}

// BenchmarkSizedWaitGroupConcurrency benchmarks concurrent SizedWaitGroup operations
func BenchmarkSizedWaitGroupConcurrency(b *testing.B) {
	swg := NewSizedGroup(100)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			swg.Add()
			swg.Done()
		}
	})
}

// TestPoolDestructorRace tests race conditions in destructor calls
func TestPoolDestructorRace(t *testing.T) {
	var destructorCalls int64
	
	pool := NewPool(3, 0,
		func(s *string) { *s = "created" },
		func(s *string) bool {
			atomic.AddInt64(&destructorCalls, 1)
			*s = "destroyed"
			return false // Don't reject the object
		},
	)
	
	var wg sync.WaitGroup
	
	// Multiple goroutines putting objects
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				obj := pool.NewObj()
				*obj = "test_data"
				pool.Put(obj) // This should call destructor
			}
		}()
	}
	
	wg.Wait()
	
	// Destructor should have been called for successful Put operations
	if destructorCalls == 0 {
		t.Error("Destructor was never called")
	}
	
	t.Logf("Destructor called %d times", destructorCalls)
}