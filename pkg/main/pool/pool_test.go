package pool

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolObjGet(t *testing.T) {
	p := NewPool(5, 0, func(s *string) { *s = "initialized" }, nil)

	obj := p.Get()
	if *obj != "initialized" {
		t.Errorf("Expected initialized object, got %v", obj)
	}

	for range 3 {
		p.Put(obj)
		obj2 := p.Get()
		if obj != obj2 {
			t.Error("Expected same object reference from pool")
		}
	}
}

func TestPoolObjPut(t *testing.T) {
	destructorCalled := 0
	p := NewPool(3, 3, nil, func(i *int) bool {
		destructorCalled++
		return false
	})

	obj1 := p.Get()
	obj2 := p.Get()
	obj3 := p.Get()

	p.Put(obj1)
	p.Put(obj2)
	p.Put(obj3)

	if len(p.objs) != 3 {
		t.Errorf("Expected pool size 3, got %d", len(p.objs))
	}

	if destructorCalled != 6 {
		t.Errorf("Expected destructor called 6 times, got %d", destructorCalled)
	}
}

func TestPoolObjInit(t *testing.T) {
	var p Poolobj[int]
	constructorCalled := 0

	p.Init(200, 3, func(i *int) {
		constructorCalled++
		*i = constructorCalled
	}, nil)

	if constructorCalled != 3 {
		t.Errorf("Expected constructor called 3 times, got %d", constructorCalled)
	}

	if len(p.objs) != 3 {
		t.Errorf("Expected 3 objects in pool, got %d", len(p.objs))
	}
}

func TestSizedWaitGroupConcurrent(t *testing.T) {
	swg := NewSizedGroup(2)
	results := make([]int, 0, 4)
	var mu sync.Mutex

	// Like sync.WaitGroup, Add must happen before Wait in the spawning
	// goroutine; with a limit of 2 the third Add blocks until a slot frees.
	for i := range 4 {
		swg.Add()
		go func() {
			defer swg.Done()

			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			results = append(results, i)
			mu.Unlock()
		}()
	}

	swg.Wait()

	mu.Lock()
	resultLen := len(results)
	mu.Unlock()

	if resultLen != 4 {
		t.Errorf("Expected 4 results, got %d", resultLen)
	}
}

func TestSizedWaitGroupZeroLimit(t *testing.T) {
	swg := NewSizedGroup(0)
	if swg.Size != 1 {
		t.Errorf("Expected size 1 for zero limit, got %d", swg.Size)
	}
}

func TestPoolObjDestructorTrue(t *testing.T) {
	p := NewPool(5, 0, nil, func(s *string) bool {
		return false
	})

	t.Logf("obj 1: %v", len(p.objs))
	obj := p.Get()
	t.Logf("obj 2: %v", len(p.objs))
	*obj = "test"
	a := p.Put(obj)
	t.Logf("obj 3 ret: %v", a)
	t.Logf("obj 3: %v", len(p.objs))

	if len(p.objs) == 0 {
		t.Errorf("Expected non empty pool when destructor returns true, got size %d", len(p.objs))
	}
	obj2 := p.Get()
	_ = obj2
	if len(p.objs) != 0 {
		t.Errorf("Expected empty pool when destructor returns true, got size %d", len(p.objs))
	}
}

func TestPoolObjNilPut(t *testing.T) {
	p := NewPool[string](5, 0, nil, nil)
	p.Put(nil)

	if len(p.objs) != 0 {
		t.Error("Expected pool to ignore nil objects")
	}
}

// TestPoolObjPutFullSkipsDestructor checks that a full pool drops the object
// untouched instead of resetting it first.
func TestPoolObjPutFullSkipsDestructor(t *testing.T) {
	calls := 0
	p := NewPool(1, 1, nil, func(s *string) bool {
		calls++
		*s = ""
		return false
	})

	calls = 0
	obj := "keep"
	if p.Put(&obj) {
		t.Fatal("Put into a full pool should fail")
	}

	if calls != 0 || obj != "keep" {
		t.Fatalf("destructor ran on a rejected Put: calls=%d obj=%q", calls, obj)
	}

	if got := p.Stats().CurrentSize; got != 1 {
		t.Fatalf("CurrentSize = %d, want 1", got)
	}
}

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

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for range iterations {
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
		for range 10 {
			wg.Go(func() {
				for range 100 {
					obj := pool.NewObj()
					atomic.AddInt64(&createdObjects, 1)

					// Try to put it back (may fail if pool is full)
					if pool.Put(obj) {
						atomic.AddInt64(&putObjects, 1)
					}
				}
			})
		}

		// Goroutines getting and putting objects
		for range 10 {
			wg.Go(func() {
				for range 50 {
					obj := pool.Get()
					if obj != nil {
						// Simulate some work
						*obj = "processed"
						pool.Put(obj)
					}
				}
			})
		}

		wg.Wait()

		t.Logf("Created %d objects, put back %d objects", createdObjects, putObjects)
	})

	t.Run("ConcurrentInitialization", func(t *testing.T) {
		var wg sync.WaitGroup
		numPools := 20

		// Create multiple pools concurrently
		for i := range numPools {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				pool := NewPool(5, 2,
					func(s *string) { *s = "pool_" + string(rune(id+'A')) },
					func(s *string) bool { return false },
				)

				// Immediately start using the pool
				for range 10 {
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

		for i := range numGoroutines {
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

		for i := range numGroups {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				swg := NewSizedGroup(5)
				var localWg sync.WaitGroup

				// Start workers for this group
				for j := range 20 {
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
		// Add before spawning: an Add inside the goroutine can race Wait, which
		// then returns before any worker has started.
		for i := range 1000 {
			swg.Add()
			go func(id int) {
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
		for i := range 50 {
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
	for range 20 {
		wg.Go(func() {
			for range 10 {
				obj := pool.NewObj()
				*obj = "test_data"
				pool.Put(obj) // This should call destructor
			}
		})
	}

	wg.Wait()

	// Destructor should have been called for successful Put operations
	if destructorCalls == 0 {
		t.Error("Destructor was never called")
	}

	t.Logf("Destructor called %d times", destructorCalls)
}

func TestPutWithError(t *testing.T) {
	p := NewPool(1, 0, nil, func(i *int) bool { return *i < 0 })

	if err := p.PutWithError(nil); !errors.Is(err, ErrNilObject) {
		t.Errorf("nil object: %v", err)
	}

	bad := -1
	if err := p.PutWithError(&bad); !errors.Is(err, ErrInvalidOperation) {
		t.Errorf("destructor reject: %v", err)
	}

	a, b := 1, 2
	if err := p.PutWithError(&a); err != nil {
		t.Errorf("first put: %v", err)
	}

	if err := p.PutWithError(&b); !errors.Is(err, ErrPoolFull) {
		t.Errorf("full pool: %v", err)
	}

	s := p.Stats()
	if s.Puts != 4 || s.Rejects != 1 || s.Fails != 1 {
		t.Errorf("stats = %+v", s)
	}
}

func TestPoolStatsSnapshotDerived(t *testing.T) {
	var zero PoolStatsSnapshot
	if zero.HitRate() != 0 || zero.AverageGetTime() != 0 || zero.AveragePutTime() != 0 || zero.AverageCreateTime() != 0 {
		t.Error("zero snapshot has non-zero derived values")
	}

	s := PoolStatsSnapshot{
		Gets: 4, Hits: 3, Puts: 2, Creates: 1,
		TotalGetTime: int64(8 * time.Millisecond), TotalPutTime: int64(4 * time.Millisecond), TotalCreateTime: int64(time.Second),
	}

	if s.HitRate() != 0.75 || s.AverageGetTime() != 2*time.Millisecond || s.AveragePutTime() != 2*time.Millisecond ||
		s.AverageCreateTime() != time.Second {
		t.Errorf("derived = %v %v %v %v", s.HitRate(), s.AverageGetTime(), s.AveragePutTime(), s.AverageCreateTime())
	}

	if str := s.String(); !strings.Contains(str, "Hits=3 (75.0%)") || !strings.Contains(str, "AvgCreateTime=1s") {
		t.Errorf("String = %q", str)
	}
}

func TestPoolHealthAndUtilization(t *testing.T) {
	p := NewPool[int](4, 0, nil, nil)

	if !p.IsHealthy() || p.UtilizationPercent() != 0 {
		t.Error("fresh pool is not healthy/empty")
	}

	objs := make([]*int, 0, 12)
	for range 12 {
		objs = append(objs, p.Get()) // all misses
	}

	if p.IsHealthy() {
		t.Error("pool with 0% hit rate after 12 gets reported healthy")
	}

	for _, o := range objs[:2] {
		p.Put(o)
	}

	if got := p.UtilizationPercent(); got != 0.5 {
		t.Errorf("utilization = %v, want 0.5", got)
	}

	if !strings.HasPrefix(p.stats.String(), "Pool Stats:") {
		t.Error("PoolStats.String has the wrong format")
	}
}

func TestSizedWaitGroupTryAddAndStats(t *testing.T) {
	g := NewSizedGroup(2)

	if !g.TryAdd() || !g.TryAdd() {
		t.Fatal("TryAdd failed with free slots")
	}

	if g.TryAdd() {
		t.Fatal("TryAdd succeeded on a full group")
	}

	s := g.Stats()
	if s.Adds != 3 || s.Currently != 2 || s.UtilizationPercent() != 1 {
		t.Errorf("stats = %+v, util %v", s, s.UtilizationPercent())
	}

	cp := s.Copy()
	if cp != s {
		t.Errorf("Copy = %+v, want %+v", cp, s)
	}

	g.Done()
	g.Done()
	g.Wait()

	g.Close()

	if !g.TryAdd() {
		t.Error("TryAdd failed after Close reset the group")
	}

	if (&SizedWaitGroupStats{}).UtilizationPercent() != 0 {
		t.Error("zero-size utilization not 0")
	}
}
