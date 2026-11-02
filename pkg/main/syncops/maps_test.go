package syncops

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestModifyInPlaceConcurrentRead mutates an inner map via ModifyInPlace while
// readers index into it via ReadInPlace. Run with -race; before ModifyInPlace
// took the write lock this was a fatal concurrent map read and map write.
func TestModifyInPlaceConcurrentRead(t *testing.T) {
	sm := NewSyncMap[map[string]int](1)
	sm.Add("k", map[string]int{}, 0, false, 0)

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			for i := range 2000 {
				sm.ModifyInPlace("k", func(m map[string]int) {
					m[strconv.Itoa(w*10000+i)] = i
				})
			}
		}()

		go func() {
			defer wg.Done()

			for i := range 2000 {
				sm.ReadInPlace("k", func(m map[string]int) {
					_ = m[strconv.Itoa(i)]
				})
			}
		}()
	}

	wg.Wait()

	var n int

	if !sm.ReadInPlace("k", func(m map[string]int) { n = len(m) }) || n != 8000 {
		t.Fatalf("expected 8000 inner entries, got %d", n)
	}

	if sm.ReadInPlace("missing", func(map[string]int) { t.Fatal("fn called for missing key") }) {
		t.Fatal("ReadInPlace reported a missing key as present")
	}
}

func TestUpdateValMissingKeyIsNoop(t *testing.T) {
	sm := NewSyncMap[[]string](1)
	if sm.UpdateVal("gone", []string{"x"}) || sm.Check("gone") {
		t.Fatal("SyncMap.UpdateVal created a missing key")
	}

	su := NewSyncMapUint[Job](1)
	if su.UpdateVal(7, Job{ID: 7}) || su.Check(7) {
		t.Fatal("SyncMapUint.UpdateVal created a missing key")
	}

	// Through the queue: an update after a delete must not resurrect the job.
	freshManager(t)
	RegisterSyncMap(MapTypeQueue, su)
	QueueWorkerMapAdd(MapTypeQueue, 9, Job{ID: 9})
	QueueWorkerMapDelete(MapTypeQueue, 9)
	QueueWorkerMapUpdate(MapTypeQueue, 9, Job{ID: 9, Name: "late"})

	if su.Check(9) {
		t.Fatal("QueueWorkerMapUpdate resurrected a deleted job")
	}
}

func TestUpdateExpireOnKeyWithoutExpiry(t *testing.T) {
	sm := NewSyncMap[[]string](1)
	AtomicAppendToStringSlice(sm, "k", "v") // created with expires == 0

	past := time.Now().Add(-time.Minute).UnixNano()
	sm.UpdateExpire("k", past)

	if !sm.CheckExpires("k", false, 0) {
		t.Fatal("UpdateExpire did not set an expiry on a key that had none")
	}
}

func TestCheckExpiresResult(t *testing.T) {
	sm := NewSyncMap[[]string](2)
	if !sm.CheckExpires("missing", false, 0) {
		t.Fatal("CheckExpires should report a missing key as expired")
	}

	sm.Add("old", []string{"a"}, time.Now().Add(-time.Minute).UnixNano(), false, 0)
	sm.Add("new", []string{"b"}, time.Now().Add(time.Hour).UnixNano(), false, 0)

	freshManager(t)
	RegisterSyncMap(MapTypeString, sm)

	if !QueueSyncMapCheckExpires(MapTypeString, "old", false, 1) {
		t.Fatal("QueueSyncMapCheckExpires did not report the expired key")
	}

	if QueueSyncMapCheckExpires(MapTypeString, "new", false, 1) {
		t.Fatal("QueueSyncMapCheckExpires reported a live key as expired")
	}

	// extend=true: reported expired once, then extended.
	if !QueueSyncMapCheckExpires(MapTypeString, "old", true, 1) ||
		QueueSyncMapCheckExpires(MapTypeString, "old", true, 1) {
		t.Fatal("QueueSyncMapCheckExpires extend did not extend the expiry")
	}
}

func TestDeleteIfMatch(t *testing.T) {
	sm := NewSyncMap[uint32](1)
	sm.Add("name", 2, 0, false, 0)

	if sm.DeleteIfMatch("name", func(v uint32) bool { return v == 1 }) || !sm.Check("name") {
		t.Fatal("DeleteIfMatch removed an entry owned by someone else")
	}

	if !sm.DeleteIfMatch("name", func(v uint32) bool { return v == 2 }) || sm.Check("name") {
		t.Fatal("DeleteIfMatch did not remove the matching entry")
	}
}

func TestQueueDeleteFuncs(t *testing.T) {
	freshManager(t)

	sm := NewSyncMap[[]string](4)
	RegisterSyncMap(MapTypeString, sm)

	past := time.Now().Add(-time.Hour).UnixNano()
	future := time.Now().Add(time.Hour).UnixNano()

	sm.Add("empty", nil, future, false, 0)
	sm.Add("full", []string{"x"}, past, false, 0)
	sm.Add("keep", []string{"y"}, future, false, 0)

	if !QueueSyncMapDeleteFunc(MapTypeString, func(v []string) bool { return len(v) == 0 }) {
		t.Fatal("DeleteFunc reported failure")
	}

	if sm.Check("empty") || !sm.Check("full") {
		t.Fatal("DeleteFunc removed the wrong entries")
	}

	now := time.Now().UnixNano()
	if !QueueSyncMapDeleteFuncExpires(MapTypeString, func(exp int64) bool { return exp < now }) {
		t.Fatal("DeleteFuncExpires reported failure")
	}

	if sm.Check("full") || !sm.Check("keep") {
		t.Fatal("DeleteFuncExpires removed the wrong entries")
	}

	// A filter of the wrong type is rejected instead of panicking.
	if QueueSyncMapDeleteFunc(MapTypeString, func(string) bool { return true }) ||
		QueueSyncMapDeleteFuncExpires(MapTypeString, func(int) bool { return true }) {
		t.Error("wrongly typed filter reported success")
	}

	if !sm.Check("keep") {
		t.Error("wrongly typed filter deleted entries")
	}
}

func TestSyncMapUintQueries(t *testing.T) {
	sm := NewSyncMapUint[string](4)
	sm.Add(1, "one")
	sm.Add(2, "two")
	sm.Add(3, "three")

	k, v, ok := sm.FindFirst(func(_ uint32, v string) bool { return v == "two" })
	if !ok || k != 2 || v != "two" {
		t.Errorf("FindFirst = %d %q %t", k, v, ok)
	}

	if _, _, ok := sm.FindFirst(func(uint32, string) bool { return false }); ok {
		t.Error("FindFirst matched nothing but reported ok")
	}

	if !sm.Exists(func(k uint32, _ string) bool { return k == 3 }) || sm.Exists(func(k uint32, _ string) bool { return k == 9 }) {
		t.Error("Exists gave a wrong answer")
	}

	var sum uint32

	sm.ForEach(func(k uint32, _ string) { sum += k })

	if sum != 6 {
		t.Errorf("ForEach visited keys summing to %d, want 6", sum)
	}

	cp := sm.GetMap()
	delete(cp, 1)

	if !sm.Check(1) {
		t.Error("GetMap returned the live map instead of a copy")
	}

	sm.DeleteIf(func(k uint32, _ string) bool { return k%2 == 1 })

	if sm.Check(1) || sm.Check(3) || !sm.Check(2) {
		t.Errorf("DeleteIf left %v", sm.GetMap())
	}
}

func TestSyncMapDeleteKeyFuncAndLastScan(t *testing.T) {
	sm := NewSyncMap[int](4)
	sm.Add("movie_1", 1, 0, false, 42)
	sm.Add("serie_1", 2, 0, false, 0)

	if got := sm.GetLastScan("movie_1"); got != 42 {
		t.Errorf("GetLastScan = %d", got)
	}

	if got := sm.GetLastScan("missing"); got != 0 {
		t.Errorf("GetLastScan(missing) = %d", got)
	}

	sm.DeleteKeyFunc(func(k string) bool { return k[:5] == "movie" })

	if sm.Check("movie_1") || !sm.Check("serie_1") {
		t.Error("DeleteKeyFunc removed the wrong keys")
	}
}

func TestSyncMapOperations(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMaps
	stringMap := NewSyncMap[[]string](10)

	// Register the maps
	RegisterSyncMap(MapTypeString, stringMap)

	// Test SyncMap Add operation
	testStrings := []string{"test1", "test2", "test3"}
	QueueSyncMapAdd(
		MapTypeString,
		"testkey",
		testStrings,
		time.Now().Unix()+3600,
		false,
		time.Now().Unix(),
	)

	// Test UpdateVal operation
	newStrings := []string{"updated1", "updated2"}
	QueueSyncMapUpdateVal(MapTypeString, "testkey", newStrings)

	// Test UpdateExpire operation
	newExpire := time.Now().Unix() + 7200
	QueueSyncMapUpdateExpire(MapTypeString, "testkey", newExpire)

	// Test UpdateLastscan operation
	newLastscan := time.Now().Unix()
	QueueSyncMapUpdateLastscan(MapTypeString, "testkey", newLastscan)

	// Test Delete operation
	QueueSyncMapDelete(MapTypeString, "testkey")
}

// TestAddIfAbsent verifies the atomic check-then-add semantics importfeed relies on.
func TestAddIfAbsent(t *testing.T) {
	sm := NewSyncMap[struct{}](10)

	if !sm.AddIfAbsent("job", struct{}{}, 0) {
		t.Fatal("first AddIfAbsent should win")
	}
	if sm.AddIfAbsent("job", struct{}{}, 0) {
		t.Fatal("second AddIfAbsent should lose while entry is live")
	}

	// An expired entry can be taken over.
	sm.Delete("job")
	expired := time.Now().Add(-time.Hour).UnixNano()
	if !sm.AddIfAbsent("job2", struct{}{}, expired) {
		t.Fatal("AddIfAbsent into empty map should win")
	}
	if !sm.AddIfAbsent("job2", struct{}{}, 0) {
		t.Fatal("AddIfAbsent should take over an expired entry")
	}
}

// TestSyncOpsDebug tests basic syncops functionality
func TestSyncOpsDebug(t *testing.T) {
	initOnce = sync.Once{}
	InitSyncOps()
	defer Shutdown()

	// Test 1: Test SyncMap directly
	t.Run("DirectSyncMapTest", func(t *testing.T) {
		stringMap := NewSyncMap[[]string](10)

		// Test direct Add (bypassing queue system)
		testData := []string{"direct1", "direct2"}
		stringMap.Add("direct_key", testData, time.Now().Unix()+3600, false, time.Now().Unix())

		// Verify it exists
		if !stringMap.Check("direct_key") {
			t.Error("Direct Add failed - key should exist")
		}

		// Verify content
		retrieved := stringMap.GetVal("direct_key")
		if len(retrieved) != 2 || retrieved[0] != "direct1" || retrieved[1] != "direct2" {
			t.Errorf("Direct Add content mismatch: expected [direct1, direct2], got %v", retrieved)
		}

		// Test direct Delete
		stringMap.Delete("direct_key")
		if stringMap.Check("direct_key") {
			t.Error("Direct Delete failed - key should not exist")
		}
	})

	// Test 2: Test SyncMapUint directly
	t.Run("DirectSyncMapUintTest", func(t *testing.T) {
		scheduleMap := NewSyncMapUint[JobSchedule](10)

		// Test direct Add
		testSchedule := JobSchedule{
			ID:      100,
			JobName: "direct_test",
		}
		scheduleMap.Add(100, testSchedule)

		// Verify it exists
		if !scheduleMap.Check(100) {
			t.Error("Direct SyncMapUint Add failed - key should exist")
		}

		// Verify content
		retrieved := scheduleMap.GetVal(100)
		if retrieved.ID != 100 || retrieved.JobName != "direct_test" {
			t.Errorf(
				"Direct SyncMapUint content mismatch: expected {100, direct_test}, got {%d, %s}",
				retrieved.ID,
				retrieved.JobName,
			)
		}

		// Test direct Delete
		scheduleMap.Delete(100)
		if scheduleMap.Check(100) {
			t.Error("Direct SyncMapUint Delete failed - key should not exist")
		}
	})

	// Test 3: Test queued operations with registration
	t.Run("QueuedOperationsTest", func(t *testing.T) {
		stringMap := NewSyncMap[[]string](10)
		RegisterSyncMap(MapTypeString, stringMap)

		testData := []string{"queued1", "queued2"}

		// Queue Add operation
		QueueSyncMapAdd(
			MapTypeString,
			"queued_key",
			testData,
			time.Now().Unix()+3600,
			false,
			time.Now().Unix(),
		)

		// Since operations are synchronous, this should be immediate
		if !stringMap.Check("queued_key") {
			t.Error("Queued Add failed - key should exist immediately after synchronous operation")
		}

		// Verify content
		retrieved := stringMap.GetVal("queued_key")
		if len(retrieved) != 2 || retrieved[0] != "queued1" || retrieved[1] != "queued2" {
			t.Errorf("Queued Add content mismatch: expected [queued1, queued2], got %v", retrieved)
		}

		// Queue Delete operation
		QueueSyncMapDelete(MapTypeString, "queued_key")

		// Verify it's deleted
		if stringMap.Check("queued_key") {
			t.Error("Queued Delete failed - key should not exist after synchronous operation")
		}
	})

	// Test 4: Test worker map operations
	t.Run("QueuedWorkerMapTest", func(t *testing.T) {
		scheduleMap := NewSyncMapUint[JobSchedule](10)
		RegisterSyncMap(MapTypeSchedule, scheduleMap)

		testSchedule := JobSchedule{
			ID:      200,
			JobName: "queued_test",
		}

		// Queue Add operation
		QueueWorkerMapAdd(MapTypeSchedule, 200, testSchedule)

		// Since operations are synchronous, this should be immediate
		if !scheduleMap.Check(200) {
			t.Error(
				"Queued Worker Add failed - key should exist immediately after synchronous operation",
			)
		}

		// Verify content
		retrieved := scheduleMap.GetVal(200)
		if retrieved.ID != 200 || retrieved.JobName != "queued_test" {
			t.Errorf(
				"Queued Worker Add content mismatch: expected {200, queued_test}, got {%d, %s}",
				retrieved.ID,
				retrieved.JobName,
			)
		}

		// Queue Delete operation
		QueueWorkerMapDelete(MapTypeSchedule, 200)

		// Verify it's deleted
		if scheduleMap.Check(200) {
			t.Error(
				"Queued Worker Delete failed - key should not exist after synchronous operation",
			)
		}
	})
}

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
		sm := NewSyncMapUint[TestData](100)

		var wg sync.WaitGroup
		numGoroutines := 50
		iterations := 100

		// Writers
		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range iterations {
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
				for j := range iterations {
					key := uint32((id + j) % (numGoroutines * iterations))
					sm.Delete(key)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("ConcurrentUpdateVal", func(t *testing.T) {
		sm := NewSyncMapUint[TestData](50)

		// Pre-populate with some data
		for i := range uint32(50) {
			sm.Add(i, TestData{ID: i, Name: "initial", Value: 0})
		}

		var wg sync.WaitGroup
		var updates int64

		// Multiple updaters
		for i := range 20 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 100 {
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
		for range 10 {
			wg.Go(func() {
				for j := range 200 {
					key := uint32(j % 50)
					data := sm.GetVal(key)
					_ = data.Name // Use the data
				}
			})
		}

		wg.Wait()
		t.Logf("Completed %d updates", updates)
	})

	t.Run("ConcurrentGetMap", func(t *testing.T) {
		sm := NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := range uint32(100) {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i)})
		}

		var wg sync.WaitGroup

		// Multiple GetMap calls
		for i := range 20 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for range 50 {
					mapCopy := sm.GetMap()
					// Map can grow beyond 100 due to concurrent additions
					if len(mapCopy) > 200 {
						t.Errorf("Map copy size unexpected: %d", len(mapCopy))
					}
				}
			}(i)
		}

		// Concurrent modifications
		for i := range 10 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 25 {
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
		sm := NewSyncMapUint[TestData](50)

		// Pre-populate
		for i := range uint32(50) {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i)})
		}

		var wg sync.WaitGroup
		var forEachCalls int64

		// Multiple ForEach operations
		for range 10 {
			wg.Go(func() {
				for range 20 {
					sm.ForEach(func(key uint32, data TestData) {
						atomic.AddInt64(&forEachCalls, 1)
						_ = data.Name // Use the data
					})
				}
			})
		}

		// Concurrent modifications
		for i := range 5 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
					key := uint32(50 + id*10 + j)
					sm.Add(key, TestData{ID: key, Name: "concurrent", Value: int64(key)})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("ForEach called %d times", forEachCalls)
	})

	t.Run("ConcurrentDeleteIf", func(t *testing.T) {
		sm := NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := range uint32(100) {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i % 10)})
		}

		var wg sync.WaitGroup

		// Multiple DeleteIf operations
		for i := range 5 {
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
		for range 10 {
			wg.Go(func() {
				for j := range 50 {
					key := uint32(j % 100)
					_ = sm.Check(key)
				}
			})
		}

		wg.Wait()

		// Check final state
		finalMap := sm.GetMap()
		t.Logf("Final map size: %d", len(finalMap))
	})

	t.Run("ConcurrentFindFirst", func(t *testing.T) {
		sm := NewSyncMapUint[TestData](100)

		// Pre-populate
		for i := range uint32(100) {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("item_%d", i), Value: int64(i % 20)})
		}

		var wg sync.WaitGroup
		var foundCount int64

		// Multiple FindFirst operations
		for i := range 20 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 50 {
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
		for i := range 5 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 20 {
					key := uint32(100 + id*20 + j)
					sm.Add(key, TestData{ID: key, Name: "new", Value: int64(j % 5)})
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Found items %d times", foundCount)
	})

	t.Run("ConcurrentExists", func(t *testing.T) {
		sm := NewSyncMapUint[TestData](50)

		// Pre-populate
		for i := range uint32(50) {
			sm.Add(i, TestData{ID: i, Name: fmt.Sprintf("special_%d", i%5), Value: int64(i)})
		}

		var wg sync.WaitGroup
		var existsCount int64

		// Multiple Exists operations
		for i := range 15 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 100 {
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
		for i := range 5 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := range 10 {
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

	sm := NewSyncMapUint[TestData](1000)

	var wg sync.WaitGroup
	duration := 5 * time.Second

	// High-volume readers
	for i := range 20 {
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
	for i := range 10 {
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
	sm := NewSyncMapUint[TestData](1000)

	// Pre-populate
	for i := range uint32(1000) {
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
