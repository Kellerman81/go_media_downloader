package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/robfig/cron/v3"
)

func TestDispatchCron(t *testing.T) {
	CreateCronWorker()
	StartCronWorker()
	defer StopCronWorker()

	tests := []struct {
		name      string
		cfgpstr   string
		cronStr   string
		jobName   string
		queue     string
		jobFunc   string
		wantError bool
	}{
		{
			name:      "valid data queue job",
			cronStr:   "*/5 * * * * *",
			jobName:   "test_job",
			queue:     QueueData,
			jobFunc:   "test_func",
			wantError: false,
		},
		{
			name:      "standard 5-field cron",
			cronStr:   "0 */4 * * *",
			jobName:   "test_job5",
			queue:     QueueData,
			jobFunc:   "test_func",
			wantError: false,
		},
		{
			name:      "invalid queue",
			cronStr:   "*/5 * * * * *",
			jobName:   "test_job",
			queue:     "invalid",
			jobFunc:   "test_func",
			wantError: true,
		},
		{
			name:      "invalid cron expression",
			cronStr:   "invalid",
			jobName:   "test_job",
			queue:     QueueData,
			jobFunc:   "test_func",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DispatchCron(tt.cfgpstr, tt.cronStr, tt.jobName, tt.queue, tt.jobFunc)
			if (err != nil) != tt.wantError {
				t.Errorf("DispatchCron() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestDispatchEvery(t *testing.T) {
	tests := []struct {
		name      string
		interval  time.Duration
		jobName   string
		queue     string
		jobFunc   string
		wantError bool
	}{
		{
			name:      "valid interval job",
			interval:  time.Second * 5,
			jobName:   "test_interval_job",
			queue:     QueueData,
			jobFunc:   "test_func",
			wantError: false,
		},
	}

	// The schedule must be registered so the cleanup can cancel its ticker; a
	// leaked ticker reads globalScheduleSet while later tests replace it.
	syncops.InitSyncOps()
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](10)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := DispatchEvery(tt.jobName, tt.interval, tt.jobName, tt.queue, tt.jobFunc)
			t.Cleanup(func() { RemoveSchedule(id) })
			if (err != nil) != tt.wantError {
				t.Errorf("DispatchEvery() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestDispatch(t *testing.T) {
	InitWorkerPools(1, 1, 1, 1, 1, 1)
	defer CloseWorkerPools()

	// Initialize clean state and register with syncops
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)

	// Test nil function
	t.Run("nil function", func(t *testing.T) {
		err := Dispatch("test_job", nil, QueueData)
		if err == nil {
			t.Errorf("Expected error for nil function, got nil")
		}
	})

	t.Run("submit function", func(t *testing.T) {
		err := Dispatch("test_submit_job", func(u uint32, _ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		}, QueueData)
		if err != nil {
			t.Errorf("Expected successful dispatch, got error: %v", err)
		}
	})

	// Test duplicate detection
	t.Run("duplicate detection", func(t *testing.T) {
		jobName := "test_duplicate_job"

		// Dispatch first job
		err1 := Dispatch(jobName, func(uint32, context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}, QueueData)

		// Try to dispatch duplicate immediately - should fail
		err2 := Dispatch(jobName, func(uint32, context.Context) error {
			return nil
		}, QueueData)

		// The first job should succeed, second should fail
		if err1 != nil {
			t.Errorf("First dispatch should succeed, got error: %v", err1)
		}
		if err2 == nil {
			t.Errorf("Second dispatch should fail with duplicate error, got nil")
		}
	})
}

func TestCheckQueue(t *testing.T) {
	tests := []struct {
		name     string
		jobName  string
		setup    func()
		expected bool
	}{
		{
			name:     "empty queue",
			jobName:  "test_job",
			setup:    func() {},
			expected: false,
		},
		{
			name:    "job in queue",
			jobName: "test_job",
			setup: func() {
				jobNameIndex.Add("test_job", 1, 0, false, 0)
			},
			expected: true,
		},
		{
			name:    "alternative job names",
			jobName: "searchmissinginc_test",
			setup: func() {
				jobNameIndex.Add("searchmissingfull_test", 1, 0, false, 0)
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
			jobNameIndex = syncops.NewSyncMap[uint32](100)
			tt.setup()
			result := checkQueue(tt.jobName)
			if result != tt.expected {
				t.Errorf("checkQueue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExecuteJob(t *testing.T) {
	config.LoadCfgDB(true)
	database.InitCache()
	executed := false
	jobs := make(map[string]func(uint32, context.Context) error, 1)
	jobs["test_job"] = func(uint32, context.Context) error {
		executed = true
		return nil
	}
	config.GetSettingsGeneral().Jobs = jobs
	tests := []struct {
		name          string
		job           syncops.Job
		wantExecuted  bool
		setupConfig   func()
		cleanupConfig func()
	}{
		{
			name: "general job execution",
			job: syncops.Job{
				JobName: "test_job",
				Ctx:     context.Background(),
			},
			wantExecuted:  true,
			setupConfig:   func() {},
			cleanupConfig: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executed = false
			tt.setupConfig()
			executeJob(tt.job)
			if executed != tt.wantExecuted {
				t.Errorf("executeJob() executed = %v, want %v", executed, tt.wantExecuted)
			}
			tt.cleanupConfig()
		})
	}
}

func TestSyncMapUint(t *testing.T) {
	sm := syncops.NewSyncMapUint[string](100)

	t.Run("Add and Get", func(t *testing.T) {
		sm.Add(1, "test")
		if val := sm.GetVal(1); val != "test" {
			t.Errorf("GetVal() = %v, want %v", val, "test")
		}
	})

	t.Run("Check", func(t *testing.T) {
		if !sm.Check(1) {
			t.Error("Check() = false, want true")
		}
		if sm.Check(2) {
			t.Error("Check() = true, want false")
		}
	})

	t.Run("UpdateVal", func(t *testing.T) {
		sm.UpdateVal(1, "updated")
		if val := sm.GetVal(1); val != "updated" {
			t.Errorf("GetVal() after update = %v, want %v", val, "updated")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		sm.Delete(1)
		if sm.Check(1) {
			t.Error("Check() after delete = true, want false")
		}
	})
}

// TestGlobalQueueSetRaceConditions tests thread safety of globalQueueSet operations
// Run with: go test -race -run TestGlobalQueueSetRaceConditions
func TestGlobalQueueSetRaceConditions(t *testing.T) {
	t.Run("ConcurrentJobAddRemove", func(t *testing.T) {
		// Clear global queue for clean test
		originalQueue := globalQueueSet
		globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
		defer func() {
			globalQueueSet = originalQueue
		}()

		var wg sync.WaitGroup
		numWorkers := 20
		jobsPerWorker := 50

		// Workers adding jobs
		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range jobsPerWorker {
					ctx, cancel := context.WithCancel(context.Background())
					job := syncops.Job{
						ID:         uint32(workerID*jobsPerWorker + j),
						Name:       fmt.Sprintf("test_job_%d_%d", workerID, j),
						JobName:    "TestJob",
						Queue:      QueueData,
						Added:      time.Now(),
						Ctx:        ctx,
						CancelFunc: cancel,
					}
					globalQueueSet.Add(job.ID, job)
				}
			}(i)
		}

		// Workers removing jobs
		for i := 0; i < numWorkers/2; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < jobsPerWorker/2; j++ {
					jobID := uint32(workerID*jobsPerWorker + j)
					globalQueueSet.Delete(jobID)
				}
			}(i)
		}

		// Workers reading jobs
		for i := 0; i < numWorkers/4; i++ {
			wg.Go(func() {
				for j := 0; j < jobsPerWorker*2; j++ {
					jobID := uint32(j % (numWorkers * jobsPerWorker))
					_ = globalQueueSet.Check(jobID)
					_ = globalQueueSet.GetVal(jobID)
				}
			})
		}

		wg.Wait()

		// Clean up any remaining jobs
		globalQueueSet.ForEach(func(key uint32, job syncops.Job) {
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
		})
	})

	t.Run("ConcurrentJobOperations", func(t *testing.T) {
		// Clear global queue for clean test
		originalQueue := globalQueueSet
		globalQueueSet = syncops.NewSyncMapUint[syncops.Job](200)
		defer func() {
			globalQueueSet = originalQueue
		}()

		var wg sync.WaitGroup
		var operationsCount int64

		// Test RemoveQueueEntry
		for i := range 10 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 20 {
					ctx, cancel := context.WithCancel(context.Background())
					jobID := uint32(workerID*100 + j)
					job := syncops.Job{
						ID:         jobID,
						Name:       fmt.Sprintf("remove_test_%d", jobID),
						JobName:    "RemoveTest",
						Queue:      QueueSearch,
						Added:      time.Now(),
						Ctx:        ctx,
						CancelFunc: cancel,
					}
					globalQueueSet.Add(jobID, job)

					// Immediately try to remove it
					RemoveQueueEntry(jobID)
					atomic.AddInt64(&operationsCount, 1)
				}
			}(i)
		}

		// Test DeleteJobQueue
		for i := range 5 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 10 {
					// Add jobs to delete
					for k := range 5 {
						ctx, cancel := context.WithCancel(context.Background())
						jobID := uint32(workerID*1000 + j*10 + k)
						job := syncops.Job{
							ID:         jobID,
							Name:       fmt.Sprintf("delete_queue_test_%d", jobID),
							JobName:    "DeleteQueueTest",
							Queue:      QueueRSS,
							Added:      time.Now(),
							Started:    time.Now(), // Mark as started
							Ctx:        ctx,
							CancelFunc: cancel,
						}
						globalQueueSet.Add(jobID, job)
					}

					// Delete by queue
					DeleteJobQueue(QueueRSS, true) // Delete started jobs
					atomic.AddInt64(&operationsCount, 1)
				}
			}(i)
		}

		// Test GetQueues
		for range 5 {
			wg.Go(func() {
				for range 50 {
					queues := GetQueues()
					_ = len(queues) // Use the result
					atomic.AddInt64(&operationsCount, 1)
				}
			})
		}

		wg.Wait()
		t.Logf("Completed %d queue operations", operationsCount)

		// Clean up
		globalQueueSet.ForEach(func(key uint32, job syncops.Job) {
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
		})
	})

	t.Run("ConcurrentCheckQueueStarted", func(t *testing.T) {
		// Clear global queue for clean test
		originalQueue := globalQueueSet
		globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
		defer func() {
			globalQueueSet = originalQueue
		}()

		var wg sync.WaitGroup

		// Add some jobs for checking
		for i := range 20 {
			ctx, cancel := context.WithCancel(context.Background())
			job := syncops.Job{
				ID:         uint32(i),
				Name:       fmt.Sprintf("check_test_%d", i),
				JobName:    "CheckTest",
				Queue:      QueueData,
				Added:      time.Now(),
				Ctx:        ctx,
				CancelFunc: cancel,
			}
			globalQueueSet.Add(uint32(i), job)
		}

		// Concurrent checkers
		for i := range 15 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 100 {
					jobName := fmt.Sprintf("check_test_%d", j%20)
					_ = checkQueueStarted(jobName, false, "", "")

					// Test with alternatives
					_ = checkQueueStarted(
						"searchmissinginctitle_test",
						true,
						"searchmissinginc",
						"_test",
					)
				}
			}(i)
		}

		// Concurrent job modifications
		for i := range 5 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 20 {
					jobID := uint32(20 + workerID*20 + j)
					ctx, cancel := context.WithCancel(context.Background())
					job := syncops.Job{
						ID:         jobID,
						Name:       fmt.Sprintf("concurrent_check_%d", jobID),
						JobName:    "ConcurrentCheck",
						Queue:      QueueFeeds,
						Added:      time.Now(),
						Ctx:        ctx,
						CancelFunc: cancel,
					}
					globalQueueSet.Add(jobID, job)
					globalQueueSet.Delete(jobID)
				}
			}(i)
		}

		wg.Wait()

		// Clean up
		globalQueueSet.ForEach(func(key uint32, job syncops.Job) {
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
		})
	})
}

// TestGlobalScheduleSetRaceConditions tests thread safety of globalScheduleSet operations
func TestGlobalScheduleSetRaceConditions(t *testing.T) {
	t.Run("ConcurrentScheduleAddRemove", func(t *testing.T) {
		// Clear global schedule for clean test
		originalSchedule := globalScheduleSet
		globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](100)
		defer func() {
			globalScheduleSet = originalSchedule
		}()

		var wg sync.WaitGroup
		numWorkers := 15
		schedulesPerWorker := 30

		// Workers adding schedules
		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range schedulesPerWorker {
					scheduleID := uint32(workerID*schedulesPerWorker + j)
					schedule := syncops.JobSchedule{
						ID:             scheduleID,
						JobName:        fmt.Sprintf("schedule_job_%d", scheduleID),
						JobID:          newUUID(),
						ScheduleTyp:    ScheduleTypeInterval,
						ScheduleString: "1m",
						Interval:       time.Minute,
						NextRun:        time.Now().Add(time.Minute),
						IsRunning:      false,
					}
					globalScheduleSet.Add(scheduleID, schedule)
				}
			}(i)
		}

		// Workers removing schedules
		for i := 0; i < numWorkers/2; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < schedulesPerWorker/2; j++ {
					scheduleID := uint32(workerID*schedulesPerWorker + j)
					globalScheduleSet.Delete(scheduleID)
				}
			}(i)
		}

		// Workers reading schedules
		for i := 0; i < numWorkers/3; i++ {
			wg.Go(func() {
				for j := 0; j < schedulesPerWorker*2; j++ {
					scheduleID := uint32(j % (numWorkers * schedulesPerWorker))
					_ = globalScheduleSet.Check(scheduleID)
					_ = globalScheduleSet.GetVal(scheduleID)
				}
			})
		}

		wg.Wait()
	})

	t.Run("ConcurrentScheduleOperations", func(t *testing.T) {
		// Clear global schedule for clean test
		originalSchedule := globalScheduleSet
		globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](50)
		defer func() {
			globalScheduleSet = originalSchedule
		}()

		var wg sync.WaitGroup
		var operations int64

		// Pre-populate with some schedules
		for i := range uint32(20) {
			cronSchedule, _ := cron.ParseStandard("*/1 * * * *")
			schedule := syncops.JobSchedule{
				ID:           i,
				JobName:      fmt.Sprintf("cron_job_%d", i),
				JobID:        newUUID(),
				ScheduleTyp:  ScheduleTypeCron,
				CronSchedule: cronSchedule,
				NextRun:      time.Now().Add(time.Minute),
				IsRunning:    false,
			}
			globalScheduleSet.Add(i, schedule)
		}

		// Test SetScheduleStarted
		for i := range 10 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 20 {
					scheduleID := uint32(j % 20)
					SetScheduleStarted(scheduleID)
					atomic.AddInt64(&operations, 1)
				}
			}(i)
		}

		// Test SetScheduleEnded
		for i := range 10 {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := range 20 {
					scheduleID := uint32(j % 20)
					SetScheduleEnded(scheduleID)
					atomic.AddInt64(&operations, 1)
				}
			}(i)
		}

		// Test GetSchedules
		for range 5 {
			wg.Go(func() {
				for range 50 {
					schedules := GetSchedules()
					_ = len(schedules) // Use the result
					atomic.AddInt64(&operations, 1)
				}
			})
		}

		wg.Wait()
		t.Logf("Completed %d schedule operations", operations)
	})
}

// TestJobLifecycleRace tests the complete job lifecycle under concurrent access
func TestJobLifecycleRace(t *testing.T) {
	// Clear globals for clean test
	originalQueue := globalQueueSet
	originalSchedule := globalScheduleSet
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](50)

	// Register the test maps with syncops for async operations
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)

	defer func() {
		globalQueueSet = originalQueue
		globalScheduleSet = originalSchedule
		// Re-register original maps
		syncops.RegisterSyncMap(syncops.MapTypeQueue, originalQueue)
		syncops.RegisterSyncMap(syncops.MapTypeSchedule, originalSchedule)
	}()

	var wg sync.WaitGroup
	var completedJobs int64

	// Simulate job lifecycle: schedule -> queue -> execute -> complete
	for i := range 20 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := range 10 {
				// Create schedule
				scheduleID := uint32(workerID*10 + j)
				schedule := syncops.JobSchedule{
					ID:             scheduleID,
					JobName:        fmt.Sprintf("lifecycle_job_%d", scheduleID),
					JobID:          newUUID(),
					ScheduleTyp:    ScheduleTypeInterval,
					ScheduleString: "30s",
					Interval:       30 * time.Second,
					NextRun:        time.Now().Add(30 * time.Second),
					IsRunning:      false,
				}
				syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, scheduleID, schedule)

				// Create and queue job
				ctx, cancel := context.WithCancel(context.Background())
				jobID := newUUID()
				job := syncops.Job{
					ID:          jobID,
					Name:        fmt.Sprintf("lifecycle_job_%d_%d", workerID, j),
					JobName:     schedule.JobName,
					Queue:       QueueData,
					SchedulerID: scheduleID,
					Added:       time.Now(),
					Ctx:         ctx,
					CancelFunc:  cancel,
				}
				syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, jobID, job)

				// Simulate job execution
				SetScheduleStarted(scheduleID)

				// Update job as started
				job.Started = time.Now()
				syncops.QueueWorkerMapUpdate(syncops.MapTypeQueue, jobID, job)

				// Simulate work
				time.Sleep(time.Microsecond * time.Duration(j%10))

				// Complete job
				SetScheduleEnded(scheduleID)
				RemoveQueueEntry(jobID)

				atomic.AddInt64(&completedJobs, 1)
			}
		}(i)
	}

	// Concurrent monitoring
	for range 5 {
		wg.Go(func() {
			for range 100 {
				_ = GetQueues()
				_ = GetSchedules()
				time.Sleep(time.Microsecond)
			}
		})
	}

	wg.Wait()

	// All operations are now synchronous, no need to wait

	if completedJobs != 200 {
		t.Errorf("Expected 200 completed jobs, got %d", completedJobs)
	}

	// Verify clean state
	remainingJobs := len(GetQueues())
	if remainingJobs > 0 {
		t.Errorf("Expected 0 remaining jobs, got %d", remainingJobs)
	}
}

// TestGlobalSetsStress performs stress testing under high load
func TestGlobalSetsStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Clear globals for clean test
	originalQueue := globalQueueSet
	originalSchedule := globalScheduleSet
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](1000)
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](500)
	defer func() {
		// Clean up any remaining jobs
		globalQueueSet.ForEach(func(key uint32, job syncops.Job) {
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
		})
		globalQueueSet = originalQueue
		globalScheduleSet = originalSchedule
	}()

	var wg sync.WaitGroup
	duration := 3 * time.Second

	// High-volume job operations
	for i := range 15 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			operations := 0

			for time.Since(start) < duration {
				jobID := uint32(workerID*10000 + operations)
				ctx, cancel := context.WithCancel(context.Background())

				job := syncops.Job{
					ID:         jobID,
					Name:       fmt.Sprintf("stress_job_%d", jobID),
					JobName:    "StressTest",
					Queue:      QueueData,
					Added:      time.Now(),
					Ctx:        ctx,
					CancelFunc: cancel,
				}

				switch operations % 4 {
				case 0:
					globalQueueSet.Add(jobID, job)
				case 1:
					globalQueueSet.Delete(jobID)
				case 2:
					_ = globalQueueSet.Check(jobID)
				case 3:
					RemoveQueueEntry(jobID)
				}
				operations++
			}

			t.Logf("Job worker %d completed %d operations", workerID, operations)
		}(i)
	}

	// High-volume schedule operations
	for i := range 10 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			operations := 0

			for time.Since(start) < duration {
				scheduleID := uint32(workerID*5000 + operations)

				schedule := syncops.JobSchedule{
					ID:             scheduleID,
					JobName:        fmt.Sprintf("stress_schedule_%d", scheduleID),
					JobID:          newUUID(),
					ScheduleTyp:    ScheduleTypeInterval,
					ScheduleString: "1m",
					Interval:       time.Minute,
					NextRun:        time.Now().Add(time.Minute),
					IsRunning:      false,
				}

				switch operations % 5 {
				case 0:
					globalScheduleSet.Add(scheduleID, schedule)
				case 1:
					SetScheduleStarted(scheduleID)
				case 2:
					SetScheduleEnded(scheduleID)
				case 3:
					_ = globalScheduleSet.Check(scheduleID)
				case 4:
					globalScheduleSet.Delete(scheduleID)
				}
				operations++
			}

			t.Logf("Schedule worker %d completed %d operations", workerID, operations)
		}(i)
	}

	wg.Wait()
}

// BenchmarkGlobalSetsConcurrency benchmarks concurrent access to global sets
func BenchmarkGlobalSetsConcurrency(b *testing.B) {
	// Use separate instances for benchmarking
	testQueue := syncops.NewSyncMapUint[syncops.Job](1000)
	testSchedule := syncops.NewSyncMapUint[syncops.JobSchedule](500)

	// Pre-populate
	for i := range uint32(500) {
		ctx, cancel := context.WithCancel(context.Background())
		job := syncops.Job{
			ID:         i,
			Name:       fmt.Sprintf("bench_job_%d", i),
			JobName:    "BenchJob",
			Queue:      QueueData,
			Added:      time.Now(),
			Ctx:        ctx,
			CancelFunc: cancel,
		}
		testQueue.Add(i, job)

		schedule := syncops.JobSchedule{
			ID:          i,
			JobName:     fmt.Sprintf("bench_schedule_%d", i),
			JobID:       newUUID(),
			ScheduleTyp: ScheduleTypeInterval,
			Interval:    time.Minute,
			NextRun:     time.Now().Add(time.Minute),
			IsRunning:   false,
		}
		testSchedule.Add(i, schedule)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := uint32(0)
		for pb.Next() {
			key := i % 500
			switch i % 6 {
			case 0:
				_ = testQueue.Check(key)
			case 1:
				_ = testQueue.GetVal(key)
			case 2:
				_ = testSchedule.Check(key)
			case 3:
				_ = testSchedule.GetVal(key)
			case 4:
				_ = testQueue.GetMap()
			case 5:
				_ = testSchedule.GetMap()
			}
			i++
		}
	})

	// Clean up
	testQueue.ForEach(func(key uint32, job syncops.Job) {
		if job.CancelFunc != nil {
			job.CancelFunc()
		}
	})
}

// freshWorkerState gives a test its own pools, queue map and name index.
func freshWorkerState(t *testing.T) {
	t.Helper()

	InitWorkerPools(1, 1, 1, 1, 1, 1)
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	jobNameIndex = syncops.NewSyncMap[uint32](100)
	recentJobs.Clear() // names are reused across -count runs

	t.Cleanup(func() { PauseWorkerPools(context.Background()) })
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func queuedID(name string) uint32 {
	for id, j := range GetQueues() {
		if j.Name == name {
			return id
		}
	}

	return 0
}

// TestCancelQueuedJobReleasesName: cancelling a job still waiting in the pool
// used to leave its name in jobNameIndex forever (runJobLifecycle returned
// before installing the cleanup defer), blocking the name until restart.
func TestCancelQueuedJobReleasesName(t *testing.T) {
	freshWorkerState(t)

	release := make(chan struct{})
	if err := Dispatch("w1_blocker", func(uint32, context.Context) error {
		<-release
		return nil
	}, QueueSearch); err != nil {
		t.Fatalf("dispatch blocker: %v", err)
	}

	ran := atomic.Bool{}
	if err := Dispatch("w1_queued", func(uint32, context.Context) error {
		ran.Store(true)
		return nil
	}, QueueSearch); err != nil {
		t.Fatalf("dispatch queued: %v", err)
	}

	id := queuedID("w1_queued")
	if id == 0 {
		t.Fatal("queued job not registered")
	}

	CancelQueueEntry(id)

	if jobNameIndex.Check("w1_queued") {
		t.Fatal("CancelQueueEntry left the job name claimed")
	}

	close(release)
	waitFor(t, "blocker to finish", func() bool { return !jobNameIndex.Check("w1_blocker") })

	if ran.Load() {
		t.Fatal("cancelled job ran anyway")
	}
}

// TestRunnerWithoutQueueEntryReleasesName covers the early-return path itself
// (job removed by DeleteQueue/Cleanqueue before it started).
func TestRunnerWithoutQueueEntryReleasesName(t *testing.T) {
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](10)
	jobNameIndex = syncops.NewSyncMap[uint32](10)
	jobNameIndex.AddIfAbsent("w1_gone", 42, 0)

	err := runJobLifecycle(42, "w1_gone", func(syncops.Job) error {
		t.Fatal("exec must not run for a missing job")
		return nil
	})()

	if !errors.Is(err, errJobNotFound) {
		t.Fatalf("expected errJobNotFound, got %v", err)
	}

	if jobNameIndex.Check("w1_gone") {
		t.Fatal("early return left the job name claimed")
	}
}

// TestStaleOwnerCannotReleaseNewClaim: after the sweeper clears a hung job,
// the job's late cleanup must not delete the name claimed by its successor.
func TestStaleOwnerCannotReleaseNewClaim(t *testing.T) {
	syncops.InitSyncOps()

	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](10)
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	jobNameIndex = syncops.NewSyncMap[uint32](10)

	jobNameIndex.AddIfAbsent("w5_job", 1, 0)
	syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, 1, syncops.Job{
		ID:      1,
		Name:    "w5_job",
		Started: time.Now().Add(-staleJobThreshold - time.Hour),
	})

	sweepStaleJobs()

	if jobNameIndex.Check("w5_job") {
		t.Fatal("sweeper did not release the stale job's name")
	}

	if !jobNameIndex.AddIfAbsent("w5_job", 2, 0) {
		t.Fatal("successor could not claim the name")
	}

	releaseJobName("w5_job", 1) // the old goroutine's deferred cleanup

	if jobNameIndex.GetVal("w5_job") != 2 {
		t.Fatal("stale owner released the successor's claim")
	}
}

// TestConcurrentSameNameDispatch: the duplicate check and the name claim used
// to be separate steps, so concurrent dispatches of one name could all run.
func TestConcurrentSameNameDispatch(t *testing.T) {
	freshWorkerState(t)

	release := make(chan struct{})
	defer close(release)

	var (
		wg      sync.WaitGroup
		success atomic.Int32
	)

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if Dispatch("w4_same", func(uint32, context.Context) error {
				<-release
				return nil
			}, QueueFeeds) == nil {
				success.Add(1)
			}
		}()
	}

	wg.Wait()

	if n := success.Load(); n != 1 {
		t.Fatalf("expected exactly 1 successful dispatch, got %d", n)
	}
}

// TestReserveQueueSlotSerializes: concurrent reservations must be spaced by
// queueCheckInterval instead of all passing on the same old timestamp.
func TestReserveQueueSlotSerializes(t *testing.T) {
	var (
		last atomic.Int64
		wg   sync.WaitGroup
		ok   atomic.Int32
	)

	start := time.Now()

	for range 3 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if reserveQueueSlot(&last, "w4_rate") == nil {
				ok.Add(1)
			}
		}()
	}

	wg.Wait()

	if ok.Load() != 3 {
		t.Fatalf("expected all 3 reservations to succeed, got %d", ok.Load())
	}

	if elapsed := time.Since(start); elapsed < 2*queueCheckInterval {
		t.Fatalf("3 reservations finished in %v, want >= %v", elapsed, 2*queueCheckInterval)
	}
}

// TestJobPanicIsRecovered: logger.HandlePanic was called one level too deep
// in the lifecycle defer, so recover() never saw the panic.
func TestJobPanicIsRecovered(t *testing.T) {
	syncops.InitSyncOps()

	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](10)
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	jobNameIndex = syncops.NewSyncMap[uint32](10)

	jobNameIndex.AddIfAbsent("w8_panic", 5, 0)
	syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, 5, syncops.Job{
		ID:   5,
		Name: "w8_panic",
		Ctx:  context.Background(),
	})

	err := runJobLifecycle(5, "w8_panic", func(syncops.Job) error { panic("boom") })()
	if err == nil {
		t.Fatal("expected the recovered panic to be returned as an error")
	}

	if jobNameIndex.Check("w8_panic") || globalQueueSet.Check(5) {
		t.Fatal("panicking job was not cleaned up")
	}
}

// TestPauseFromInsideJob: a job pausing the pools it runs on (BackupDatabase)
// used to wait for itself until closeWaitTimeout and then cancel every job.
// It must wait for the other jobs only, and keep its own context alive.
func TestPauseFromInsideJob(t *testing.T) {
	freshWorkerState(t)

	otherDone := atomic.Bool{}
	if err := Dispatch("w3_other", func(uint32, context.Context) error {
		time.Sleep(300 * time.Millisecond)
		otherDone.Store(true)

		return nil
	}, QueueSearch); err != nil {
		t.Fatalf("dispatch other: %v", err)
	}

	type result struct {
		elapsed   time.Duration
		otherDone bool
		ctxErr    error
	}

	res := make(chan result, 1)

	if err := Dispatch("w3_backup", func(_ uint32, ctx context.Context) error {
		start := time.Now()
		PauseWorkerPools(ctx)
		res <- result{time.Since(start), otherDone.Load(), ctx.Err()}

		return nil
	}, QueueData); err != nil {
		t.Fatalf("dispatch backup: %v", err)
	}

	select {
	case r := <-res:
		if r.elapsed >= closeWaitTimeout {
			t.Fatalf("pause from inside a job waited for itself (%v)", r.elapsed)
		}

		if !r.otherDone {
			t.Fatal("pause returned before the other running job finished")
		}

		if r.ctxErr != nil {
			t.Fatalf("pause cancelled the calling job's own context: %v", r.ctxErr)
		}

	case <-time.After(closeWaitTimeout / 2):
		t.Fatal("pause from inside a job did not return")
	}
}

// TestPauseKeepsIntervalSchedules: the backup cycle used CloseWorkerPools,
// which cancelled every interval ticker with nothing to restart them.
func TestPauseKeepsIntervalSchedules(t *testing.T) {
	freshWorkerState(t)

	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](10)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, 77, syncops.JobSchedule{
		ID:          77,
		ScheduleTyp: ScheduleTypeInterval,
		CancelFunc:  cancel,
	})

	PauseWorkerPools(context.Background())

	if ctx.Err() != nil {
		t.Fatal("PauseWorkerPools stopped an interval schedule")
	}

	InitWorkerPools(1, 1, 1, 1, 1, 1)
	CloseWorkerPools()

	if ctx.Err() == nil {
		t.Fatal("CloseWorkerPools (shutdown) did not stop the interval schedule")
	}
}

// TestRemoveSchedule: a removed schedule stops submitting (ticker cancelled,
// cron entry gone) and leaves the schedule set.
func TestRemoveSchedule(t *testing.T) {
	syncops.InitSyncOps()

	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](10)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)

	CreateCronWorker()

	cronID, err := DispatchCron("", "0 3 * * *", "rm_cron", QueueSearch, "x")
	if err != nil {
		t.Fatal(err)
	}

	everyID, err := DispatchEvery("", time.Hour, "rm_every", QueueData, "x")
	if err != nil {
		t.Fatal(err)
	}

	cancelled := globalScheduleSet.GetVal(everyID).CancelFunc

	RemoveSchedule(cronID)
	RemoveSchedule(everyID)

	if n := len(cronWorkerSearch.Entries()); n != 0 {
		t.Fatalf("cron entry not removed, %d left", n)
	}

	if globalScheduleSet.Check(cronID) || globalScheduleSet.Check(everyID) {
		t.Fatal("removed schedules still listed")
	}

	if cancelled == nil {
		t.Fatal("interval schedule had no cancel func")
	}

	if _, err := DispatchEvery("", 0, "bad", QueueData, "x"); err == nil {
		t.Fatal("DispatchEvery accepted a zero interval")
	}
}

func TestInitWorkerPoolsParseSize(t *testing.T) {
	tests := []struct {
		name      string
		files     int
		parse     int
		wantParse int
		wantFiles int
	}{
		{"parse set", 2, 5, 5, 2},
		{"parse zero falls back to files", 3, 0, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitWorkerPools(1, tt.files, 1, 1, 1, tt.parse)
			t.Cleanup(func() { PauseWorkerPools(context.Background()) })

			if got := WorkerPoolParse().MaxConcurrency(); got != tt.wantParse {
				t.Errorf("parse pool size = %d, want %d", got, tt.wantParse)
			}

			if got := workerPoolFiles.MaxConcurrency(); got != tt.wantFiles {
				t.Errorf("files pool size = %d, want %d", got, tt.wantFiles)
			}
		})
	}
}

// TestSetSchedulePaused: a stopped schedule's timer queues no jobs while a
// manual QueueSchedule still does; a restarted schedule's timer queues again
// and its NextRun is moved into the future.
func TestSetSchedulePaused(t *testing.T) {
	config.LoadCfgDB(true)

	var runs atomic.Int32

	config.GetSettingsGeneral().Jobs = map[string]func(uint32, context.Context) error{
		"pause_job": func(uint32, context.Context) error {
			runs.Add(1)
			return nil
		},
	}

	syncops.InitSyncOps()
	InitWorkerPools(1, 1, 1, 1, 1, 1)
	defer CloseWorkerPools()

	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](10)
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](10)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)
	jobNameIndex = syncops.NewSyncMap[uint32](10)

	const id = 7

	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, id, syncops.JobSchedule{
		ID:          id,
		JobName:     "pause_job",
		JobFunc:     "pause_job",
		Queue:       QueueData,
		ScheduleTyp: ScheduleTypeInterval,
		Interval:    time.Minute,
		NextRun:     time.Now().Add(-90 * time.Minute),
	})

	if SetSchedulePaused(12345, true) {
		t.Fatal("unknown schedule reported as stopped")
	}

	if !SetSchedulePaused(id, true) || !globalScheduleSet.GetVal(id).Paused {
		t.Fatal("schedule not stopped")
	}

	queueScheduledRun("", "pause_job", "pause_job", QueueData, id)
	time.Sleep(200 * time.Millisecond)

	if runs.Load() != 0 {
		t.Fatal("stopped schedule queued a job")
	}

	if err := QueueSchedule(12345); !errors.Is(err, logger.ErrNotFound) {
		t.Fatalf("unknown schedule queued: %v", err)
	}

	if err := QueueSchedule(id); err != nil {
		t.Fatalf("manual run of stopped schedule: %v", err)
	}

	waitRuns(t, &runs, 1)

	if !SetSchedulePaused(id, false) {
		t.Fatal("schedule not started")
	}

	s := globalScheduleSet.GetVal(id)
	if s.Paused || !s.NextRun.After(time.Now()) || s.NextRun.After(time.Now().Add(time.Minute)) {
		t.Fatalf("restarted schedule: paused %v, next run %v", s.Paused, s.NextRun)
	}

	queueScheduledRun("", "pause_job", "pause_job", QueueData, id)
	waitRuns(t, &runs, 2)
}

func waitRuns(t *testing.T, runs *atomic.Int32, want int32) {
	t.Helper()

	for i := 0; i < 60 && runs.Load() < want; i++ {
		time.Sleep(50 * time.Millisecond)
	}

	if got := runs.Load(); got != want {
		t.Fatalf("ran %d jobs, want %d", got, want)
	}
}
