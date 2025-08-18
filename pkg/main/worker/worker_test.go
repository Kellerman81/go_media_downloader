package worker

import (
	"context"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
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
			err := DispatchCron(tt.cfgpstr, tt.cronStr, tt.jobName, tt.queue, tt.jobFunc)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DispatchEvery(tt.jobName, tt.interval, tt.jobName, tt.queue, tt.jobFunc)
			if (err != nil) != tt.wantError {
				t.Errorf("DispatchEvery() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestDispatch(t *testing.T) {
	InitWorkerPools(1, 1, 1, 1, 1)
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
			time.Sleep(10 * time.Second)
			return nil
		}, QueueData)
		if err == nil {
			t.Errorf("Expected error for nil function, got nil")
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
				globalQueueSet.Add(1, syncops.Job{
					Name:    "test_job",
					Started: time.Time{},
				})
			},
			expected: true,
		},
		{
			name:    "alternative job names",
			jobName: "searchmissinginc_test",
			setup: func() {
				globalQueueSet.Add(1, syncops.Job{
					Name:    "searchmissingfull_test",
					Started: time.Time{},
				})
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalQueueSet = syncops.NewSyncMapUint[syncops.Job](100)
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
