package syncops

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// Mock types for testing
type mockScheduleMap struct {
	m  map[uint32]JobSchedule
	mu sync.RWMutex
}

func (m *mockScheduleMap) Add(key uint32, value JobSchedule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *mockScheduleMap) UpdateVal(key uint32, value JobSchedule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *mockScheduleMap) Delete(key uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

type mockQueueMap struct {
	m  map[uint32]Job
	mu sync.RWMutex
}

func (m *mockQueueMap) Add(key uint32, value Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *mockQueueMap) UpdateVal(key uint32, value Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *mockQueueMap) Delete(key uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

func TestSyncOpsManagerInit(t *testing.T) {
	// Reset global manager for test
	manager = nil
	initOnce = sync.Once{}

	InitSyncOps()

	if manager == nil {
		t.Fatal("Manager should be initialized")
	}

	if !manager.writerActive {
		t.Fatal("Writer should be active")
	}

	// Cleanup
	Shutdown()
}

func TestSyncMapOperations(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMaps
	stringMap := NewSyncMap[[]string](10)
	regexMap := NewSyncMap[*regexp.Regexp](10)

	// Register the maps
	RegisterSyncMap(MapTypeString, stringMap)
	RegisterSyncMap(MapTypeRegex, regexMap)

	// Test SyncMap Add operation
	testStrings := []string{"test1", "test2", "test3"}
	QueueSyncMapAdd(MapTypeString, "testkey", testStrings, time.Now().Unix()+3600, false, time.Now().Unix())

	// Test Regex Add operation
	testRegex := regexp.MustCompile("test.*")
	QueueSyncMapAdd(MapTypeRegex, "regexkey", *testRegex, time.Now().Unix()+3600, false, time.Now().Unix())

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
	QueueSyncMapDelete(MapTypeRegex, "regexkey")
}

func TestWorkerMapOperations(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create mock worker maps
	scheduleMap := &mockScheduleMap{m: make(map[uint32]JobSchedule)}
	queueMap := &mockQueueMap{m: make(map[uint32]Job)}

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

	// Test WorkerMap Delete operations
	QueueWorkerMapDelete(MapTypeSchedule, 123)
	QueueWorkerMapDelete(MapTypeQueue, 789)
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
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperationsPerGoroutine; j++ {
				key := "key_" + string(rune('A'+goroutineID)) + "_" + string(rune('0'+j%10))
				value := []string{"value1_" + string(rune('A'+goroutineID)), "value2_" + string(rune('0'+j%10))}
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

	for i := 0; i < numOperations; i++ {
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
	manager = nil
	initOnce = sync.Once{}

	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Test registering all supported SyncMap types
	stringMap := NewSyncMap[[]string](10)
	twoStringMap := NewSyncMap[[]DbstaticTwoStringOneInt](10)
	threeStringMap := NewSyncMap[[]DbstaticThreeStringTwoInt](10)
	twoIntMap := NewSyncMap[[]DbstaticOneStringTwoInt](10)
	xStmtMap := NewSyncMap[*sqlx.Stmt](10)
	regexMap := NewSyncMap[*regexp.Regexp](10)

	RegisterSyncMap(MapTypeString, stringMap)
	RegisterSyncMap(MapTypeTwoString, twoStringMap)
	RegisterSyncMap(MapTypeThreeString, threeStringMap)
	RegisterSyncMap(MapTypeTwoInt, twoIntMap)
	RegisterSyncMap(MapTypeXStmt, xStmtMap)
	RegisterSyncMap(MapTypeRegex, regexMap)

	// Test that all maps are registered
	if len(manager.syncMaps) != 6 {
		t.Fatalf("Expected 6 registered maps, got %d", len(manager.syncMaps))
	}

	// Test operations on each type
	QueueSyncMapAdd(MapTypeString, "str_key", []string{"test"}, time.Now().Unix()+3600, false, time.Now().Unix())
	QueueSyncMapAdd(MapTypeTwoString, "two_str_key", []DbstaticTwoStringOneInt{{Str1: "test1", Str2: "test2", Num: 1}}, time.Now().Unix()+3600, false, time.Now().Unix())
	QueueSyncMapAdd(MapTypeThreeString, "three_str_key", []DbstaticThreeStringTwoInt{{Str1: "test1", Str2: "test2", Str3: "test3", Num1: 1, Num2: 2}}, time.Now().Unix()+3600, false, time.Now().Unix())
	QueueSyncMapAdd(MapTypeTwoInt, "two_int_key", []DbstaticOneStringTwoInt{{Str: "test", Num1: 1, Num2: 2}}, time.Now().Unix()+3600, false, time.Now().Unix())

	testRegex := regexp.MustCompile("test.*")
	QueueSyncMapAdd(MapTypeRegex, "regex_key", *testRegex, time.Now().Unix()+3600, false, time.Now().Unix())
}

func TestShutdownAndReinitialization(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()

	if manager == nil {
		t.Fatal("Manager should be initialized")
	}

	// Shutdown the manager
	Shutdown()

	// Manager should still exist but not be active
	if manager.writerActive {
		t.Fatal("Writer should not be active after shutdown")
	}

	// Reinitialize - should not create a new instance due to sync.Once
	InitSyncOps()

	// Should still use the same manager instance but it should be inactive
	if manager.writerActive {
		t.Fatal("Writer should not be active after sync.Once prevents reinitialization")
	}

	// Reset for proper reinitialization
	manager = nil
	initOnce = sync.Once{}

	InitSyncOps()
	defer Shutdown()

	if !manager.writerActive {
		t.Fatal("Writer should be active after proper reinitialization")
	}
}

// TestSyncMapAddDeleteVerification tests that Add and Delete operations actually work
func TestSyncMapAddDeleteVerification(t *testing.T) {
	// Initialize syncops manager
	InitSyncOps()
	defer Shutdown()

	// Create test SyncMaps
	stringMap := NewSyncMap[[]string](100)
	twoStringMap := NewSyncMap[[]DbstaticTwoStringOneInt](100)
	regexMap := NewSyncMap[*regexp.Regexp](100)

	// Register the maps
	RegisterSyncMap(MapTypeString, stringMap)
	RegisterSyncMap(MapTypeTwoString, twoStringMap)
	RegisterSyncMap(MapTypeRegex, regexMap)

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
			if retrieved.Str1 != item.Str1 || retrieved.Str2 != item.Str2 || retrieved.Num != item.Num {
				t.Errorf("Item %d mismatch: expected {%s, %s, %d}, got {%s, %s, %d}",
					i, item.Str1, item.Str2, item.Num, retrieved.Str1, retrieved.Str2, retrieved.Num)
			}
		}
	})

	t.Run("RegexMapAddAndVerify", func(t *testing.T) {
		testKey := "verify_regex_key"
		testPattern := "verify_pattern_\\d+"
		testRegex := regexp.MustCompile(testPattern)
		expires := time.Now().Unix() + 3600
		lastScan := time.Now().Unix()

		// Verify key doesn't exist initially
		if regexMap.Check(testKey) {
			t.Errorf("Key %s should not exist initially", testKey)
		}

		// Add the regex
		QueueSyncMapAdd(MapTypeRegex, testKey, *testRegex, expires, false, lastScan)

		// Verify the key now exists
		if !regexMap.Check(testKey) {
			t.Errorf("Key %s should exist after Add operation", testKey)
		}

		// Verify the regex functionality
		retrievedRegex := regexMap.GetVal(testKey)
		testString1 := "verify_pattern_123"
		testString2 := "invalid_pattern_abc"

		if !retrievedRegex.MatchString(testString1) {
			t.Errorf("Regex should match %s", testString1)
		}
		if retrievedRegex.MatchString(testString2) {
			t.Errorf("Regex should not match %s", testString2)
		}

		// Verify pattern string
		if retrievedRegex.String() != testPattern {
			t.Errorf("Expected pattern %s, got %s", testPattern, retrievedRegex.String())
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
			t.Errorf("Initial data length mismatch: expected %d, got %d", len(initialData), len(retrievedData))
		}

		// Update the value
		QueueSyncMapUpdateVal(MapTypeString, testKey, updatedData)

		// Verify updated data
		retrievedData = stringMap.GetVal(testKey)
		if len(retrievedData) != len(updatedData) {
			t.Errorf("Updated data length mismatch: expected %d, got %d", len(updatedData), len(retrievedData))
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
			t.Errorf("Data should persist after metadata updates: expected %d, got %d", len(updatedData), len(retrievedData))
		}
	})
}

// TestWorkerMapAddDeleteVerification tests worker map operations
func TestWorkerMapAddDeleteVerification(t *testing.T) {
	// Initialize syncops manager
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
			t.Errorf("Schedule ID mismatch: expected %d, got %d", testSchedule.ID, retrievedSchedule.ID)
		}
		if retrievedSchedule.JobName != testSchedule.JobName {
			t.Errorf("Schedule JobName mismatch: expected %s, got %s", testSchedule.JobName, retrievedSchedule.JobName)
		}
		if retrievedSchedule.ScheduleTyp != testSchedule.ScheduleTyp {
			t.Errorf("Schedule ScheduleTyp mismatch: expected %s, got %s", testSchedule.ScheduleTyp, retrievedSchedule.ScheduleTyp)
		}
		if retrievedSchedule.IsRunning != testSchedule.IsRunning {
			t.Errorf("Schedule IsRunning mismatch: expected %t, got %t", testSchedule.IsRunning, retrievedSchedule.IsRunning)
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
			t.Errorf("Job JobName mismatch: expected %s, got %s", testJob.JobName, retrievedJob.JobName)
		}
		if retrievedJob.Name != testJob.Name {
			t.Errorf("Job Name mismatch: expected %s, got %s", testJob.Name, retrievedJob.Name)
		}
		if retrievedJob.SchedulerID != testJob.SchedulerID {
			t.Errorf("Job SchedulerID mismatch: expected %d, got %d", testJob.SchedulerID, retrievedJob.SchedulerID)
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
			t.Errorf("Schedule IsRunning should be true after update, got %t", retrievedSchedule.IsRunning)
		}
		if retrievedSchedule.JobName != updatedSchedule.JobName {
			t.Errorf("Schedule JobName should be preserved: expected %s, got %s", updatedSchedule.JobName, retrievedSchedule.JobName)
		}
	})
}

// TestConcurrentAddDeleteVerification tests concurrent add/delete operations
func TestConcurrentAddDeleteVerification(t *testing.T) {
	// Initialize syncops manager
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
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < itemsPerWorker; j++ {
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
		for i := 0; i < numWorkers; i++ {
			for j := 0; j < itemsPerWorker; j++ {
				key := fmt.Sprintf("concurrent_key_%d_%d", i, j)
				if stringMap.Check(key) {
					actualItems++
					// Verify content
					data := stringMap.GetVal(key)
					expectedData := fmt.Sprintf("worker_%d_item_%d", i, j)
					if len(data) != 1 || data[0] != expectedData {
						t.Errorf("Key %s has incorrect data: expected [%s], got %v", key, expectedData, data)
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
		for i := 0; i < numItems; i++ {
			key := fmt.Sprintf("delete_key_%d", i)
			data := []string{fmt.Sprintf("delete_item_%d", i)}
			expires := time.Now().Unix() + 3600
			lastScan := time.Now().Unix()
			QueueSyncMapAdd(MapTypeString, key, data, expires, false, lastScan)
		}

		// Verify all items exist
		for i := 0; i < numItems; i++ {
			key := fmt.Sprintf("delete_key_%d", i)
			if !stringMap.Check(key) {
				t.Errorf("Key %s should exist before deletion", key)
			}
		}

		// Delete items concurrently
		var wg sync.WaitGroup
		numWorkers := 10
		itemsPerWorker := numItems / numWorkers

		for i := 0; i < numWorkers; i++ {
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
		for i := 0; i < numItems; i++ {
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

// TestSyncOpsDebug tests basic syncops functionality
func TestSyncOpsDebug(t *testing.T) {
	// Initialize syncops manager
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
			t.Errorf("Direct SyncMapUint content mismatch: expected {100, direct_test}, got {%d, %s}", retrieved.ID, retrieved.JobName)
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
		QueueSyncMapAdd(MapTypeString, "queued_key", testData, time.Now().Unix()+3600, false, time.Now().Unix())

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
			t.Error("Queued Worker Add failed - key should exist immediately after synchronous operation")
		}

		// Verify content
		retrieved := scheduleMap.GetVal(200)
		if retrieved.ID != 200 || retrieved.JobName != "queued_test" {
			t.Errorf("Queued Worker Add content mismatch: expected {200, queued_test}, got {%d, %s}", retrieved.ID, retrieved.JobName)
		}

		// Queue Delete operation
		QueueWorkerMapDelete(MapTypeSchedule, 200)

		// Verify it's deleted
		if scheduleMap.Check(200) {
			t.Error("Queued Worker Delete failed - key should not exist after synchronous operation")
		}
	})
}
