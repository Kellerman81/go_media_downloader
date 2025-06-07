package worker

import (
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
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
	InitWorkerPools(1, 1, 1)
	defer CloseWorkerPools()

	tests := []struct {
		name      string
		jobName   string
		queue     string
		fn        func(uint32)
		wantError bool
	}{
		{
			name:      "nil function",
			jobName:   "test_job",
			queue:     QueueData,
			fn:        nil,
			wantError: true,
		},
		{
			name:    "valid job",
			jobName: "test_job",
			queue:   QueueData,
			fn: func(uint32) {
				time.Sleep(time.Millisecond)
			},
			wantError: false,
		},
		{
			name:    "duplicate job",
			jobName: "test_job",
			queue:   QueueData,
			fn: func(uint32) {
				time.Sleep(time.Millisecond)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Dispatch(tt.jobName, tt.fn, tt.queue)
			if (err != nil) != tt.wantError {
				t.Errorf("Dispatch() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
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
				globalQueueSet.Add(1, Job{
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
				globalQueueSet.Add(1, Job{
					Name:    "searchmissingfull_test",
					Started: time.Time{},
				})
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalQueueSet = syncMapUint[Job]{m: make(map[uint32]Job)}
			tt.setup()
			result := checkQueue(tt.jobName)
			if result != tt.expected {
				t.Errorf("checkQueue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExecuteJob(t *testing.T) {
	executed := false
	config.SettingsGeneral.Jobs = map[string]func(uint32){
		"test_job": func(uint32) { executed = true },
	}

	tests := []struct {
		name          string
		job           Job
		wantExecuted  bool
		setupConfig   func()
		cleanupConfig func()
	}{
		{
			name: "general job execution",
			job: Job{
				JobName: "test_job",
			},
			wantExecuted:  true,
			setupConfig:   func() {},
			cleanupConfig: func() {},
		},
		{
			name: "media job execution",
			job: Job{
				JobName: "test_media_job",
				Cfgpstr: "test_config",
			},
			wantExecuted: true,
			setupConfig: func() {
				config.SettingsMedia = make(map[string]*config.MediaTypeConfig)
				config.SettingsMedia["test_config"] = &config.MediaTypeConfig{
					Jobs: map[string]func(uint32){
						"test_media_job": func(uint32) { executed = true },
					},
				}
			},
			cleanupConfig: func() {
				config.SettingsMedia = make(map[string]*config.MediaTypeConfig)
			},
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
	sm := syncMapUint[string]{m: make(map[uint32]string)}

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
