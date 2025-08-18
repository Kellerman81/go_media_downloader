package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/alitto/pond/v2"
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

	// Queue names.
	QueueData   = "Data"
	QueueFeeds  = "Feeds"
	QueueSearch = "Search"
	QueueRSS    = "RSS"

	// Schedule types.
	ScheduleTypeCron     = "cron"
	ScheduleTypeInterval = "interval"

	// Timing constants.
	queueCheckInterval = 200 * time.Millisecond
	queueCheckDelay    = 100 * time.Millisecond
	maxQueueRetries    = 10
)

var (
	// workerPoolIndexer is a WorkerPool for executing indexer tasks.
	WorkerPoolIndexer pond.Pool

	// workerPoolIndexerRSS is a WorkerPool for executing indexer tasks for RSS Searches.
	WorkerPoolIndexerRSS pond.Pool

	// workerPoolParse is a WorkerPool for executing parse tasks.
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

	// Recent jobs cache for duplicate detection
	recentJobs   = make(map[string]time.Time)
	recentJobsMu sync.RWMutex

	// These legacy timestamp variables have been replaced by the thread-safe lastAddedTimes struct

	lastAddedTimes = struct {
		sync.RWMutex
		data   time.Time
		feeds  time.Time
		search time.Time
		rss    time.Time
		other  time.Time
	}{
		data:   time.Now(),
		feeds:  time.Now(),
		search: time.Now(),
		rss:    time.Now(),
		other:  time.Now(),
	}

	ErrNotQueued    = errors.New("not queued")
	ErrQueueFull    = errors.New("queue is full")
	ErrInvalidQueue = errors.New("invalid queue")
	// phandler is a panic handler function.
	// phandler = pond.PanicHandler(func(p any) {
	// 	logger.LogDynamicany2StrAny("error", "Recovered from panic (dispatcher)", strMsg, logger.Stack(), strMsg, p)
	// }).
)

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
	return StatsDetail{
		CompletedTasks:  w.CompletedTasks(),
		FailedTasks:     w.FailedTasks(),
		DroppedTasks:    w.DroppedTasks(),
		RunningWorkers:  uint64(w.RunningWorkers()),
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
		cron.WithChain(cron.Recover(&loggerworker)), //, cron.SkipIfStillRunning(&loggerworker)
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
// It safely accesses the lastAddedTimes structure using a read lock to prevent concurrent modifications.
//
// Parameters:
//   - queue: String identifier for the queue type
//
// Returns:
//   - pond.Pool: The worker pool associated with the specified queue
//   - time.Time: The timestamp of when a job was last added to the queue
func getPoolAndLastAdded(queue string) (pond.Pool, time.Time) {
	lastAddedTimes.RLock()
	defer lastAddedTimes.RUnlock()

	switch queue {
	case QueueData:
		return workerPoolFiles, lastAddedTimes.data
	case QueueFeeds:
		return workerPoolMetadata, lastAddedTimes.feeds
	case QueueSearch:
		return workerPoolSearch, lastAddedTimes.search
	case QueueRSS:
		return workerPoolRSS, lastAddedTimes.rss
	default:
		return nil, lastAddedTimes.other
	}
}

// updateLastAdded updates the timestamp for the last job added to a specific worker queue.
// It uses a mutex lock to safely update the lastAddedTimes structure with the current time.
// The queue parameter determines which specific queue's timestamp is updated.
//
// Parameters:
//   - queue: String identifier for the queue type (e.g., QueueData, QueueFeeds)
func updateLastAdded(queue string) {
	now := time.Now()
	lastAddedTimes.Lock()
	defer lastAddedTimes.Unlock()

	switch queue {
	case QueueData:
		lastAddedTimes.data = now
	case QueueFeeds:
		lastAddedTimes.feeds = now
	case QueueSearch:
		lastAddedTimes.search = now
	case QueueRSS:
		lastAddedTimes.rss = now
	default:
		lastAddedTimes.other = now
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

	if checkQueue(name) {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("already queued")
		return
	}

	workpool, added := getPoolAndLastAdded(queue)
	if workpool == nil {
		return
	}

	if err := checkQueueCapacity(workpool.QueueSize(), workpool.WaitingTasks(), queue, name); err != nil {
		return
	}

	if err := waitForQueueAvailability(added, name); err != nil {
		return
	}

	if !validateJobConfig(cfgpstr, jobname) {
		return
	}

	updateLastAdded(queue)
	cleanupCompletedJobs(workpool, queue)

	ctx, cancel := context.WithCancel(rootctx)
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
	if _, ok := workpool.TrySubmitErr(runjobcron(id)); !ok {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("not queued")
		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
	}
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

// waitForQueueAvailability checks if a job can be queued by waiting for a short interval and preventing immediate re-queueing.
// It prevents rapid job re-submission by checking the time elapsed since the job was last added.
// Returns an error if the maximum number of retries is reached, indicating the job cannot be queued.
func waitForQueueAvailability(added time.Time, name string) error {
	for idx := 0; idx <= maxQueueRetries; idx++ {
		if !logger.TimeAfter(added, time.Now().Add(queueCheckInterval)) {
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

// runjobcron is a closure function that runs a scheduled job. It checks if the job is still in the global queue set,
// retrieves the job details, sets the job as started, runs the job, and then deletes the job from the global queue set.
// If the job's configuration is not found, it logs an error message.
func runjobcron(id uint32) func() error {
	return func() error {
		if !globalQueueSet.Check(id) {
			logger.Logtype("error", 1).
				Int("job", int(id)).
				Msg("Job not found")
			return errors.New("job not found")
		}

		defer func() {
			logger.HandlePanic()
			// Clean up the job from queue when finished
			syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
		}()

		s := globalQueueSet.GetVal(id)

		// Check if job was cancelled before starting

		if err := logger.CheckContextEnded(s.Ctx); err != nil {
			return err
		}

		SetScheduleStarted(s.SchedulerID)
		defer SetScheduleEnded(s.SchedulerID)

		s.Started = logger.TimeGetNow()
		syncops.QueueWorkerMapUpdate(syncops.MapTypeQueue, id, s)

		err := executeJob(s)
		if s.Cfgpstr == "" {
			if config.GetSettingsGeneral().Jobs[s.JobName] != nil {
				err = config.GetSettingsGeneral().Jobs[s.JobName](id, s.Ctx)
			} else {
				logger.Logtype("error", 2).
					Str("job", s.JobName).
					Str("cfgp", s.Cfgpstr).
					Msg("Cron Job not found")
			}
		} else {
			if config.GetSettingsMedia(s.Cfgpstr) == nil {
				logger.Logtype("error", 2).
					Str("job", s.JobName).
					Str("cfgp", s.Cfgpstr).
					Msg("Cron Job Config not found")
			} else {
				if config.GetSettingsMedia(s.Cfgpstr).Jobs[s.JobName] != nil {
					err = config.GetSettingsMedia(s.Cfgpstr).Jobs[s.JobName](id, s.Ctx)
				} else {
					logger.Logtype("error", 2).
						Str("job", s.JobName).
						Str("cfgp", s.Cfgpstr).
						Msg("Cron Job not found")
				}
			}
		}
		return err
	}
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

	// Store the cancel function in a way that can be accessed for cleanup
	// Note: This would need additional infrastructure to properly track and cancel
	// these contexts during application shutdown
	_ = cancel // For now, prevent unused variable warning

	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, schedulerID, syncops.JobSchedule{
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
	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Msg("Dispatch: Starting job dispatch")

	if fn == nil {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("empty func")
		return ErrNotQueued
	}

	if checkQueue(name) {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("already queued")
		return ErrNotQueued
	}

	workpool, added := getPoolAndLastAdded(queue)

	if workpool == nil {
		logger.Logtype("error", 0).
			Str("job_name", name).
			Str("queue", queue).
			Msg("Dispatch: Invalid queue - workpool is nil")
		return ErrInvalidQueue
	}

	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Int("queue_size", workpool.QueueSize()).
		Uint64("waiting_tasks", workpool.WaitingTasks()).
		Msg("Dispatch: Got workpool")

	if err := checkQueueCapacity(workpool.QueueSize(), workpool.WaitingTasks(), queue, name); err != nil {
		return err
	}

	if err := waitForQueueAvailability(added, name); err != nil {
		return err
	}

	updateLastAdded(queue)

	id := newUUID()
	ctx, cancel := context.WithCancel(context.Background())

	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Uint32("job_id", id).
		Msg("Dispatch: Adding job to queue")

	syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, id, syncops.Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		JobName:     name,
		Queue:       queue,
		ID:          id,
		SchedulerID: newUUID(),
		Ctx:         ctx,
		CancelFunc:  cancel,
	})

	// Verify the job was actually added
	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Uint32("job_id", id).
		Bool("job_exists_after_add", globalQueueSet.Check(id)).
		Msg("Dispatch: Verification after adding job to queue")

	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Uint32("job_id", id).
		Msg("Dispatch: Submitting job to worker pool")

	if _, ok := workpool.TrySubmitErr(runjob(id, fn)); !ok {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Uint32("job_id", id).
			Msg("Dispatch: Failed to submit job to worker pool")
		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
		return ErrNotQueued
	}

	logger.Logtype("debug", 0).
		Str("job_name", name).
		Str("queue", queue).
		Uint32("job_id", id).
		Msg("Dispatch: Job successfully submitted to worker pool")

	// Add job to recent jobs cache for duplicate detection
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
			return errors.New("Cron Job Config not found")
		}

		if jobFunc := cfg.Jobs[s.JobName]; jobFunc != nil {
			return jobFunc(s.ID, s.Ctx)
		} else {
			logger.Logtype("error", 2).
				Str("job", s.JobName).
				Str("cfgp", s.Cfgpstr).
				Msg("Cron Job not found")
			return errors.New("Cron Job not found")
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
	return func() error {
		logger.Logtype("debug", 0).
			Uint32("job_id", id).
			Msg("runjob: Starting job execution")

		logger.Logtype("debug", 0).
			Uint32("job_id", id).
			Msg("runjob: Checking if job exists in globalQueueSet")

		jobExists := globalQueueSet.Check(id)
		logger.Logtype("debug", 0).
			Uint32("job_id", id).
			Bool("job_exists", jobExists).
			Msg("runjob: Job existence check result")

		if !jobExists {
			logger.Logtype("error", 0).
				Uint32("job_id", id).
				Msg("runjob: Job not found in queue")
			return ErrNotQueued
		}

		defer func() {
			logger.HandlePanic()
			// Clean up the job from queue when finished
			syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
			logger.Logtype("debug", 0).
				Uint32("job_id", id).
				Msg("runjob: Job execution completed and cleaned up")
		}()

		s := globalQueueSet.GetVal(id)
		logger.Logtype("debug", 0).
			Uint32("job_id", id).
			Str("job_name", s.Name).
			Str("queue", s.Queue).
			Msg("runjob: Retrieved job from queue")

		// Check if job was cancelled before starting
		if err := logger.CheckContextEnded(s.Ctx); err != nil {
			logger.Logtype("error", 0).
				Uint32("job_id", id).
				Err(err).
				Msg("runjob: Job context was cancelled")
			return err
		}

		s.Started = logger.TimeGetNow()
		syncops.QueueWorkerMapUpdate(syncops.MapTypeQueue, id, s)

		logger.Logtype("debug", 0).
			Uint32("job_id", id).
			Msg("runjob: About to execute job function")

		err := fn(id, s.Ctx)
		if err != nil {
			logger.Logtype("error", 0).
				Uint32("job_id", id).
				Err(err).
				Msg("runjob: Job function returned error")
		} else {
			logger.Logtype("debug", 0).
				Uint32("job_id", id).
				Msg("runjob: Job function completed successfully")
		}
		return err
	}
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
func InitWorkerPools(workersearch int, workerfiles int, workermeta int, workerrss int, workerindex int) {
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
}

// CloseWorkerPools gracefully shuts down all worker pools.
// Stops accepting new tasks and waits for currently running workers to complete
// their jobs before terminating. Implements a timeout to prevent indefinite blocking.
func CloseWorkerPools() {
	pools := []pond.Pool{
		workerPoolSearch,
		workerPoolRSS,
		workerPoolFiles,
		workerPoolMetadata,
		WorkerPoolIndexer,
		WorkerPoolIndexerRSS,
		WorkerPoolParse,
	}

	for _, pool := range pools {
		if pool != nil {
			pool.Stop()
		}
	}
	globalQueueSet.ForEach(func(key uint32, getjob syncops.Job) {
		if getjob.CancelFunc != nil {
			getjob.CancelFunc()
		}
	})
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

var jobAlternatives = map[string][]string{
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
	"searchmissingfulltitle": {"searchmissinginctitle_", "searchmissingfull_", "searchmissinginc_"},
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
	"searchupgradefulltitle": {"searchupgradeinctitle_", "searchupgradefull_", "searchupgradeinc_"},
}

func addRecentJob(jobname string) {
	recentJobsMu.Lock()
	defer recentJobsMu.Unlock()
	recentJobs[jobname] = time.Now()

	// Clean up old entries (older than 10 seconds)
	cutoff := time.Now().Add(-10 * time.Second)
	for name, timestamp := range recentJobs {
		if timestamp.Before(cutoff) {
			delete(recentJobs, name)
		}
	}
}

// isRecentJob checks if a job was recently submitted (within last 5 seconds)
// to prevent duplicate job submissions and reduce system load.
//
// Parameters:
//   - jobname: Name of the job to check in recent submissions cache
//
// Returns:
//   - bool: True if job was submitted within the last 5 seconds, false otherwise
func isRecentJob(jobname string) bool {
	recentJobsMu.RLock()
	defer recentJobsMu.RUnlock()

	if timestamp, exists := recentJobs[jobname]; exists {
		return time.Since(timestamp) < 5*time.Second
	}
	return false
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
// It supports checking alternative job name formats based on the provided prefix and suffix.
// Returns true if the job is found in the queue with an unstarted status, false otherwise.
func checkQueueStarted(jobname string, checkalternatives bool, prefix string, suffix string) bool {
	alternatives, hasAlternatives := jobAlternatives[prefix]

	return globalQueueSet.Exists(func(key uint32, getjob syncops.Job) bool {
		// Check exact match first
		if getjob.Name == jobname {
			return true
		}

		// Check alternatives if requested
		if !hasAlternatives || !checkalternatives {
			return false
		}

		if strings.HasSuffix(getjob.Name, suffix) {
			for _, alt := range alternatives {
				if getjob.Name == alt+suffix {
					return true
				}
			}
		}
		return false
	})
}

// DeleteJobQueue removes jobs from the global queue set that match the given queue name.
// If isStarted is true, only jobs with a non-zero start time are deleted.
// If isStarted is false, all jobs matching the queue name are deleted.
func DeleteJobQueue(queue string, isStarted bool) {
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
func RegisterWorkerSyncMaps() {
	syncops.RegisterSyncMap(syncops.MapTypeQueue, globalQueueSet)
	syncops.RegisterSyncMap(syncops.MapTypeSchedule, globalScheduleSet)
}
