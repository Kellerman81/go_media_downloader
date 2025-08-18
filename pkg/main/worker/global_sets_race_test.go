package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/robfig/cron/v3"
)

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
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < jobsPerWorker; j++ {
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
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < jobsPerWorker*2; j++ {
					jobID := uint32(j % (numWorkers * jobsPerWorker))
					_ = globalQueueSet.Check(jobID)
					_ = globalQueueSet.GetVal(jobID)
				}
			}()
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
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
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
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					// Add jobs to delete
					for k := 0; k < 5; k++ {
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
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					queues := GetQueues()
					_ = len(queues) // Use the result
					atomic.AddInt64(&operationsCount, 1)
				}
			}()
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
		for i := 0; i < 20; i++ {
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
		for i := 0; i < 15; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					jobName := fmt.Sprintf("check_test_%d", j%20)
					_ = checkQueueStarted(jobName, false, "", "")

					// Test with alternatives
					_ = checkQueueStarted("searchmissinginctitle_test", true, "searchmissinginc", "_test")
				}
			}(i)
		}

		// Concurrent job modifications
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
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
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < schedulesPerWorker; j++ {
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
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < schedulesPerWorker*2; j++ {
					scheduleID := uint32(j % (numWorkers * schedulesPerWorker))
					_ = globalScheduleSet.Check(scheduleID)
					_ = globalScheduleSet.GetVal(scheduleID)
				}
			}()
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
		for i := uint32(0); i < 20; i++ {
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
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					scheduleID := uint32(j % 20)
					SetScheduleStarted(scheduleID)
					atomic.AddInt64(&operations, 1)
				}
			}(i)
		}

		// Test SetScheduleEnded
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					scheduleID := uint32(j % 20)
					SetScheduleEnded(scheduleID)
					atomic.AddInt64(&operations, 1)
				}
			}(i)
		}

		// Test GetSchedules
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					schedules := GetSchedules()
					_ = len(schedules) // Use the result
					atomic.AddInt64(&operations, 1)
				}
			}()
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
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
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
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = GetQueues()
				_ = GetSchedules()
				time.Sleep(time.Microsecond)
			}
		}()
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
	for i := 0; i < 15; i++ {
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
	for i := 0; i < 10; i++ {
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
	for i := uint32(0); i < 500; i++ {
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
