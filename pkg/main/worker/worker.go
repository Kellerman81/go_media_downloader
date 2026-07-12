package worker

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/alitto/pond/v2"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type StatsDetail struct {
	CompletedTasks  uint64 `json:"CompletedTasks"`
	DroppedTasks    uint64 `json:"DroppedTasks"`
	FailedTasks     uint64 `json:"FailedTasks"`
	RunningWorkers  uint64 `json:"RunningWorkers"`
	SubmittedTasks  uint64 `json:"SubmittedTasks"`
	SuccessfulTasks uint64 `json:"SuccessfulTasks"`
	WaitingTasks    uint64 `json:"WaitingTasks"`
}

type Stats struct {
	WorkerParse    StatsDetail                    `json:"WorkerParse"`
	WorkerSearch   StatsDetail                    `json:"WorkerSearch"`
	WorkerRSS      StatsDetail                    `json:"WorkerRSS"`
	WorkerFiles    StatsDetail                    `json:"WorkerFiles"`
	WorkerMeta     StatsDetail                    `json:"WorkerMeta"`
	WorkerIndex    StatsDetail                    `json:"WorkerIndex"`
	WorkerIndexRSS StatsDetail                    `json:"WorkerIndexRSS"`
	ListQueue      map[uint32]syncops.Job         `json:"ListQueue"`
	ListSchedule   map[uint32]syncops.JobSchedule `json:"ListSchedule"`
}

const (
	strMsg = "msg"

	// QueueData and related constants are queue names.
	QueueData   = "Data"
	QueueFeeds  = "Feeds"
	QueueSearch = "Search"
	QueueRSS    = "RSS"

	// ScheduleTypeCron and related constants are schedule types.
	ScheduleTypeCron     = "cron"
	ScheduleTypeInterval = "interval"

	// Timing constants.
	queueCheckInterval = 200 * time.Millisecond
	queueCheckDelay    = 100 * time.Millisecond
	maxQueueRetries    = 10
)

var (
	// WorkerPoolIndexer is a WorkerPool for executing indexer tasks.
	WorkerPoolIndexer pond.Pool

	// WorkerPoolIndexerRSS is a WorkerPool for executing indexer tasks for RSS Searches.
	WorkerPoolIndexerRSS pond.Pool

	// WorkerPoolParse is a WorkerPool for executing parse tasks.
	WorkerPoolParse pond.Pool

	// workerPoolSearch is a WorkerPool for executing search tasks.
	workerPoolSearch pond.Pool

	// workerPoolSearch is a WorkerPool for executing RSS search tasks.
	workerPoolRSS pond.Pool

	// workerPoolFiles is a WorkerPool for executing file tasks.
	workerPoolFiles pond.Pool

	// workerPoolMetadata is a WorkerPool for executing metadata tasks.
	workerPoolMetadata pond.Pool

	// cronWorkerData is a Cron instance for scheduling data worker jobs.
	cronWorkerData *cron.Cron

	// cronWorkerFeeds is a Cron instance for scheduling feeds worker jobs.
	cronWorkerFeeds *cron.Cron

	// cronWorkerSearch is a Cron instance for scheduling search worker jobs.
	cronWorkerSearch *cron.Cron

	// globalScheduleSet is a sync.Map to store jobSchedule objects.
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](100)

	// globalQueueSet is a sync.Map to store dispatcherQueue objects.
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](1000)

	// Recent jobs cache for duplicate detection.
	recentJobs *ristretto.Cache[string, struct{}]

	// jobNameIndex provides O(1) duplicate detection by job name. Accessed
	// directly (its methods are mutex-protected) - it must NOT be registered
	// with syncops under MapTypeStructEmpty: that type belongs to importfeed's
	// import-job map (main.go), and the previous double registration funneled
	// both packages' queued ops into one map of the wrong value type, silently
	// no-opping every write and leaving this index permanently empty.
	jobNameIndex = syncops.NewSyncMap[struct{}](1000)

	// Optimized last added times using atomic operations to reduce lock contention
	// Store Unix nanoseconds as int64 for atomic access.
	lastAddedData   atomic.Int64
	lastAddedFeeds  atomic.Int64
	lastAddedSearch atomic.Int64
	lastAddedRSS    atomic.Int64
	lastAddedOther  atomic.Int64

	ErrNotQueued             = errors.New("not queued")
	ErrQueueFull             = errors.New("queue is full")
	ErrInvalidQueue          = errors.New("invalid queue")
	errJobNotFound           = errors.New("job not found")
	errCronJobConfigNotFound = errors.New("cron Job Config not found")
	errCronJobNotFound       = errors.New("cron Job not found")

	jobAlternatives = map[string][]string{
		"searchmissinginc": {
			"searchmissinginctitle_",
			"searchmissingfull_",
			"searchmissingfulltitle_",
		},
		"searchmissinginctitle": {
			"searchmissinginc_",
			"searchmissingfull_",
			"searchmissingfulltitle_",
		},
		"searchmissingfull": {
			"searchmissinginctitle_",
			"searchmissinginc_",
			"searchmissingfulltitle_",
		},
		"searchmissingfulltitle": {
			"searchmissinginctitle_",
			"searchmissingfull_",
			"searchmissinginc_",
		},
		"searchupgradeinc": {
			"searchupgradeinctitle_",
			"searchupgradefull_",
			"searchupgradefulltitle_",
		},
		"searchupgradeinctitle": {
			"searchupgradeinc_",
			"searchupgradefull_",
			"searchupgradefulltitle_",
		},
		"searchupgradefull": {
			"searchupgradeinctitle_",
			"searchupgradeinc_",
			"searchupgradefulltitle_",
		},
		"searchupgradefulltitle": {
			"searchupgradeinctitle_",
			"searchupgradefull_",
			"searchupgradeinc_",
		},
	}
)

func init() {
	// Initialize atomic timestamps to current time
	now := time.Now().UnixNano()
	lastAddedData.Store(now)
	lastAddedFeeds.Store(now)
	lastAddedSearch.Store(now)
	lastAddedRSS.Store(now)
	lastAddedOther.Store(now)
}

type wrappedLogger struct {
	// cron.Logger
}

// Info logs an informational message to the wrapped logger.
// The message and key/value pairs are passed through to the wrapped
// zerolog Logger's Info method.
func (*wrappedLogger) Info(_ string, _ ...any) {
	// wl.logger.Info().Any("values", keysAndValues).Str("msg", msg).Msg("cron")
}

// Error logs an error message with additional key-value pairs to the wrapped logger.
// It takes in an error, a message string, and any number of key-value pairs.
func (*wrappedLogger) Error(err error, msg string, keysAndValues ...any) {
	logger.Logtype("error", 0).
		Any("values", keysAndValues).
		Str(strMsg, msg).
		Err(err).
		Msg("cron error")
}

// GetStats retrieves comprehensive statistics for all worker pools and job queues.
// Provides real-time monitoring data for system performance analysis and debugging.
//
// Returns:
//   - Stats: Comprehensive statistics structure containing:
//   - Individual worker pool metrics (completed, failed, running, etc.)
//   - Current queue contents and job details
//   - Active schedule information and timing
//
// Worker Pool Statistics Include:
//   - CompletedTasks: Total successful job completions
//   - FailedTasks: Total job failures and errors
//   - DroppedTasks: Jobs dropped due to capacity limits
//   - RunningWorkers: Currently active worker threads
//   - SubmittedTasks: Total jobs submitted to pool
//   - SuccessfulTasks: Jobs completed without errors
//   - WaitingTasks: Jobs queued but not yet started
//
// Additional Information:
//   - ListQueue: Current jobs in all queues with metadata
//   - ListSchedule: Active schedules with next run times
func GetStats() Stats {
	return Stats{
		WorkerIndex:    GetWorkerStats(WorkerPoolIndexer),
		WorkerIndexRSS: GetWorkerStats(WorkerPoolIndexerRSS),
		WorkerParse:    GetWorkerStats(WorkerPoolParse),
		WorkerSearch:   GetWorkerStats(workerPoolSearch),
		WorkerRSS:      GetWorkerStats(workerPoolRSS),
		WorkerFiles:    GetWorkerStats(workerPoolFiles),
		WorkerMeta:     GetWorkerStats(workerPoolMetadata),
		ListQueue:      GetQueues(),
		ListSchedule:   GetSchedules(),
	}
}

// GetWorkerStats extracts detailed performance metrics from a specific worker pool.
// This helper function standardizes statistics collection across all worker pool types.
//
// Parameters:
//   - w: Worker pool instance implementing the pond.Pool interface
//
// Returns:
//   - StatsDetail: Structured statistics containing all relevant performance metrics
//
// Metrics Collected:
//   - CompletedTasks: Total jobs finished (successful + failed)
//   - FailedTasks: Jobs that terminated with errors or panics
//   - DroppedTasks: Jobs rejected due to pool capacity limits
//   - RunningWorkers: Active worker threads currently processing jobs
//   - SubmittedTasks: Total jobs submitted since pool creation
//   - SuccessfulTasks: Jobs completed without errors
//   - WaitingTasks: Jobs queued awaiting available workers
//
// Performance Insights:
//   - High WaitingTasks may indicate need for more workers
//   - High FailedTasks suggests job logic or resource issues
//   - DroppedTasks indicates system overload conditions
//   - RunningWorkers shows current resource utilization
func GetWorkerStats(w pond.Pool) StatsDetail {
	if w == nil {
		return StatsDetail{}
	}

	return StatsDetail{
		CompletedTasks: w.CompletedTasks(),
		FailedTasks:    w.FailedTasks(),
		DroppedTasks:   w.DroppedTasks(),
		RunningWorkers: uint64(
			w.RunningWorkers(),
		),
		SubmittedTasks:  w.SubmittedTasks(),
		SuccessfulTasks: w.SuccessfulTasks(),
		WaitingTasks:    w.WaitingTasks(),
	}
}

// SetScheduleStarted marks a job schedule as currently running and updates timing information.
// This function is thread-safe and handles schedule state management for cron jobs
// and interval-based jobs.
//
// Parameters:
//   - id: Unique identifier of the job schedule to start
func SetScheduleStarted(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}

	s := globalScheduleSet.GetVal(id)

	s.IsRunning = true

	s.LastRun = logger.TimeGetNow()
	if s.ScheduleTyp == "cron" {
		s.NextRun = s.CronSchedule.Next(logger.TimeGetNow())
	} else {
		s.NextRun = logger.TimeGetNow().Add(s.Interval)
	}

	syncops.QueueWorkerMapUpdate(syncops.MapTypeSchedule, id, s)
}

// SetScheduleEnded marks a job schedule as no longer running.
// This function is thread-safe and ensures proper state cleanup after job completion.
//
// Parameters:
//   - id: Unique identifier of the job schedule to end
func SetScheduleEnded(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}

	s := globalScheduleSet.GetVal(id)
	if !s.IsRunning {
		return
	}

	s.IsRunning = false
	syncops.QueueWorkerMapUpdate(syncops.MapTypeSchedule, id, s)
}

// CreateCronWorker initializes three separate cron schedulers for different job categories.
// Each scheduler is configured with consistent options for timezone handling, logging,
// error recovery, and second-level precision scheduling.
//
// Cron workers created:
//   - cronWorkerData: Handles data processing and database operations
//   - cronWorkerFeeds: Handles RSS feed processing and updates
//   - cronWorkerSearch: Handles search indexing and query operations
func CreateCronWorker() {
	loggerworker := wrappedLogger{}
	opts := []cron.Option{
		cron.WithLocation(logger.GetTimeZone()),
		cron.WithLogger(&loggerworker),
		cron.WithChain(cron.Recover(&loggerworker)), // , cron.SkipIfStillRunning(&loggerworker)
		cron.WithSeconds(),
	}

	cronWorkerData = cron.New(opts...)
	cronWorkerFeeds = cron.New(opts...)
	cronWorkerSearch = cron.New(opts...)
}

// StartCronWorker starts all cron schedulers to begin executing scheduled jobs.
// This function activates the cron workers created by CreateCronWorker(),
// enabling them to process their respective job queues according to their schedules.
//
// Workers started:
//   - cronWorkerData: Begins processing data-related scheduled jobs
//   - cronWorkerFeeds: Begins processing RSS feed scheduled jobs
//   - cronWorkerSearch: Begins processing search-related scheduled jobs
func StartCronWorker() {
	cronWorkerData.Start()
	cronWorkerFeeds.Start()
	cronWorkerSearch.Start()
}

// StopCronWorker stops all cron schedulers and prevents new job executions.
// Currently running jobs will continue to completion, but no new jobs will be started.
// This provides a graceful shutdown mechanism for the cron scheduling system.
//
// Workers stopped:
//   - cronWorkerData: Stops data processing scheduled jobs
//   - cronWorkerFeeds: Stops RSS feed scheduled jobs
//   - cronWorkerSearch: Stops search-related scheduled jobs
func StopCronWorker() {
	cronWorkerData.Stop()
	cronWorkerFeeds.Stop()
	cronWorkerSearch.Stop()
}

// getcron returns the appropriate cron scheduler instance for the specified queue type.
// This function provides a centralized way to access the correct cron worker based
// on the type of jobs being scheduled.
//
// Parameters:
//   - queue: String identifier for the queue type
//
// Returns:
//   - *cron.Cron: The appropriate cron scheduler instance
//   - nil: If the queue name is not recognized
func getcron(queue string) *cron.Cron {
	switch queue {
	case QueueData:
		return cronWorkerData
	case QueueFeeds:
		return cronWorkerFeeds
	case QueueSearch:
		return cronWorkerSearch
	case QueueRSS:
		return cronWorkerSearch
	}

	return nil
}

// getPoolAndLastAdded retrieves the worker pool and the last added timestamp for a given queue type.
// Uses atomic operations for lock-free access to reduce contention under high load.
//
// Parameters:
//   - queue: String identifier for the queue type
//
// Returns:
//   - pond.Pool: The worker pool associated with the specified queue
//   - time.Time: The timestamp of when a job was last added to the queue
func getPoolAndLastAdded(queue string) (pond.Pool, time.Time) {
	switch queue {
	case QueueData:
		return workerPoolFiles, time.Unix(0, lastAddedData.Load())
	case QueueFeeds:
		return workerPoolMetadata, time.Unix(0, lastAddedFeeds.Load())
	case QueueSearch:
		return workerPoolSearch, time.Unix(0, lastAddedSearch.Load())
	case QueueRSS:
		return workerPoolRSS, time.Unix(0, lastAddedRSS.Load())
	default:
		return nil, time.Unix(0, lastAddedOther.Load())
	}
}

// updateLastAdded updates the timestamp for the last job added to a specific worker queue.
// Uses atomic operations for lock-free updates to reduce contention under high load.
// The queue parameter determines which specific queue's timestamp is updated.
//
// Parameters:
//   - queue: String identifier for the queue type (e.g., QueueData, QueueFeeds)
func updateLastAdded(queue string) {
	now := time.Now().UnixNano()

	switch queue {
	case QueueData:
		lastAddedData.Store(now)
	case QueueFeeds:
		lastAddedFeeds.Store(now)
	case QueueSearch:
		lastAddedSearch.Store(now)
	case QueueRSS:
		lastAddedRSS.Store(now)
	default:
		lastAddedOther.Store(now)
	}
}

// newUUID generates a unique 32-bit unsigned integer identifier using the UUID package.
// It returns a new UUID's ID component as a uint32 value.
//
// Returns:
//   - uint32: A unique identifier derived from a newly generated UUID
func newUUID() uint32 {
	return uuid.New().ID()
}

// validateJobConfig checks if a job configuration exists for a given configuration path and job name.
// If no configuration path is provided, it checks the general job settings.
// If a configuration path is provided, it checks the media-specific job settings.
//
// Parameters:
//   - cfgpstr: Optional configuration path string
//   - jobname: Name of the job to validate
//
// Returns:
//   - bool: True if the job configuration exists, false otherwise
func validateJobConfig(cfgpstr, jobname string) bool {
	if cfgpstr == "" {
		return config.GetSettingsGeneral().Jobs[jobname] != nil
	}

	cfg := config.GetSettingsMedia(cfgpstr)

	return cfg != nil && cfg.Jobs[jobname] != nil
}

// TestWorker provides a testing interface for manually triggering worker jobs.
// It creates a new job with the specified parameters and adds it to the queue
// for immediate execution. This function is primarily used for debugging and
// testing worker functionality outside of the normal scheduling system.
//
// Parameters:
//   - cfgpstr: Configuration prefix string for the job context
//   - name: Human-readable name for the job
//   - queue: Target queue name for job execution
//   - jobname: Specific job function name to execute
func TestWorker(cfgpstr string, name string, queue string, jobname string) {
	addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, 0)
}

// DispatchCron schedules a job to run periodically using a cron expression.
// It adds the job to a specified queue with a unique scheduler ID and returns an error if scheduling fails.
//
// Parameters:
//   - cfgpstr: Configuration path string for the job
//   - cronStr: Cron expression defining the job's schedule
//   - name: Name of the job
//   - queue: Queue to which the job will be added
//   - jobname: Specific name of the job function
//
// Returns:
//   - error: An error if the queue is invalid or scheduling fails, otherwise nil
func DispatchCron(cfgpstr string, cronStr string, name string, queue string, jobname string) error {
	schedulerID := newUUID()

	dc := getcron(queue)
	if dc == nil {
		return ErrInvalidQueue
	}

	cjob, err := dc.AddFunc(cronStr, func() {
		addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, schedulerID)
	})
	if err != nil {
		return err
	}

	dcentry := dc.Entry(cjob)
	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, schedulerID, syncops.JobSchedule{
		JobName:        name,
		JobID:          newUUID(),
		ID:             schedulerID,
		ScheduleTyp:    ScheduleTypeCron,
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dcentry.Schedule.Next(logger.TimeGetNow()),
		CronSchedule:   dcentry.Schedule,
		CronID:         cjob,
	})

	return nil
}

// acquireQueueSlot performs the shared admission checks for a job: duplicate
// detection, pool lookup, capacity check, and the per-queue rate limit. On
// success the queue's last-added timestamp is updated and the target pool is
// returned.
func acquireQueueSlot(name, queue string) (pond.Pool, error) {
	if checkQueue(name) {
		logger.Logtype("info", 1).
			Str(logger.StrJob, name).
			Msg("already queued")

		return nil, ErrNotQueued
	}

	workpool, added := getPoolAndLastAdded(queue)
	if workpool == nil {
		logger.Logtype("error", 0).
			Str(logger.StrJob, name).
			Str("queue", queue).
			Msg("invalid queue")

		return nil, ErrInvalidQueue
	}

	if err := checkQueueCapacity(
		workpool.QueueSize(),
		workpool.WaitingTasks(),
		queue,
		name,
	); err != nil {
		return nil, err
	}

	if err := waitForQueueAvailability(added, name); err != nil {
		return nil, err
	}

	updateLastAdded(queue)

	return workpool, nil
}

// submitJob registers the job in the queue map and name index, then submits
// the runner to the pool. When submission fails the registration is rolled
// back and the job context cancelled.
func submitJob(
	rootctx context.Context,
	workpool pond.Pool,
	id uint32,
	name, jobname, queue, cfgpstr string,
	schedulerID uint32,
	runner func() error,
) bool {
	ctx, cancel := context.WithCancel(rootctx)
	// cancel is stored in the Job and invoked by the runner's deferred cleanup
	// (or by the rollback below) - deferring it here would cancel the context
	// before the pooled job even starts.
	syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, id, syncops.Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		Queue:       queue,
		ID:          id,
		JobName:     jobname,
		Cfgpstr:     cfgpstr,
		SchedulerID: schedulerID,
		Ctx:         ctx,
		CancelFunc:  cancel,
	})
	jobNameIndex.Add(name, struct{}{}, 0, false, 0)

	if _, ok := workpool.TrySubmitErr(runner); !ok {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("not queued")
		cancel()
		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
		jobNameIndex.Delete(name)

		return false
	}

	return true
}

// addjob adds a job to the specified queue with the given configuration and details.
// It performs several checks before adding the job, including queue availability, capacity, and job configuration validation.
// The job is added to the global queue set and submitted to the workpool for execution.
// If any checks fail, the job is not added and an error is logged.
func addjob(
	rootctx context.Context,
	cfgpstr string,
	id uint32,
	name string,
	jobname string,
	queue string,
	schedulerID uint32,
) {
	if jobname == "" {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("empty func")
		return
	}

	workpool, err := acquireQueueSlot(name, queue)
	if err != nil {
		return
	}

	if !validateJobConfig(cfgpstr, jobname) {
		return
	}

	cleanupCompletedJobs(workpool, queue)

	submitJob(rootctx, workpool, id, name, jobname, queue, cfgpstr, schedulerID, runjobcron(id))
}

// checkQueueCapacity validates if a job can be added to a queue based on capacity limits.
// It prevents queue overflow by checking if the number of waiting tasks exceeds
// the configured capacity limit for the specified queue.
//
// Parameters:
//   - capa: Maximum capacity limit for the queue (0 means unlimited)
//   - waiting: Current number of waiting tasks in the queue
//   - queue: Name of the queue being checked
//   - name: Name of the job being queued (for logging)
//
// Returns:
//   - error: ErrQueueFull if capacity limit is reached, nil otherwise
func checkQueueCapacity(capa int, waiting uint64, queue, name string) error {
	if capa > 0 && uint64(capa) <= waiting {
		logger.Logtype("error", 0).
			Str("queue", queue).
			Str(logger.StrJob, name).
			Msg("queue limit reached")

		return ErrQueueFull
	}

	return nil
}

// waitForQueueAvailability rate-limits job admission: when the last job was
// added to this queue within queueCheckInterval, it waits in queueCheckDelay
// steps for the window to pass and gives up after maxQueueRetries.
// The previous condition compared against now+interval (a future time a past
// timestamp can never be after), so the rate limiter never engaged.
func waitForQueueAvailability(added time.Time, name string) error {
	for idx := 0; idx <= maxQueueRetries; idx++ {
		// added lies in the past; "recently added" = after (now - interval).
		if !logger.TimeAfter(added, time.Now().Add(-queueCheckInterval)) {
			break
		}

		time.Sleep(queueCheckDelay)

		if idx == maxQueueRetries {
			logger.Logtype("error", 1).
				Str(logger.StrJob, name).
				Msg("queue recently added")
			return ErrNotQueued
		}
	}

	return nil
}

// cleanupCompletedJobs checks if all tasks in the workpool have been completed and deletes the running queue if so.
// It handles two scenarios: when all submitted tasks are completed, or when all non-waiting tasks are completed.
func cleanupCompletedJobs(workpool pond.Pool, queue string) {
	if workpool.SubmittedTasks() == workpool.CompletedTasks() ||
		workpool.SubmittedTasks()-workpool.WaitingTasks() == workpool.CompletedTasks() {
		DeleteQueueRunning(queue)
	}
}

// runJobLifecycle wraps job execution with the shared queue-state management:
// existence check, started-timestamp update, and deferred cleanup (panic
// recovery, context cancel, queue and name-index removal). exec receives the
// job snapshot taken before execution.
func runJobLifecycle(id uint32, exec func(syncops.Job) error) func() error {
	return func() error {
		if !globalQueueSet.Check(id) {
			logger.Logtype("error", 1).
				Uint32("job_id", id).
				Msg("Job not found")
			return errJobNotFound
		}

		s := globalQueueSet.GetVal(id)

		defer func() {
			logger.HandlePanic()
			// Cancel the context and clean up the job from queue and name index when finished
			if s.CancelFunc != nil {
				s.CancelFunc()
			}

			syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
			jobNameIndex.Delete(s.Name)
		}()

		// Check if job was cancelled before starting
		if err := logger.CheckContextEnded(s.Ctx); err != nil {
			return err
		}

		s.Started = logger.TimeGetNow()
		syncops.QueueWorkerMapUpdate(syncops.MapTypeQueue, id, s)

		return exec(s)
	}
}

// runjobcron returns the pooled runner for a scheduled job: it tracks the
// schedule's running state and executes the configured job function.
func runjobcron(id uint32) func() error {
	return runJobLifecycle(id, func(s syncops.Job) error {
		SetScheduleStarted(s.SchedulerID)
		defer SetScheduleEnded(s.SchedulerID)

		err := executeJob(s)
		if err != nil {
			logger.Logtype("error", 2).
				Err(err).
				Str("job", s.JobName).
				Str("cfgp", s.Cfgpstr).
				Msg("Cron Job failed")
		}

		return err
	})
}

// RemoveQueueEntry removes a job from the global job queue by its unique identifier.
// This function provides safe cleanup of completed jobs from the queue.
// It does NOT cancel the job's context - use CancelQueueEntry for manual cancellation.
//
// Parameters:
//   - id: Unique identifier of the job to remove (uint32)
func RemoveQueueEntry(id uint32) {
	if id != 0 {
		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
	}
}

// CancelQueueEntry cancels a running job by its unique identifier and removes it from the queue.
// This function is used for manual cancellation (e.g., via API endpoints).
// It cancels the job's context to stop execution and then removes it from the queue.
//
// Parameters:
//   - id: Unique identifier of the job to cancel (uint32)
func CancelQueueEntry(id uint32) {
	if id != 0 {
		// Check if job exists and get it to access its cancel function
		if globalQueueSet.Check(id) {
			job := globalQueueSet.GetVal(id)
			// Cancel the job's context to stop execution
			if job.CancelFunc != nil {
				job.CancelFunc()
			}
		}

		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
	}
}

// DispatchEvery schedules a job to run repeatedly at specified time intervals.
// Creates a persistent schedule that will continue executing until the application
// stops or the schedule is manually removed. The ticker goroutine is properly
// managed and can be cancelled when the schedule is removed.
//
// Parameters:
//   - cfgpstr: Configuration prefix string for job context
//   - interval: Time duration between job executions (e.g., 5*time.Minute)
//   - name: Human-readable name for the scheduled job
//   - queue: Target queue name for job execution
//   - jobname: Specific job function name to execute
//
// Returns:
//   - error: Any error encountered during schedule setup
func DispatchEvery(
	cfgpstr string,
	interval time.Duration,
	name string,
	queue string,
	jobname string,
) error {
	schedulerID := newUUID()
	t := time.NewTicker(interval)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, schedulerID)
			}
		}
	}()

	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, schedulerID, syncops.JobSchedule{
		CancelFunc:     cancel,
		JobName:        name,
		JobID:          newUUID(),
		ID:             schedulerID,
		ScheduleTyp:    ScheduleTypeInterval,
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval),
	})

	return nil
}

// Dispatch adds a new job to the appropriate worker pool queue for immediate execution.
// This function handles job validation, queue management, capacity checking, and
// rate limiting to ensure optimal system performance.
//
// Parameters:
//   - name: Human-readable name for the job (used for logging and identification)
//   - fn: Job function to execute, must accept uint32 parameter (job ID)
//   - queue: Target worker pool queue name (e.g., "search", "files", "metadata")
//
// Returns:
//   - error: Specific error if job cannot be queued:
//   - ErrNotQueued: Function is nil or job already queued or submission failed
//   - ErrInvalidQueue: Queue name not recognized
//   - ErrQueueFull: Worker pool has reached capacity limits
func Dispatch(name string, fn func(uint32, context.Context) error, queue string) error {
	if fn == nil {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("empty func")
		return ErrNotQueued
	}

	workpool, err := acquireQueueSlot(name, queue)
	if err != nil {
		return err
	}

	id := newUUID()
	if !submitJob(
		context.Background(),
		workpool,
		id,
		name,
		name,
		queue,
		"",
		newUUID(),
		runjob(id, fn),
	) {
		return ErrNotQueued
	}

	// Add job to recent jobs cache for duplicate detection. Interval/cron jobs
	// (addjob path) intentionally skip this - short intervals would otherwise
	// block their own next run for the cache TTL.
	addRecentJob(name)

	return nil
}

// executeJob executes a job based on its configuration prefix and job name.
// It attempts to locate the job function in either general settings (when no config prefix
// is specified) or media-specific settings (when a config prefix is provided).
//
// Parameters:
//   - s: Job containing the job name, configuration prefix, and ID
func executeJob(s syncops.Job) error {
	// Check if job was cancelled before execution
	if err := logger.CheckContextEnded(s.Ctx); err != nil {
		return err
	}

	if s.Cfgpstr == "" {
		if jobFunc := config.GetSettingsGeneral().Jobs[s.JobName]; jobFunc != nil {
			return jobFunc(s.ID, s.Ctx)
		} else {
			logger.Logtype("error", 2).
				Str("job", s.JobName).
				Str("cfgp", s.Cfgpstr).
				Msg("Cron Job not found")
		}
	} else {
		cfg := config.GetSettingsMedia(s.Cfgpstr)
		if cfg == nil {
			logger.Logtype("error", 2).
				Str("job", s.JobName).
				Str("cfgp", s.Cfgpstr).
				Msg("Cron Job Config not found")

			return errCronJobConfigNotFound
		}

		if jobFunc := cfg.Jobs[s.JobName]; jobFunc != nil {
			return jobFunc(s.ID, s.Ctx)
		} else {
			logger.Logtype("error", 2).
				Str("job", s.JobName).
				Str("cfgp", s.Cfgpstr).
				Msg("Cron Job not found")

			return errCronJobNotFound
		}
	}

	return nil
}

// runjob creates a closure that wraps a job function with lifecycle management.
// The returned function handles job validation, execution tracking, and cleanup.
//
// Parameters:
//   - id: Unique identifier for the job
//   - fn: Job function to execute, receiving the job ID as parameter
//
// Returns:
//   - A closure function that manages the complete job lifecycle
func runjob(id uint32, fn func(uint32, context.Context) error) func() error {
	return runJobLifecycle(id, func(s syncops.Job) error {
		err := fn(id, s.Ctx)
		if err != nil {
			logger.Logtype("error", 0).
				Uint32("job_id", id).
				Str(logger.StrJob, s.Name).
				Err(err).
				Msg("Job failed")
		}

		return err
	})
}

// InitWorkerPools initializes all worker pools used by the application.
// Creates separate pools for different types of operations to optimize resource usage
// and provide isolation between different workload types.
//
// Parameters:
//   - workersearch: Number of workers for search operations (defaults to 1 if 0)
//   - workerfiles: Number of workers for file operations (defaults to 1 if 0)
//   - workermeta: Number of workers for metadata operations (defaults to 1 if 0)
//   - workerrss: Number of workers for RSS operations (defaults to 1 if 0)
//   - workerindex: Number of workers for indexing operations (defaults to 1 if 0)
func InitWorkerPools(
	workersearch int,
	workerfiles int,
	workermeta int,
	workerrss int,
	workerindex int,
) {
	if workersearch == 0 {
		workersearch = 1
	}

	if workerfiles == 0 {
		workerfiles = 1
	}

	if workermeta == 0 {
		workermeta = 1
	}

	if workerrss == 0 {
		workerrss = 1
	}

	if workerindex == 0 {
		workerindex = 1
	}

	workerPoolSearch = pond.NewPool(workersearch)
	workerPoolRSS = pond.NewPool(workerrss)
	workerPoolFiles = pond.NewPool(workerfiles)
	workerPoolMetadata = pond.NewPool(workermeta)
	WorkerPoolIndexer = pond.NewPool(workerindex)
	WorkerPoolIndexerRSS = pond.NewPool(workerindex)
	WorkerPoolParse = pond.NewPool(workerfiles)

	recentJobs, _ = ristretto.NewCache(&ristretto.Config[string, struct{}]{
		NumCounters: 10_000,
		MaxCost:     1 << 20, // 1MB
		BufferItems: 64,
	})
}

// closeWaitTimeout bounds how long CloseWorkerPools waits for running jobs to
// finish before cancelling their contexts; closeCancelGrace is the additional
// grace period after cancellation.
const (
	closeWaitTimeout = 30 * time.Second
	closeCancelGrace = 5 * time.Second
)

// StopIntervalSchedules cancels the ticker goroutines of all interval-based
// schedules so they stop submitting jobs. Cron schedules are stopped via
// StopCronWorker.
func StopIntervalSchedules() {
	globalScheduleSet.ForEach(func(_ uint32, s syncops.JobSchedule) {
		if s.ScheduleTyp == ScheduleTypeInterval && s.CancelFunc != nil {
			s.CancelFunc()
		}
	})
}

// CloseWorkerPools gracefully shuts down all worker pools.
// Interval schedules are stopped first so their tickers stop feeding the
// pools, then the pools stop accepting tasks and running jobs get up to
// closeWaitTimeout to finish. Jobs still running after that are cancelled via
// their contexts and given a short grace period.
// The previous implementation called Stop() without waiting (pond's Stop only
// returns a task) and immediately cancelled all job contexts, so shutdown
// could kill jobs mid-write despite documenting a graceful wait.
func CloseWorkerPools() {
	StopIntervalSchedules()

	pools := []pond.Pool{
		workerPoolSearch,
		workerPoolRSS,
		workerPoolFiles,
		workerPoolMetadata,
		WorkerPoolIndexer,
		WorkerPoolIndexerRSS,
		WorkerPoolParse,
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		defer logger.HandlePanic()

		for i := range pools {
			if pools[i] != nil {
				pools[i].StopAndWait()
			}
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(closeWaitTimeout):
	}

	// Jobs still running after the wait: cancel their contexts and allow a
	// short grace period for them to exit.
	globalQueueSet.ForEach(func(_ uint32, getjob syncops.Job) {
		if getjob.CancelFunc != nil {
			getjob.CancelFunc()
		}
	})

	select {
	case <-done:
	case <-time.After(closeCancelGrace):
		logger.Logtype("error", 0).
			Msg("worker pools did not shut down cleanly within the timeout")
	}
}

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
// It checks all worker pools (data, feeds, search, RSS) for active or waiting tasks and only
// clears the queue if all pools are idle. This prevents premature cleanup of pending jobs
// and ensures system stability during shutdown or maintenance operations.
func Cleanqueue() error {
	pools := map[string]pond.Pool{
		QueueData:   workerPoolFiles,
		QueueFeeds:  workerPoolMetadata,
		QueueSearch: workerPoolSearch,
		QueueRSS:    workerPoolRSS,
	}

	for queueName, pool := range pools {
		if pool != nil && pool.CompletedTasks() == pool.SubmittedTasks() {
			DeleteQueue(queueName)
		}
	}

	return nil
}

// GetQueues returns a map of all currently configured queues, keyed by the queue name.
// The map contains all active and pending jobs across all worker pools, providing
// a snapshot of the current system state. This is primarily used for monitoring,
// debugging, and administrative interfaces to display job status and queue health.
func GetQueues() map[uint32]syncops.Job {
	return globalQueueSet.GetMap()
}

// GetJobContext returns the context for a job with the given ID.
// Job functions can use this to check for cancellation and respond appropriately.
// Returns context.Background() if the job is not found.
func GetJobContext(id uint32) context.Context {
	if globalQueueSet.Check(id) {
		job := globalQueueSet.GetVal(id)
		if job.Ctx != nil {
			return job.Ctx
		}
	}

	return context.Background()
}

// GetSchedules returns a map of all currently configured schedules.
// The map contains all active and pending scheduled jobs across the system,
// providing a snapshot for monitoring and administrative purposes.
//
// Returns:
//   - map[uint32]syncops.JobSchedule: Map of schedule IDs to their configurations
func GetSchedules() map[uint32]syncops.JobSchedule {
	return globalScheduleSet.GetMap()
}

// GetGlobalScheduleSet returns the global schedule set for syncops registration.
// This provides access to the thread-safe schedule storage for external packages
// that need to interact with the scheduling system.
//
// Returns:
//   - *syncops.SyncMapUint[syncops.JobSchedule]: Thread-safe map of active schedules
func GetGlobalScheduleSet() *syncops.SyncMapUint[syncops.JobSchedule] {
	return globalScheduleSet
}

// GetGlobalQueueSet returns the global queue set for syncops registration.
// This provides access to the thread-safe job queue storage for external packages
// that need to interact with the job queuing system.
//
// Returns:
//   - *syncops.SyncMapUint[syncops.Job]: Thread-safe map of active jobs
func GetGlobalQueueSet() *syncops.SyncMapUint[syncops.Job] {
	return globalQueueSet
}

func addRecentJob(jobname string) {
	if recentJobs == nil {
		return
	}

	recentJobs.SetWithTTL(jobname, struct{}{}, 1, 10*time.Second)
	// Ristretto applies sets asynchronously - wait so a duplicate dispatched
	// immediately afterwards already sees the entry.
	recentJobs.Wait()
}

// isRecentJob checks if a job was recently submitted (within last 10 seconds)
// to prevent duplicate job submissions and reduce system load.
func isRecentJob(jobname string) bool {
	if recentJobs == nil {
		return false
	}

	_, ok := recentJobs.Get(jobname)

	return ok
}

// checkQueue checks if a job with the given name is currently running in any
// of the global queues. It handles checking alternate name formats that may
// have been used when adding the job. Returns true if the job is found to be
// running in a queue, false otherwise.
//
// Parameters:
//   - jobname: Name of the job to check for in the queues
//
// Returns:
//   - bool: True if job is currently running, false otherwise
func checkQueue(jobname string) bool {
	// First check if job is currently in the queue
	idx := strings.LastIndexByte(jobname, '_')

	var inQueue bool
	if idx <= 0 || idx >= len(jobname)-1 {
		inQueue = checkQueueStarted(jobname, false, "", "")
	} else {
		inQueue = checkQueueStarted(jobname, true, jobname[:idx], jobname[idx+1:])
	}

	// If not in queue, check if it was recently submitted
	if !inQueue {
		return isRecentJob(jobname)
	}

	return inQueue
}

// checkQueueStarted checks if a job with the given name is currently in the global queue.
// Optimized with O(1) lookup using job name index instead of O(n) iteration.
// It supports checking alternative job name formats based on the provided prefix and suffix.
// Returns true if the job is found in the queue, false otherwise.
func checkQueueStarted(jobname string, checkalternatives bool, prefix string, suffix string) bool {
	// O(1) check for exact match using name index
	if jobNameIndex.Check(jobname) {
		return true
	}

	// Only check alternatives if requested and alternatives exist
	if !checkalternatives {
		return false
	}

	alternatives, hasAlternatives := jobAlternatives[prefix]
	if !hasAlternatives {
		return false
	}

	// Check each alternative name with O(1) lookups instead of O(n) iteration
	for i := range alternatives {
		altName := alternatives[i] + suffix
		if jobNameIndex.Check(altName) {
			return true
		}
	}

	return false
}

// DeleteJobQueue removes jobs from the global queue set that match the given queue name.
// If isStarted is true, only jobs with a non-zero start time are deleted.
// If isStarted is false, all jobs matching the queue name are deleted.
// The contexts of the removed jobs are cancelled first - deleting without
// cancelling leaked the contexts and left running jobs uncancellable.
// CancelFunc is idempotent, so cancelling already-finished jobs is harmless.
func DeleteJobQueue(queue string, isStarted bool) {
	globalQueueSet.ForEach(func(_ uint32, job syncops.Job) {
		if job.Queue != queue || job.CancelFunc == nil {
			return
		}

		if isStarted && job.Started.IsZero() {
			return
		}

		job.CancelFunc()
	})

	syncops.QueueWorkerMapDeleteQueue(syncops.MapTypeQueue, queue, isStarted)
}

// DeleteQueue deletes all jobs from the global queue set that match the given queue name.
// This is a convenience function that removes all jobs regardless of their execution status.
//
// Parameters:
//   - queue: Name of the queue to clear of all jobs
func DeleteQueue(queue string) {
	DeleteJobQueue(queue, false)
}

// DeleteQueueRunning deletes all jobs from the global queue set that match the given queue name and have a non-zero start time.
// This function only removes jobs that are currently running or have been started, leaving pending jobs untouched.
//
// Parameters:
//   - queue: Name of the queue to clear of running jobs
func DeleteQueueRunning(queue string) {
	DeleteJobQueue(queue, true)
}

// RegisterWorkerSyncMaps registers the worker SyncMaps with the global SyncOpsManager.
// This function integrates the worker's thread-safe data structures with the global
// synchronization system, enabling coordinated access across the application.
// jobNameIndex is deliberately NOT registered: MapTypeStructEmpty belongs to
// importfeed's import-job map (registered in main.go), and registering the
// name index here used to overwrite that mapping, breaking both packages'
// queued operations. The name index is accessed directly instead.
func RegisterWorkerSyncMaps() {
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)
}
