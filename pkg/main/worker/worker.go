package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

	// maxPoolQueueSize caps each worker pool's waiting queue - see the comment
	// at its use in InitWorkerPools.
	maxPoolQueueSize = 50_000

	// staleJobThreshold and staleSweepInterval - see sweepStaleJobs.
	staleJobThreshold  = 60 * time.Hour
	staleSweepInterval = 30 * time.Minute
)

var (
	// poolsMu guards the pool variables, recentJobs and staleSweeperCancel:
	// InitWorkerPools replaces them while dispatchers, stats readers and the
	// pool accessors read them from other goroutines.
	poolsMu sync.RWMutex

	// workerPoolIndexer is a WorkerPool for executing indexer tasks.
	workerPoolIndexer pond.Pool

	// workerPoolIndexerRSS is a WorkerPool for executing indexer tasks for RSS Searches.
	workerPoolIndexerRSS pond.Pool

	// workerPoolParse is a WorkerPool for executing parse tasks.
	workerPoolParse pond.Pool

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

	// staleSweeperCancel stops the stale-job sweeper goroutine started by
	// InitWorkerPools; see sweepStaleJobs.
	staleSweeperCancel context.CancelFunc

	// globalScheduleSet is a sync.Map to store jobSchedule objects.
	globalScheduleSet = syncops.NewSyncMapUint[syncops.JobSchedule](100)

	// globalQueueSet is a sync.Map to store dispatcherQueue objects.
	globalQueueSet = syncops.NewSyncMapUint[syncops.Job](1000)

	// Recent jobs cache for duplicate detection. Created once by the first
	// InitWorkerPools and reused across pool restarts (ristretto's Close is not
	// safe against concurrent Set/Wait, so it is never swapped out).
	recentJobs *ristretto.Cache[string, struct{}]

	// jobNameIndex maps a queued/running job name to the ID of the job that
	// owns it, for O(1) duplicate detection. The name is claimed atomically
	// (AddIfAbsent) at admission and released with a compare-and-delete on the
	// owning ID, so a stale owner (swept or cancelled job still unwinding) can
	// never release a newer job's claim. Accessed directly (its methods are
	// mutex-protected) - it must NOT be registered with syncops under
	// MapTypeStructEmpty: that type belongs to importfeed's import-job map
	// (main.go).
	jobNameIndex = syncops.NewSyncMap[uint32](1000)

	// Last-added times per queue (Unix nanoseconds), used as a rate limiter;
	// updated by compare-and-swap in reserveQueueSlot.
	lastAddedData   atomic.Int64
	lastAddedFeeds  atomic.Int64
	lastAddedSearch atomic.Int64
	lastAddedRSS    atomic.Int64
	lastAddedOther  atomic.Int64

	ErrNotQueued             = errors.New("not queued")
	ErrQueueFull             = errors.New("queue is full")
	ErrInvalidQueue          = errors.New("invalid queue")
	ErrInvalidInterval       = errors.New("schedule interval must be positive")
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
	poolsMu.RLock()
	st := Stats{
		WorkerIndex:    GetWorkerStats(workerPoolIndexer),
		WorkerIndexRSS: GetWorkerStats(workerPoolIndexerRSS),
		WorkerParse:    GetWorkerStats(workerPoolParse),
		WorkerSearch:   GetWorkerStats(workerPoolSearch),
		WorkerRSS:      GetWorkerStats(workerPoolRSS),
		WorkerFiles:    GetWorkerStats(workerPoolFiles),
		WorkerMeta:     GetWorkerStats(workerPoolMetadata),
	}
	poolsMu.RUnlock()

	st.ListQueue = GetQueues()
	st.ListSchedule = GetSchedules()

	return st
}

// WorkerPoolIndexer returns the pool for executing indexer tasks.
func WorkerPoolIndexer() pond.Pool {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	return workerPoolIndexer
}

// WorkerPoolIndexerRSS returns the pool for executing indexer tasks for RSS searches.
func WorkerPoolIndexerRSS() pond.Pool {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	return workerPoolIndexerRSS
}

// WorkerPoolParse returns the pool for executing parse tasks.
func WorkerPoolParse() pond.Pool {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	return workerPoolParse
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
// The read-modify-write runs inside the syncops writer goroutine, so it is
// serialized with every other queued schedule update and cannot lose one.
//
// Parameters:
//   - id: Unique identifier of the job schedule to start
func SetScheduleStarted(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}

	syncops.QueueFunc(func() {
		s := globalScheduleSet.GetVal(id)
		if s.ID != id {
			return
		}

		s.IsRunning = true

		s.LastRun = logger.TimeGetNow()
		if s.ScheduleTyp == ScheduleTypeCron {
			s.NextRun = s.CronSchedule.Next(logger.TimeGetNow())
		} else {
			s.NextRun = logger.TimeGetNow().Add(s.Interval)
		}

		globalScheduleSet.UpdateVal(id, s)
	})
}

// queueScheduledRun is what a schedule's cron entry or ticker fires: it
// queues one run of the job unless the schedule was stopped.
func queueScheduledRun(cfgpstr, name, jobname, queue string, schedulerID uint32) {
	if globalScheduleSet.GetVal(schedulerID).Paused {
		logger.Logtype("debug", 1).
			Str(logger.StrJob, name).
			Msg("schedule stopped - job not queued")

		return
	}

	addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, schedulerID)
}

// QueueSchedule queues one run of a schedule's job now, outside its timing
// and also while the schedule is stopped. Returns logger.ErrNotFound for an
// unknown id, otherwise addjob's error (e.g. ErrNotQueued when the job is
// already queued or running, ErrQueueFull).
func QueueSchedule(id uint32) error {
	s := globalScheduleSet.GetVal(id)
	if s.ID != id {
		return logger.ErrNotFound
	}

	return addjob(context.Background(), s.Cfgpstr, newUUID(), s.JobName, s.JobFunc, s.Queue, id)
}

// SetSchedulePaused stops (paused) or restarts one schedule without removing
// it: a stopped schedule's cron entry or ticker keeps firing but queues no
// jobs, so restarting needs no rebuild. A job it already started keeps
// running. The state lives in memory only - a restart or a scheduler config
// reload recreates every schedule running. Returns false for an unknown id.
func SetSchedulePaused(id uint32, paused bool) bool {
	if !globalScheduleSet.Check(id) {
		return false
	}

	return syncops.QueueFunc(func() {
		s := globalScheduleSet.GetVal(id)
		if s.ID != id {
			return
		}

		s.Paused = paused

		// Ticks skipped while stopped left NextRun in the past.
		if !paused {
			now := logger.TimeGetNow()
			if s.ScheduleTyp == ScheduleTypeCron {
				s.NextRun = s.CronSchedule.Next(now)
			} else if s.Interval > 0 {
				for !s.NextRun.After(now) {
					s.NextRun = s.NextRun.Add(s.Interval)
				}
			}
		}

		globalScheduleSet.UpdateVal(id, s)
	})
}

// SetScheduleEnded marks a job schedule as no longer running.
// Serialized through the syncops writer goroutine like SetScheduleStarted.
//
// Parameters:
//   - id: Unique identifier of the job schedule to end
func SetScheduleEnded(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}

	syncops.QueueFunc(func() {
		s := globalScheduleSet.GetVal(id)
		if s.ID != id || !s.IsRunning {
			return
		}

		s.IsRunning = false
		globalScheduleSet.UpdateVal(id, s)
	})
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
		// The config docs use standard 5-field cron; older configs may carry
		// a leading seconds field, so accept both.
		cron.WithParser(cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		)),
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

// getPoolAndLastAdded retrieves the worker pool and the last-added rate
// limiter timestamp for a given queue type.
//
// Parameters:
//   - queue: String identifier for the queue type
//
// Returns:
//   - pond.Pool: The worker pool associated with the specified queue
//   - *atomic.Int64: The queue's last-added timestamp (Unix nanoseconds)
func getPoolAndLastAdded(queue string) (pond.Pool, *atomic.Int64) {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	switch queue {
	case QueueData:
		return workerPoolFiles, &lastAddedData
	case QueueFeeds:
		return workerPoolMetadata, &lastAddedFeeds
	case QueueSearch:
		return workerPoolSearch, &lastAddedSearch
	case QueueRSS:
		return workerPoolRSS, &lastAddedRSS
	default:
		return nil, &lastAddedOther
	}
}

// releaseJobName removes name from jobNameIndex only while it is still owned
// by job id, so a stale owner cannot release a newer job's claim.
func releaseJobName(name string, id uint32) {
	jobNameIndex.DeleteIfMatch(name, func(owner uint32) bool { return owner == id })
}

// jobCtxKey is the context key under which submitJob stores jobCtxInfo.
type jobCtxKey struct{}

// jobCtxInfo identifies the pooled job a context belongs to, so maintenance
// code running inside a job (PauseWorkerPools) can exclude itself.
type jobCtxInfo struct {
	pool pond.Pool
	id   uint32
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
//   - uint32: the schedule ID (for RemoveSchedule)
//   - error: An error if the queue is invalid or scheduling fails, otherwise nil
func DispatchCron(
	cfgpstr string,
	cronStr string,
	name string,
	queue string,
	jobname string,
) (uint32, error) {
	schedulerID := newUUID()

	dc := getcron(queue)
	if dc == nil {
		return 0, ErrInvalidQueue
	}

	cjob, err := dc.AddFunc(cronStr, func() {
		queueScheduledRun(cfgpstr, name, jobname, queue, schedulerID)
	})
	if err != nil {
		return 0, err
	}

	dcentry := dc.Entry(cjob)
	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, schedulerID, syncops.JobSchedule{
		JobName:        name,
		JobFunc:        jobname,
		Cfgpstr:        cfgpstr,
		JobID:          newUUID(),
		ID:             schedulerID,
		ScheduleTyp:    ScheduleTypeCron,
		ScheduleString: cronStr,
		Queue:          queue,
		LastRun:        time.Time{},
		NextRun:        dcentry.Schedule.Next(logger.TimeGetNow()),
		CronSchedule:   dcentry.Schedule,
		CronID:         cjob,
	})

	return schedulerID, nil
}

// acquireQueueSlot performs the shared admission checks for a job: duplicate
// detection, pool lookup, capacity check, and the per-queue rate limit. The
// job name is claimed for id atomically before the other checks; on any
// failure the claim is released again. On success the queue's rate-limit slot
// is reserved and the target pool is returned - the caller owns the name claim
// from then on (submitJob / runJobLifecycle release it).
func acquireQueueSlot(name, queue string, id uint32) (pond.Pool, error) {
	if checkQueue(name) || !jobNameIndex.AddIfAbsent(name, id, 0) {
		logger.Logtype("info", 1).
			Str(logger.StrJob, name).
			Msg("already queued")

		return nil, ErrNotQueued
	}

	workpool, lastAdded := getPoolAndLastAdded(queue)
	if workpool == nil {
		releaseJobName(name, id)
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
		releaseJobName(name, id)
		return nil, err
	}

	if err := reserveQueueSlot(lastAdded, name); err != nil {
		releaseJobName(name, id)
		return nil, err
	}

	return workpool, nil
}

// submitJob registers the job in the queue map, then submits the runner to
// the pool. The job name must already be claimed for id (acquireQueueSlot).
// When registration or submission fails, everything is rolled back: the job
// context is cancelled and the queue entry and name claim are removed.
func submitJob(
	rootctx context.Context,
	workpool pond.Pool,
	id uint32,
	name, jobname, queue, cfgpstr string,
	schedulerID uint32,
	runner func() error,
) bool {
	ctx, cancel := context.WithCancel(
		context.WithValue(rootctx, jobCtxKey{}, jobCtxInfo{pool: workpool, id: id}),
	)
	// cancel is stored in the Job and invoked by the runner's deferred cleanup
	// (or by the rollback below) - deferring it here would cancel the context
	// before the pooled job even starts.
	if !syncops.QueueWorkerMapAdd(syncops.MapTypeQueue, id, syncops.Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		Queue:       queue,
		ID:          id,
		JobName:     jobname,
		Cfgpstr:     cfgpstr,
		SchedulerID: schedulerID,
		Ctx:         ctx,
		CancelFunc:  cancel,
	}) {
		// The runner would only find no queue entry and exit - don't submit it.
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("not queued - job registration failed")
		cancel()
		releaseJobName(name, id)

		return false
	}

	if _, ok := workpool.TrySubmitErr(runner); !ok {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("not queued")
		cancel()
		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
		releaseJobName(name, id)

		return false
	}

	return true
}

// addjob adds a job to the specified queue with the given configuration and details.
// It performs several checks before adding the job, including queue availability, capacity, and job configuration validation.
// The job is added to the global queue set and submitted to the workpool for execution.
// If any checks fail, the job is not added and an error is logged and returned.
func addjob(
	rootctx context.Context,
	cfgpstr string,
	id uint32,
	name string,
	jobname string,
	queue string,
	schedulerID uint32,
) error {
	if jobname == "" {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Msg("empty func")
		return ErrNotQueued
	}

	// Validated before admission so a misconfigured job neither consumes the
	// queue's rate-limit slot nor briefly claims its name.
	if !validateJobConfig(cfgpstr, jobname) {
		logger.Logtype("error", 1).
			Str(logger.StrJob, name).
			Str("job", jobname).
			Str("cfgp", cfgpstr).
			Msg("job not queued - no job function configured")

		return ErrNotQueued
	}

	workpool, err := acquireQueueSlot(name, queue, id)
	if err != nil {
		return err
	}

	if !submitJob(
		rootctx,
		workpool,
		id,
		name,
		jobname,
		queue,
		cfgpstr,
		schedulerID,
		runjobcron(id, name),
	) {
		return ErrNotQueued
	}

	return nil
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

// reserveQueueSlot rate-limits job admission per queue: a slot is free when
// the last job was admitted at least queueCheckInterval ago, and is taken by
// compare-and-swap on the queue's last-added timestamp, so concurrent
// dispatchers cannot both pass the check. While the window is busy it waits in
// queueCheckDelay steps and gives up after maxQueueRetries waits (re-checking
// after the last one).
func reserveQueueSlot(lastAdded *atomic.Int64, name string) error {
	for attempt := 0; ; attempt++ {
		prev := lastAdded.Load()

		now := time.Now().UnixNano()
		if now-prev >= int64(queueCheckInterval) {
			if lastAdded.CompareAndSwap(prev, now) {
				return nil
			}

			// Another dispatcher took the slot first - re-evaluate.
			continue
		}

		if attempt >= maxQueueRetries {
			logger.Logtype("error", 1).
				Str(logger.StrJob, name).
				Msg("queue recently added")

			return ErrNotQueued
		}

		time.Sleep(queueCheckDelay)
	}
}

// runJobLifecycle wraps job execution with the shared queue-state management:
// existence check, started-timestamp update, and deferred cleanup (panic
// recovery, context cancel, queue and name-index removal). exec receives the
// job snapshot taken before execution. name is the job name claimed for id;
// it is released on every exit path, including a job that was cancelled or
// removed while it was still waiting in the pool.
func runJobLifecycle(id uint32, name string, exec func(syncops.Job) error) func() error {
	return func() (err error) {
		s := globalQueueSet.GetVal(id)
		if s.ID != id {
			releaseJobName(name, id)
			logger.Logtype("error", 1).
				Uint32("job_id", id).
				Str(logger.StrJob, name).
				Msg("Job not found")

			return errJobNotFound
		}

		defer func() {
			// recover must be called directly by the deferred function -
			// a nested call (as logger.HandlePanic was used here) never
			// sees the panic.
			if r := recover(); r != nil {
				logger.Logtype("error", 0).
					Str("RECOVER", logger.Stack()).
					Any("vap", r).
					Uint32("job_id", id).
					Str(logger.StrJob, name).
					Msg("Recovered from panic in job")

				err = fmt.Errorf("job %s panicked: %v", name, r)
			}

			if s.CancelFunc != nil {
				s.CancelFunc()
			}

			syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)
			releaseJobName(name, id)
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
func runjobcron(id uint32, name string) func() error {
	return runJobLifecycle(id, name, func(s syncops.Job) error {
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
// It cancels the job's context to stop execution, removes it from the queue and
// releases its job name so the job can be dispatched again.
//
// Parameters:
//   - id: Unique identifier of the job to cancel (uint32)
func CancelQueueEntry(id uint32) {
	if id == 0 {
		return
	}

	job := globalQueueSet.GetVal(id)
	if job.ID == id && job.CancelFunc != nil {
		job.CancelFunc()
	}

	syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, id)

	if job.ID == id {
		releaseJobName(job.Name, id)
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
) (uint32, error) {
	if interval <= 0 {
		return 0, fmt.Errorf("interval %v: %w", interval, ErrInvalidInterval)
	}

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
				// Recovered per tick: a panic here would otherwise kill the
				// whole process (no recover in this goroutine) - this way it is
				// logged and the schedule keeps running.
				func() {
					defer logger.HandlePanic()
					queueScheduledRun(cfgpstr, name, jobname, queue, schedulerID)
				}()
			}
		}
	}()

	syncops.QueueWorkerMapAdd(syncops.MapTypeSchedule, schedulerID, syncops.JobSchedule{
		CancelFunc:     cancel,
		JobName:        name,
		JobFunc:        jobname,
		Cfgpstr:        cfgpstr,
		JobID:          newUUID(),
		ID:             schedulerID,
		ScheduleTyp:    ScheduleTypeInterval,
		ScheduleString: interval.String(),
		Queue:          queue,
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval),
	})

	return schedulerID, nil
}

// RemoveSchedule unregisters one schedule created by DispatchCron or
// DispatchEvery: the cron entry is removed or the ticker stopped, so it
// submits no further jobs. Jobs it already queued or started keep running -
// they live in the worker pools, independent of the schedule.
func RemoveSchedule(id uint32) {
	s := globalScheduleSet.GetVal(id)
	if s.ID != id {
		return
	}

	switch s.ScheduleTyp {
	case ScheduleTypeInterval:
		if s.CancelFunc != nil {
			s.CancelFunc()
		}
	case ScheduleTypeCron:
		if dc := getcron(s.Queue); dc != nil {
			dc.Remove(s.CronID)
		}
	}

	syncops.QueueWorkerMapDelete(syncops.MapTypeSchedule, id)
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

	id := newUUID()

	workpool, err := acquireQueueSlot(name, queue, id)
	if err != nil {
		return err
	}

	if !submitJob(
		context.Background(),
		workpool,
		id,
		name,
		name,
		queue,
		"",
		newUUID(),
		runjob(id, name, fn),
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

			return errCronJobNotFound
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
func runjob(id uint32, name string, fn func(uint32, context.Context) error) func() error {
	return runJobLifecycle(id, name, func(s syncops.Job) error {
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
//   - workerparse: Number of workers for the parse pool (defaults to workerfiles if 0)
func InitWorkerPools(
	workersearch int,
	workerfiles int,
	workermeta int,
	workerrss int,
	workerindex int,
	workerparse int,
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

	if workerparse <= 0 {
		workerparse = workerfiles
	}

	// pond's default queue size is Unbounded (math.MaxInt), so checkQueueCapacity's
	// "queue full" guard never actually trips - a pool whose workers are all stuck
	// on hung jobs would otherwise accept new submissions forever, growing the
	// waiting queue without bound instead of failing loudly. Capping it still
	// leaves plenty of headroom for legitimate backlogs (e.g. large RSS/search
	// batches) while turning silent unbounded growth into a clean, already-handled
	// ErrQueueFull once something is actually wrong.
	queueOpt := pond.WithQueueSize(maxPoolQueueSize)

	poolsMu.Lock()
	defer poolsMu.Unlock()

	workerPoolSearch = pond.NewPool(workersearch, queueOpt)
	workerPoolRSS = pond.NewPool(workerrss, queueOpt)
	workerPoolFiles = pond.NewPool(workerfiles, queueOpt)
	workerPoolMetadata = pond.NewPool(workermeta, queueOpt)
	workerPoolIndexer = pond.NewPool(workerindex, queueOpt)
	workerPoolIndexerRSS = pond.NewPool(workerindex, queueOpt)
	workerPoolParse = pond.NewPool(workerparse, queueOpt)

	if recentJobs == nil {
		recentJobs, _ = ristretto.NewCache(&ristretto.Config[string, struct{}]{
			NumCounters: 10_000,
			MaxCost:     1 << 20, // 1MB
			BufferItems: 64,
		})
	}

	if staleSweeperCancel != nil {
		staleSweeperCancel()
	}

	staleSweeperCancel = startStaleJobSweeper()
}

// sweepStaleJobs clears the queue and job-name-index bookkeeping for any job
// that has been in the "Started" state for longer than staleJobThreshold.
//
// Job runtimes here range from seconds to multiple days, so there's no safe
// universal execution timeout to enforce (see runJobLifecycle - a job's
// bookkeeping is normally only cleared once its function returns, which a
// genuinely hung job never does, permanently blocking that job name from
// ever being re-dispatched for the rest of the process's uptime). 60h is
// deliberately far above any legitimate job duration, so this only fires for
// jobs that are almost certainly stuck.
//
// This does NOT and cannot force-kill the underlying goroutine - Go has no
// such mechanism. It best-effort cancels the job's context (a no-op if the
// job doesn't check it, which is presumably why it's stuck) and then forgets
// the bookkeeping so the job name becomes dispatchable again. If the old
// goroutine is in fact still running and later completes, its own deferred
// cleanup in runJobLifecycle will safely no-op: the queue delete is
// delete-if-present and the name release only removes the name while this
// job's ID still owns it, so a re-dispatched job keeps its claim.
func sweepStaleJobs() {
	now := logger.TimeGetNow()

	// ForEach holds globalQueueSet's read lock for the whole callback (see
	// SyncMapUint.ForEach in syncops/maps.go). QueueWorkerMapDelete is
	// synchronous and its handler needs the write lock on this same map, so
	// calling it from inside the callback would deadlock the single writer
	// goroutine against this read lock - and since that goroutine is the only
	// thing draining the sync-ops queue, the whole app's syncops layer would
	// freeze with it. Collect the stale entries here and act on them only
	// after ForEach has released the lock.
	var stale []syncops.Job

	globalQueueSet.ForEach(func(_ uint32, job syncops.Job) {
		if job.Started.IsZero() || now.Sub(job.Started) < staleJobThreshold {
			return
		}

		stale = append(stale, job)
	})

	for _, job := range stale {
		logger.Logtype("error", 0).
			Uint32("job_id", job.ID).
			Str(logger.StrJob, job.Name).
			Time("started", job.Started).
			Msg("job exceeded stale threshold - clearing bookkeeping so its name can be re-dispatched; the job itself may still be running if it ignores cancellation")

		if job.CancelFunc != nil {
			job.CancelFunc()
		}

		syncops.QueueWorkerMapDelete(syncops.MapTypeQueue, job.ID)
		releaseJobName(job.Name, job.ID)
	}
}

// startStaleJobSweeper runs sweepStaleJobs on a fixed interval until the
// returned cancel func is called (see CloseWorkerPools).
func startStaleJobSweeper() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer logger.HandlePanic()

		ticker := time.NewTicker(staleSweepInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweepStaleJobs()
			}
		}
	}()

	return cancel
}

// closeWaitTimeout bounds how long CloseWorkerPools waits for running jobs to
// finish before cancelling their contexts; closeCancelGrace is the additional
// grace period after cancellation.
const (
	closeWaitTimeout = 30 * time.Second
	closeCancelGrace = 5 * time.Second
)

// StopIntervalSchedules cancels the ticker goroutines of all interval-based
// schedules so they stop submitting jobs, permanently: nothing restarts them,
// so this is for process shutdown only (see CloseWorkerPools). Cron schedules
// are stopped via StopCronWorker.
func StopIntervalSchedules() {
	globalScheduleSet.ForEach(func(_ uint32, s syncops.JobSchedule) {
		if s.ScheduleTyp == ScheduleTypeInterval && s.CancelFunc != nil {
			s.CancelFunc()
		}
	})
}

// CloseWorkerPools gracefully shuts down all worker pools for process
// shutdown. Interval schedules are stopped first so their tickers stop feeding
// the pools, then the pools are drained as in PauseWorkerPools.
// For a temporary stop (e.g. around a database backup) use PauseWorkerPools:
// interval schedules cannot be restarted once stopped here.
func CloseWorkerPools() {
	StopIntervalSchedules()
	PauseWorkerPools(context.Background())
}

// PauseWorkerPools stops all worker pools for a maintenance cycle and waits
// for their jobs to finish; InitWorkerPools (plus StartCronWorker after
// StopCronWorker) resumes work afterwards. Interval schedules are left running
// - their tickers' submissions fail ("not queued") while the pools are stopped
// and succeed again once InitWorkerPools has installed new pools.
//
// The pools stop accepting tasks and running jobs get up to closeWaitTimeout
// to finish; jobs still running after that are cancelled via their contexts
// and given a short grace period.
//
// When ctx belongs to a pooled job (the ctx a job function receives), that job
// is excluded: its own pool is stopped without waiting for it, and its context
// is not cancelled. Without this, a job pausing the pools it runs on (the
// BackupDatabase job) waited for itself until the timeout and then cancelled
// every running job.
func PauseWorkerPools(ctx context.Context) {
	self, inJob := ctx.Value(jobCtxKey{}).(jobCtxInfo)

	poolsMu.Lock()
	if staleSweeperCancel != nil {
		staleSweeperCancel()
		staleSweeperCancel = nil
	}

	pools := []pond.Pool{
		workerPoolSearch,
		workerPoolRSS,
		workerPoolFiles,
		workerPoolMetadata,
		workerPoolIndexer,
		workerPoolIndexerRSS,
		workerPoolParse,
	}
	poolsMu.Unlock()

	done := make(chan struct{})

	go func() {
		defer close(done)
		defer logger.HandlePanic()

		for i := range pools {
			switch {
			case pools[i] == nil:
			case inJob && pools[i] == self.pool:
				pools[i].Stop()
				waitForOtherTasks(pools[i])
			default:
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
		if inJob && getjob.ID == self.id {
			return
		}

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

// waitForOtherTasks blocks until p has at most one unfinished task - the
// calling job itself. The counters are read completed/dropped first and
// submitted last so the subtraction cannot underflow.
func waitForOtherTasks(p pond.Pool) {
	for {
		finished := p.CompletedTasks() + p.DroppedTasks()
		if p.SubmittedTasks()-finished <= 1 {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
// It checks all worker pools (data, feeds, search, RSS) for active or waiting tasks and only
// clears the queue if all pools are idle. This prevents premature cleanup of pending jobs
// and ensures system stability during shutdown or maintenance operations.
func Cleanqueue() error {
	poolsMu.RLock()
	pools := map[string]pond.Pool{
		QueueData:   workerPoolFiles,
		QueueFeeds:  workerPoolMetadata,
		QueueSearch: workerPoolSearch,
		QueueRSS:    workerPoolRSS,
	}
	poolsMu.RUnlock()

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
	poolsMu.RLock()
	rc := recentJobs
	poolsMu.RUnlock()

	if rc == nil {
		return
	}

	rc.SetWithTTL(jobname, struct{}{}, 1, 10*time.Second)
	// Ristretto applies sets asynchronously - wait so a duplicate dispatched
	// immediately afterwards already sees the entry.
	rc.Wait()
}

// isRecentJob checks if a job was recently submitted (within last 10 seconds)
// to prevent duplicate job submissions and reduce system load.
func isRecentJob(jobname string) bool {
	poolsMu.RLock()
	rc := recentJobs
	poolsMu.RUnlock()

	if rc == nil {
		return false
	}

	_, ok := rc.Get(jobname)

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
// The removed jobs' names are released so they can be dispatched again; a job
// added between the scan and the delete releases its own name when its runner
// finds no queue entry.
func DeleteJobQueue(queue string, isStarted bool) {
	var removed []syncops.Job

	globalQueueSet.ForEach(func(_ uint32, job syncops.Job) {
		if job.Queue != queue {
			return
		}

		if isStarted && job.Started.IsZero() {
			return
		}

		removed = append(removed, job)
	})

	for i := range removed {
		if removed[i].CancelFunc != nil {
			removed[i].CancelFunc()
		}
	}

	syncops.QueueWorkerMapDeleteQueue(syncops.MapTypeQueue, queue, isStarted)

	for i := range removed {
		releaseJobName(removed[i].Name, removed[i].ID)
	}
}

// DeleteQueue deletes all jobs from the global queue set that match the given queue name.
// This is a convenience function that removes all jobs regardless of their execution status.
//
// Parameters:
//   - queue: Name of the queue to clear of all jobs
func DeleteQueue(queue string) {
	DeleteJobQueue(queue, false)
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
