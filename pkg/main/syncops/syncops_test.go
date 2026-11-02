package syncops

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSyncOpsManagerInit(t *testing.T) {
	// Reset global manager for test
	manager.Store(nil)
	initOnce = sync.Once{}

	InitSyncOps()

	if manager.Load() == nil {
		t.Fatal("Manager should be initialized")
	}

	if !manager.Load().writerActive.Load() {
		t.Fatal("Writer should be active")
	}

	// Cleanup
	Shutdown()
}

func TestWorkerMapOperations(t *testing.T) {
	// Initialize syncops manager
	initOnce = sync.Once{}
	InitSyncOps()
	defer Shutdown()

	// Create real worker maps
	scheduleMap := NewSyncMapUint[JobSchedule](10)
	queueMap := NewSyncMapUint[Job](10)

	// Register the maps
	RegisterSyncMap(MapTypeSchedule, scheduleMap)
	RegisterSyncMap(MapTypeQueue, queueMap)

	// Test WorkerMap Add operations
	testSchedule := JobSchedule{
		JobName:        "test_job",
		ScheduleTyp:    "cron",
		ScheduleString: "0 0 * * *",
		LastRun:        time.Now(),
		NextRun:        time.Now().Add(time.Hour * 24),
		ID:             123,
		JobID:          456,
		IsRunning:      false,
	}
	QueueWorkerMapAdd(MapTypeSchedule, 123, testSchedule)

	testJob := Job{
		Queue:       "test_queue",
		JobName:     "test_job",
		Name:        "Test Job",
		Added:       time.Now(),
		ID:          789,
		SchedulerID: 123,
		Ctx:         context.Background(),
	}
	QueueWorkerMapAdd(MapTypeQueue, 789, testJob)

	// Test WorkerMap Update operations
	updatedSchedule := testSchedule
	updatedSchedule.IsRunning = true
	QueueWorkerMapUpdate(MapTypeSchedule, 123, updatedSchedule)

	updatedJob := testJob
	updatedJob.Started = time.Now()
	QueueWorkerMapUpdate(MapTypeQueue, 789, updatedJob)

	// Operations are synchronous, so updates are visible immediately.
	if !scheduleMap.GetVal(123).IsRunning {
		t.Error("Schedule update not applied")
	}
	if queueMap.GetVal(789).Started.IsZero() {
		t.Error("Job update not applied")
	}

	// Test WorkerMap Delete operations
	QueueWorkerMapDelete(MapTypeSchedule, 123)
	QueueWorkerMapDelete(MapTypeQueue, 789)

	if scheduleMap.Check(123) || queueMap.Check(789) {
		t.Error("Worker map entries not deleted")
	}
}

// TestRegisterSyncMapOverwriteWarns verifies a second registration of the same
// MapType replaces the first (the warning itself is a log side-effect).
func TestRegisterSyncMapOverwriteWarns(t *testing.T) {
	initOnce = sync.Once{}
	manager.Store(nil)
	InitSyncOps()
	defer Shutdown()

	first := NewSyncMap[[]string](10)
	second := NewSyncMap[[]string](10)

	RegisterSyncMap(MapTypeString, first)
	RegisterSyncMap(MapTypeString, second)

	// Operations must now reach the second map, not the first.
	QueueSyncMapAdd(MapTypeString, "k", []string{"v"}, 0, false, 0)

	if first.Check("k") {
		t.Error("operation reached the overwritten (first) map")
	}
	if !second.Check("k") {
		t.Error("operation did not reach the current (second) map")
	}
}

// TestQueueOperationReturnsFailure verifies QueueOperation reports false when an
// operation cannot be applied (unregistered MapType or mismatched value type),
// instead of silently succeeding.
func TestQueueOperationReturnsFailure(t *testing.T) {
	initOnce = sync.Once{}
	manager.Store(nil)
	InitSyncOps()
	defer Shutdown()

	// Unregistered MapType.
	if QueueOperation(SyncOperation{OpType: OpSyncMapAdd, MapType: MapTypeTwoInt, Key: "x", Value: []DbstaticOneStringTwoInt{}}) {
		t.Error("expected failure for unregistered MapType")
	}

	// Registered, but value type does not match the map's element type.
	sm := NewSyncMap[[]string](10)
	RegisterSyncMap(MapTypeString, sm)

	if QueueOperation(SyncOperation{OpType: OpSyncMapAdd, MapType: MapTypeString, Key: "x", Value: 42}) {
		t.Error("expected failure for mismatched value type")
	}
	if sm.Check("x") {
		t.Error("mismatched-type operation should not have stored anything")
	}

	// A well-formed operation still succeeds.
	if !QueueOperation(SyncOperation{OpType: OpSyncMapAdd, MapType: MapTypeString, Key: "y", Value: []string{"ok"}}) {
		t.Error("expected success for well-formed operation")
	}
}

// TestShutdownUnderLoad spams operations from many goroutines while Shutdown is
// called concurrently. It must not panic (no send on closed channel) and every
// caller must return rather than block forever.
func TestShutdownUnderLoad(t *testing.T) {
	initOnce = sync.Once{}
	manager.Store(nil)
	InitSyncOps()

	sm := NewSyncMap[[]string](100)
	RegisterSyncMap(MapTypeString, sm)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range 100 {
				key := fmt.Sprintf("k_%d_%d", id, j)
				QueueSyncMapAdd(MapTypeString, key, []string{"v"}, 0, false, 0)
			}
		}(i)
	}

	// Shut down while the goroutines are still submitting.
	time.Sleep(2 * time.Millisecond)
	Shutdown()

	// All submitters must finish; the test's overall timeout catches a hang.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("QueueOperation callers blocked after Shutdown")
	}
}

func TestConcurrentOperations(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMap
	stringMap := NewSyncMap[[]string](100)
	RegisterSyncMap(MapTypeString, stringMap)

	// Number of concurrent goroutines
	numGoroutines := 10
	numOperationsPerGoroutine := 100

	var wg sync.WaitGroup

	// Launch concurrent goroutines performing various operations
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := range numOperationsPerGoroutine {
				key := "key_" + string(rune('A'+goroutineID)) + "_" + string(rune('0'+j%10))
				value := []string{
					"value1_" + string(rune('A'+goroutineID)),
					"value2_" + string(rune('0'+j%10)),
				}
				expires := time.Now().Unix() + 3600
				lastScan := time.Now().Unix()

				// Perform different operations based on iteration
				switch j % 4 {
				case 0:
					QueueSyncMapAdd(MapTypeString, key, value, expires, false, lastScan)
				case 1:
					QueueSyncMapUpdateVal(MapTypeString, key, value)
				case 2:
					QueueSyncMapUpdateExpire(MapTypeString, key, expires+1800)
				case 3:
					QueueSyncMapDelete(MapTypeString, key)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestQueueFullScenario(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMap
	stringMap := NewSyncMap[[]string](100)
	RegisterSyncMap(MapTypeString, stringMap)

	// Fill up the queue to test queue full scenario
	// The queue size is 10000, so we'll try to add more operations
	numOperations := 15000

	for i := range numOperations {
		key := "bulk_key_" + string(rune('0'+i%10))
		value := []string{"bulk_value_" + string(rune('0'+i%10))}
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		QueueSyncMapAdd(MapTypeString, key, value, expires, false, lastScan)
	}

	// All operations are now synchronous, no dropping occurs
}

func TestRegisterSyncMapTypes(t *testing.T) {
	// Reset global manager for test
	manager.Store(nil)
	initOnce = sync.Once{}

	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Test registering all supported SyncMap types
	stringMap := NewSyncMap[[]string](10)
	twoStringMap := NewSyncMap[[]DbstaticTwoStringOneInt](10)
	threeStringMap := NewSyncMap[[]DbstaticThreeStringTwoInt](10)
	twoIntMap := NewSyncMap[[]DbstaticOneStringTwoInt](10)

	RegisterSyncMap(MapTypeString, stringMap)
	RegisterSyncMap(MapTypeTwoString, twoStringMap)
	RegisterSyncMap(MapTypeThreeString, threeStringMap)
	RegisterSyncMap(MapTypeTwoInt, twoIntMap)

	// Test that all maps are registered
	if len(manager.Load().syncMaps) != 4 {
		t.Fatalf("Expected 4 registered maps, got %d", len(manager.Load().syncMaps))
	}

	// Test operations on each type
	QueueSyncMapAdd(
		MapTypeString,
		"str_key",
		[]string{"test"},
		time.Now().Unix()+3600,
		false,
		time.Now().Unix(),
	)
	QueueSyncMapAdd(
		MapTypeTwoString,
		"two_str_key",
		[]DbstaticTwoStringOneInt{{Str1: "test1", Str2: "test2", Num: 1}},
		time.Now().Unix()+3600,
		false,
		time.Now().Unix(),
	)
	QueueSyncMapAdd(
		MapTypeThreeString,
		"three_str_key",
		[]DbstaticThreeStringTwoInt{
			{Str1: "test1", Str2: "test2", Str3: "test3", Num1: 1, Num2: 2},
		},
		time.Now().Unix()+3600,
		false,
		time.Now().Unix(),
	)
	QueueSyncMapAdd(
		MapTypeTwoInt,
		"two_int_key",
		[]DbstaticOneStringTwoInt{{Str: "test", Num1: 1, Num2: 2}},
		time.Now().Unix()+3600,
		false,
		time.Now().Unix(),
	)
}

func TestShutdownAndReinitialization(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()

	if manager.Load() == nil {
		t.Fatal("Manager should be initialized")
	}

	// Shutdown the manager
	Shutdown()

	// Manager should still exist but not be active
	if manager.Load().writerActive.Load() {
		t.Fatal("Writer should not be active after shutdown")
	}

	// Reinitialize - should not create a new instance due to sync.Once
	InitSyncOps()

	// Should still use the same manager instance but it should be inactive
	if manager.Load().writerActive.Load() {
		t.Fatal("Writer should not be active after sync.Once prevents reinitialization")
	}

	// Reset for proper reinitialization
	manager.Store(nil)
	initOnce = sync.Once{}

	InitSyncOps()
	defer Shutdown()

	if !manager.Load().writerActive.Load() {
		t.Fatal("Writer should be active after proper reinitialization")
	}
}

// TestSyncMapAddDeleteVerification tests that Add and Delete operations actually work
func TestSyncMapAddDeleteVerification(t *testing.T) {
	// Reset initOnce so InitSyncOps creates a fresh manager
	// (previous tests leave writerActive=false after Shutdown)
	initOnce = sync.Once{}

	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMaps
	stringMap := NewSyncMap[[]string](100)
	twoStringMap := NewSyncMap[[]DbstaticTwoStringOneInt](100)

	// Register the maps
	RegisterSyncMap(MapTypeString, stringMap)
	RegisterSyncMap(MapTypeTwoString, twoStringMap)

	t.Run("StringMapAddAndVerify", func(t *testing.T) {
		testKey := "verify_string_key"
		testData := []string{"item1", "item2", "item3"}
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		// Verify key doesn't exist initially
		if stringMap.Check(testKey) {
			t.Errorf("Key %s should not exist initially", testKey)
		}

		// Add the data
		QueueSyncMapAdd(MapTypeString, testKey, testData, expires, false, lastScan)

		// Verify the key now exists
		if !stringMap.Check(testKey) {
			t.Errorf("Key %s should exist after Add operation", testKey)
		}

		// Verify the data content
		retrievedData := stringMap.GetVal(testKey)
		if len(retrievedData) != len(testData) {
			t.Errorf("Expected %d items, got %d", len(testData), len(retrievedData))
		}
		for i, item := range testData {
			if i >= len(retrievedData) || retrievedData[i] != item {
				t.Errorf("Item %d: expected %s, got %s", i, item, retrievedData[i])
			}
		}
	})

	t.Run("StringMapDeleteAndVerify", func(t *testing.T) {
		testKey := "verify_delete_key"
		testData := []string{"delete_item1", "delete_item2"}
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		// Add the data first
		QueueSyncMapAdd(MapTypeString, testKey, testData, expires, false, lastScan)

		// Verify it exists
		if !stringMap.Check(testKey) {
			t.Errorf("Key %s should exist after Add operation", testKey)
		}

		// Delete the data
		QueueSyncMapDelete(MapTypeString, testKey)

		// Verify it no longer exists
		if stringMap.Check(testKey) {
			t.Errorf("Key %s should not exist after Delete operation", testKey)
		}

		// Verify GetVal returns empty/zero value
		retrievedData := stringMap.GetVal(testKey)
		if len(retrievedData) != 0 {
			t.Errorf("Expected empty slice after delete, got %d items", len(retrievedData))
		}
	})

	t.Run("TwoStringMapAddAndVerify", func(t *testing.T) {
		testKey := "verify_twostring_key"
		testData := []DbstaticTwoStringOneInt{
			{Str1: "first1", Str2: "second1", Num: 10},
			{Str1: "first2", Str2: "second2", Num: 20},
		}
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		// Verify key doesn't exist initially
		if twoStringMap.Check(testKey) {
			t.Errorf("Key %s should not exist initially", testKey)
		}

		// Add the data
		QueueSyncMapAdd(MapTypeTwoString, testKey, testData, expires, false, lastScan)

		// Verify the key now exists
		if !twoStringMap.Check(testKey) {
			t.Errorf("Key %s should exist after Add operation", testKey)
		}

		// Verify the data content
		retrievedData := twoStringMap.GetVal(testKey)
		if len(retrievedData) != len(testData) {
			t.Errorf("Expected %d items, got %d", len(testData), len(retrievedData))
		}
		for i, item := range testData {
			if i >= len(retrievedData) {
				t.Errorf("Missing item %d", i)
				continue
			}
			retrieved := retrievedData[i]
			if retrieved.Str1 != item.Str1 || retrieved.Str2 != item.Str2 ||
				retrieved.Num != item.Num {
				t.Errorf(
					"Item %d mismatch: expected {%s, %s, %d}, got {%s, %s, %d}",
					i,
					item.Str1,
					item.Str2,
					item.Num,
					retrieved.Str1,
					retrieved.Str2,
					retrieved.Num,
				)
			}
		}
	})

	t.Run("UpdateOperationsVerify", func(t *testing.T) {
		testKey := "verify_update_key"
		initialData := []string{"initial1", "initial2"}
		updatedData := []string{"updated1", "updated2", "updated3"}
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		// Add initial data
		QueueSyncMapAdd(MapTypeString, testKey, initialData, expires, false, lastScan)

		// Verify initial data
		retrievedData := stringMap.GetVal(testKey)
		if len(retrievedData) != len(initialData) {
			t.Errorf(
				"Initial data length mismatch: expected %d, got %d",
				len(initialData),
				len(retrievedData),
			)
		}

		// Update the value
		QueueSyncMapUpdateVal(MapTypeString, testKey, updatedData)

		// Verify updated data
		retrievedData = stringMap.GetVal(testKey)
		if len(retrievedData) != len(updatedData) {
			t.Errorf(
				"Updated data length mismatch: expected %d, got %d",
				len(updatedData),
				len(retrievedData),
			)
		}
		for i, item := range updatedData {
			if i >= len(retrievedData) || retrievedData[i] != item {
				t.Errorf("Updated item %d: expected %s, got %s", i, item, retrievedData[i])
			}
		}

		// Test expire update
		newExpire := time.Now().Unix() + 7200
		QueueSyncMapUpdateExpire(MapTypeString, testKey, newExpire)

		// Test lastscan update
		newLastscan := time.Now().Unix()
		QueueSyncMapUpdateLastscan(MapTypeString, testKey, newLastscan)

		// Data should still be there after metadata updates
		retrievedData = stringMap.GetVal(testKey)
		if len(retrievedData) != len(updatedData) {
			t.Errorf(
				"Data should persist after metadata updates: expected %d, got %d",
				len(updatedData),
				len(retrievedData),
			)
		}
	})
}

// TestWorkerMapAddDeleteVerification tests worker map operations
func TestWorkerMapAddDeleteVerification(t *testing.T) {
	initOnce = sync.Once{}
	InitSyncOps()
	defer Shutdown()

	// Create SyncMapUint instances for worker maps
	scheduleMap := NewSyncMapUint[JobSchedule](100)
	queueMap := NewSyncMapUint[Job](100)

	// Register the maps
	RegisterSyncMap(MapTypeSchedule, scheduleMap)
	RegisterSyncMap(MapTypeQueue, queueMap)

	t.Run("ScheduleMapAddAndVerify", func(t *testing.T) {
		scheduleID := uint32(100)
		testSchedule := JobSchedule{
			ID:             scheduleID,
			JobName:        "verify_schedule_job",
			ScheduleTyp:    "interval",
			ScheduleString: "5m",
			LastRun:        time.Now(),
			NextRun:        time.Now().Add(5 * time.Minute),
			IsRunning:      false,
		}

		// Verify schedule doesn't exist initially
		if scheduleMap.Check(scheduleID) {
			t.Errorf("Schedule %d should not exist initially", scheduleID)
		}

		// Add the schedule
		QueueWorkerMapAdd(MapTypeSchedule, scheduleID, testSchedule)

		// Verify the schedule now exists
		if !scheduleMap.Check(scheduleID) {
			t.Errorf("Schedule %d should exist after Add operation", scheduleID)
		}

		// Verify schedule content
		retrievedSchedule := scheduleMap.GetVal(scheduleID)
		if retrievedSchedule.ID != testSchedule.ID {
			t.Errorf(
				"Schedule ID mismatch: expected %d, got %d",
				testSchedule.ID,
				retrievedSchedule.ID,
			)
		}
		if retrievedSchedule.JobName != testSchedule.JobName {
			t.Errorf(
				"Schedule JobName mismatch: expected %s, got %s",
				testSchedule.JobName,
				retrievedSchedule.JobName,
			)
		}
		if retrievedSchedule.ScheduleTyp != testSchedule.ScheduleTyp {
			t.Errorf(
				"Schedule ScheduleTyp mismatch: expected %s, got %s",
				testSchedule.ScheduleTyp,
				retrievedSchedule.ScheduleTyp,
			)
		}
		if retrievedSchedule.IsRunning != testSchedule.IsRunning {
			t.Errorf(
				"Schedule IsRunning mismatch: expected %t, got %t",
				testSchedule.IsRunning,
				retrievedSchedule.IsRunning,
			)
		}
	})

	t.Run("ScheduleMapDeleteAndVerify", func(t *testing.T) {
		scheduleID := uint32(101)
		testSchedule := JobSchedule{
			ID:        scheduleID,
			JobName:   "delete_schedule_job",
			IsRunning: false,
		}

		// Add the schedule first
		QueueWorkerMapAdd(MapTypeSchedule, scheduleID, testSchedule)

		// Verify it exists
		if !scheduleMap.Check(scheduleID) {
			t.Errorf("Schedule %d should exist after Add operation", scheduleID)
		}

		// Delete the schedule
		QueueWorkerMapDelete(MapTypeSchedule, scheduleID)

		// Verify it no longer exists
		if scheduleMap.Check(scheduleID) {
			t.Errorf("Schedule %d should not exist after Delete operation", scheduleID)
		}
	})

	t.Run("QueueMapAddAndVerify", func(t *testing.T) {
		jobID := uint32(200)
		testJob := Job{
			ID:          jobID,
			Queue:       "test_queue",
			JobName:     "verify_job",
			Name:        "Verification Job",
			Added:       time.Now(),
			SchedulerID: 100,
			Ctx:         context.Background(),
		}

		// Verify job doesn't exist initially
		if queueMap.Check(jobID) {
			t.Errorf("Job %d should not exist initially", jobID)
		}

		// Add the job
		QueueWorkerMapAdd(MapTypeQueue, jobID, testJob)

		// Verify the job now exists
		if !queueMap.Check(jobID) {
			t.Errorf("Job %d should exist after Add operation", jobID)
		}

		// Verify job content
		retrievedJob := queueMap.GetVal(jobID)
		if retrievedJob.ID != testJob.ID {
			t.Errorf("Job ID mismatch: expected %d, got %d", testJob.ID, retrievedJob.ID)
		}
		if retrievedJob.Queue != testJob.Queue {
			t.Errorf("Job Queue mismatch: expected %s, got %s", testJob.Queue, retrievedJob.Queue)
		}
		if retrievedJob.JobName != testJob.JobName {
			t.Errorf(
				"Job JobName mismatch: expected %s, got %s",
				testJob.JobName,
				retrievedJob.JobName,
			)
		}
		if retrievedJob.Name != testJob.Name {
			t.Errorf("Job Name mismatch: expected %s, got %s", testJob.Name, retrievedJob.Name)
		}
		if retrievedJob.SchedulerID != testJob.SchedulerID {
			t.Errorf(
				"Job SchedulerID mismatch: expected %d, got %d",
				testJob.SchedulerID,
				retrievedJob.SchedulerID,
			)
		}
	})

	t.Run("WorkerMapUpdateAndVerify", func(t *testing.T) {
		scheduleID := uint32(102)
		initialSchedule := JobSchedule{
			ID:        scheduleID,
			JobName:   "update_test_job",
			IsRunning: false,
		}

		// Add initial schedule
		QueueWorkerMapAdd(MapTypeSchedule, scheduleID, initialSchedule)

		// Update the schedule
		updatedSchedule := initialSchedule
		updatedSchedule.IsRunning = true
		updatedSchedule.LastRun = time.Now()
		QueueWorkerMapUpdate(MapTypeSchedule, scheduleID, updatedSchedule)

		// Verify the update
		if !scheduleMap.Check(scheduleID) {
			t.Errorf("Schedule %d should exist after Update operation", scheduleID)
		}

		retrievedSchedule := scheduleMap.GetVal(scheduleID)
		if retrievedSchedule.IsRunning != true {
			t.Errorf(
				"Schedule IsRunning should be true after update, got %t",
				retrievedSchedule.IsRunning,
			)
		}
		if retrievedSchedule.JobName != updatedSchedule.JobName {
			t.Errorf(
				"Schedule JobName should be preserved: expected %s, got %s",
				updatedSchedule.JobName,
				retrievedSchedule.JobName,
			)
		}
	})
}

// TestConcurrentAddDeleteVerification tests concurrent add/delete operations
func TestConcurrentAddDeleteVerification(t *testing.T) {
	initOnce = sync.Once{}
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMap
	stringMap := NewSyncMap[[]string](1000)
	RegisterSyncMap(MapTypeString, stringMap)

	t.Run("ConcurrentAddAndVerifyConsistency", func(t *testing.T) {
		var wg sync.WaitGroup
		numWorkers := 20
		itemsPerWorker := 50

		// Add items concurrently
		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range itemsPerWorker {
					key := fmt.Sprintf("concurrent_key_%d_%d", workerID, j)
					data := []string{fmt.Sprintf("worker_%d_item_%d", workerID, j)}
					expires := time.Now().Unix() + 3600
					lastScan := time.Now().Unix()

					QueueSyncMapAdd(MapTypeString, key, data, expires, false, lastScan)
				}
			}(i)
		}

		wg.Wait()

		// Verify all items were added
		expectedItems := numWorkers * itemsPerWorker
		actualItems := 0
		for i := range numWorkers {
			for j := range itemsPerWorker {
				key := fmt.Sprintf("concurrent_key_%d_%d", i, j)
				if stringMap.Check(key) {
					actualItems++
					// Verify content
					data := stringMap.GetVal(key)
					expectedData := fmt.Sprintf("worker_%d_item_%d", i, j)
					if len(data) != 1 || data[0] != expectedData {
						t.Errorf(
							"Key %s has incorrect data: expected [%s], got %v",
							key,
							expectedData,
							data,
						)
					}
				}
			}
		}

		if actualItems != expectedItems {
			t.Errorf("Expected %d items after concurrent add, got %d", expectedItems, actualItems)
		}
	})

	t.Run("ConcurrentDeleteAndVerifyRemoval", func(t *testing.T) {
		// First add items to delete
		numItems := 100
		for i := range numItems {
			key := fmt.Sprintf("delete_key_%d", i)
			data := []string{fmt.Sprintf("delete_item_%d", i)}
			expires := time.Now().Unix() + 3600
			lastScan := time.Now().Unix()
			QueueSyncMapAdd(MapTypeString, key, data, expires, false, lastScan)
		}

		// Verify all items exist
		for i := range numItems {
			key := fmt.Sprintf("delete_key_%d", i)
			if !stringMap.Check(key) {
				t.Errorf("Key %s should exist before deletion", key)
			}
		}

		// Delete items concurrently
		var wg sync.WaitGroup
		numWorkers := 10
		itemsPerWorker := numItems / numWorkers

		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				start := workerID * itemsPerWorker
				end := start + itemsPerWorker
				for j := start; j < end; j++ {
					key := fmt.Sprintf("delete_key_%d", j)
					QueueSyncMapDelete(MapTypeString, key)
				}
			}(i)
		}

		wg.Wait()

		// Verify all items were deleted
		remainingItems := 0
		for i := range numItems {
			key := fmt.Sprintf("delete_key_%d", i)
			if stringMap.Check(key) {
				remainingItems++
				t.Errorf("Key %s should not exist after deletion", key)
			}
		}

		if remainingItems > 0 {
			t.Errorf("Expected 0 remaining items after concurrent delete, got %d", remainingItems)
		}
	})
}

func freshManager(t *testing.T) {
	t.Helper()

	initOnce = sync.Once{}
	manager.Store(nil)
	InitSyncOps()
	t.Cleanup(Shutdown)
}

func TestSliceAppendCreatesMissingKey(t *testing.T) {
	sm := NewSyncMap[[]DbstaticOneStringTwoInt](1)

	freshManager(t)
	RegisterSyncMap(MapTypeTwoInt, sm)

	v := DbstaticOneStringTwoInt{Str: "a", Num1: 1, Num2: 2}
	if !QueueSliceAppend(MapTypeTwoInt, "k", v) {
		t.Fatal("QueueSliceAppend failed")
	}

	QueueSliceAppend(MapTypeTwoInt, "k", v) // duplicate is ignored

	if got := sm.GetVal("k"); len(got) != 1 || got[0] != v {
		t.Fatalf("expected the key to be created with one element, got %v", got)
	}
}

func TestQueueOpsReportFailure(t *testing.T) {
	freshManager(t)

	if QueueSyncMapAdd(MapType("unregistered"), "k", []string{}, 0, false, 0) {
		t.Fatal("op on an unregistered MapType reported success")
	}

	Shutdown()

	if QueueFunc(func() {}) {
		t.Fatal("op after Shutdown reported success")
	}
}

func TestQueueAtomicStringOps(t *testing.T) {
	freshManager(t)

	sm := NewSyncMap[[]string](4)
	RegisterSyncMap(MapTypeString, sm)

	if !QueueAtomicAppendString(MapTypeString, "k", "a") || !QueueAtomicAppendString(MapTypeString, "k", "b") ||
		!QueueAtomicAppendString(MapTypeString, "k", "a") {
		t.Fatal("append reported failure")
	}

	if got := sm.GetVal("k"); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("after append = %v, want [a b] (duplicates skipped)", got)
	}

	if !QueueAtomicRemoveString(MapTypeString, "k", "a") || !QueueAtomicRemoveString(MapTypeString, "k", "zz") ||
		!QueueAtomicRemoveString(MapTypeString, "missing", "a") {
		t.Fatal("remove reported failure")
	}

	if got := sm.GetVal("k"); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("after remove = %v, want [b]", got)
	}

	// Atomic string ops only apply to the string map type.
	RegisterSyncMap(MapTypeTwoString, NewSyncMap[[]DbstaticTwoStringOneInt](1))

	if QueueAtomicAppendString(MapTypeTwoString, "k", "a") || QueueAtomicRemoveString(MapTypeTwoString, "k", "a") {
		t.Error("atomic string op on a non-string map reported success")
	}
}

// TestQueueAtomicStringOpsConcurrent runs appends and removes from many
// goroutines while readers copy the slice. Run with -race.
func TestQueueAtomicStringOpsConcurrent(t *testing.T) {
	freshManager(t)

	sm := NewSyncMap[[]string](1)
	RegisterSyncMap(MapTypeString, sm)

	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			for i := range 50 {
				v := strconv.Itoa(w*100 + i)
				QueueAtomicAppendString(MapTypeString, "k", v)

				if i%2 == 1 {
					QueueAtomicRemoveString(MapTypeString, "k", v)
				}
			}
		}()

		go func() {
			defer wg.Done()

			for range 50 {
				sm.ReadInPlace("k", func(v []string) { _ = len(v) })
			}
		}()
	}

	wg.Wait()

	if got := len(sm.GetVal("k")); got != 8*25 {
		t.Fatalf("len = %d, want %d", got, 8*25)
	}
}

func TestQueueDeleteFuncsAllMapTypes(t *testing.T) {
	freshManager(t)

	two := NewSyncMap[[]DbstaticTwoStringOneInt](1)
	three := NewSyncMap[[]DbstaticThreeStringTwoInt](1)
	twoInt := NewSyncMap[[]DbstaticOneStringTwoInt](1)

	RegisterSyncMap(MapTypeTwoString, two)
	RegisterSyncMap(MapTypeThreeString, three)
	RegisterSyncMap(MapTypeTwoInt, twoInt)

	two.Add("k", nil, 1, false, 0)
	three.Add("k", nil, 1, false, 0)
	twoInt.Add("k", nil, 1, false, 0)

	all := func(int64) bool { return true }
	for _, mt := range []MapType{MapTypeTwoString, MapTypeThreeString, MapTypeTwoInt} {
		if !QueueSyncMapDeleteFuncExpires(mt, all) {
			t.Errorf("%s: DeleteFuncExpires failed", mt)
		}
	}

	if two.Check("k") || three.Check("k") || twoInt.Check("k") {
		t.Error("entries survived DeleteFuncExpires")
	}

	two.Add("k", []DbstaticTwoStringOneInt{{Str1: "a"}}, 0, false, 0)

	if !QueueSyncMapDeleteFunc(MapTypeTwoString, func(v []DbstaticTwoStringOneInt) bool { return len(v) == 1 }) ||
		two.Check("k") {
		t.Error("typed DeleteFunc on twostring map failed")
	}
}

func TestQueueSyncMapCheckExpires(t *testing.T) {
	freshManager(t)

	sm := NewSyncMap[[]string](4)
	RegisterSyncMap(MapTypeString, sm)

	sm.Add("forever", nil, 0, false, 0)
	sm.Add("fresh", nil, time.Now().Add(time.Hour).UnixNano(), false, 0)
	sm.Add("stale", nil, time.Now().Add(-time.Hour).UnixNano(), false, 0)

	cases := []struct {
		key    string
		extend bool
		want   bool
	}{
		{"forever", false, false},
		{"fresh", false, false},
		{"missing", false, true},
		{"stale", true, true},
		// The previous call extended it by 2 hours.
		{"stale", false, false},
	}

	for _, c := range cases {
		if got := QueueSyncMapCheckExpires(MapTypeString, c.key, c.extend, 2); got != c.want {
			t.Errorf("CheckExpires(%q, %t) = %t, want %t", c.key, c.extend, got, c.want)
		}
	}

	// Unsupported map types report "not expired" so callers keep their data.
	RegisterSyncMap(MapTypeStructEmpty, NewSyncMap[struct{}](1))

	if QueueSyncMapCheckExpires(MapTypeStructEmpty, "k", false, 1) {
		t.Error("unsupported map type reported expired")
	}
}

func TestQueueWorkerMapDeleteQueue(t *testing.T) {
	freshManager(t)

	jobs := NewSyncMapUint[Job](4)
	RegisterSyncMap(MapTypeQueue, jobs)

	jobs.Add(1, Job{ID: 1, Queue: "Data"})
	jobs.Add(2, Job{ID: 2, Queue: "Data", Started: time.Now()})
	jobs.Add(3, Job{ID: 3, Queue: "Search", Started: time.Now()})

	if !QueueWorkerMapDeleteQueue(MapTypeQueue, "Data", true) {
		t.Fatal("DeleteQueue (started only) failed")
	}

	if !jobs.Check(1) || jobs.Check(2) || !jobs.Check(3) {
		t.Fatalf("started-only delete left %v", jobs.GetMap())
	}

	if !QueueWorkerMapDeleteQueue(MapTypeQueue, "Data", false) {
		t.Fatal("DeleteQueue (all) failed")
	}

	if jobs.Check(1) || !jobs.Check(3) {
		t.Fatalf("queue delete left %v", jobs.GetMap())
	}

	RegisterSyncMap(MapTypeSchedule, NewSyncMapUint[JobSchedule](1))

	if QueueWorkerMapDeleteQueue(MapTypeSchedule, "Data", false) {
		t.Error("DeleteQueue on the schedule map reported success")
	}
}
