package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
	WorkerParse    StatsDetail            `json:"WorkerParse"`
	WorkerSearch   StatsDetail            `json:"WorkerSearch"`
	WorkerRSS      StatsDetail            `json:"WorkerRSS"`
	WorkerFiles    StatsDetail            `json:"WorkerFiles"`
	WorkerMeta     StatsDetail            `json:"WorkerMeta"`
	WorkerIndex    StatsDetail            `json:"WorkerIndex"`
	WorkerIndexRSS StatsDetail            `json:"WorkerIndexFSS"`
	ListQueue      map[uint32]Job         `json:"ListQueue"`
	ListSchedule   map[uint32]JobSchedule `json:"ListSchedule"`
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
	globalScheduleSet = syncMapUint[JobSchedule]{
		m: make(map[uint32]JobSchedule, 100),
	}

	// globalQueueSet is a sync.Map to store dispatcherQueue objects.
	globalQueueSet = syncMapUint[Job]{
		m: make(map[uint32]Job, 1000),
	}

	// lastadded is a timestamp for tracking last added time.
	lastadded = time.Now().Add(time.Second - 1)

	// lastaddeddata is a timestamp for tracking last added data time.
	lastaddeddata = time.Now().Add(time.Second - 1)

	// lastaddedfeeds is a timestamp for tracking last added feeds time.
	lastaddedfeeds = time.Now().Add(time.Second - 1)

	// lastaddedsearch is a timestamp for tracking last search time.
	lastaddedsearch = time.Now().Add(time.Second - 1)

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

// Job represents a job to be run by a worker pool.
type Job struct {
	// Queue is the name of the queue this job belongs to
	Queue   string
	JobName string `json:"-"`
	Cfgpstr string `json:"-"`
	// Name is a descriptive name for this job
	Name string
	// Added is the time this job was added to the queue
	Added time.Time
	// Started is the time this job was started by a worker
	Started time.Time
	// ID is a unique identifier for this job
	ID uint32
	// SchedulerID is the ID of the scheduler that added this job
	SchedulerID uint32
	// Run is the function to execute for this job
	// Run func(uint32) `json:"-"`
	// CronJob is the cron job instance if this is a recurring cron job
	// CronJob cron.Job `json:"-"`
}

// jobSchedule represents a scheduled job.
type JobSchedule struct {
	// JobName is the name of the job
	JobName string
	// ScheduleTyp is the type of schedule (cron, interval, etc)
	ScheduleTyp string
	// ScheduleString is the schedule string (cron expression, interval, etc)
	ScheduleString string
	// LastRun is the last time this job ran
	LastRun time.Time
	// NextRun is the next scheduled run time
	NextRun time.Time
	// Interval is the interval duration if schedule type is interval
	Interval time.Duration
	// CronID is the cron scheduler ID if scheduled as cron job
	CronID cron.EntryID
	// JobID is the unique ID of the job
	JobID uint32
	// ID is the unique ID for this schedule
	ID uint32
	// CronSchedule is the parsed cron.Schedule if type is cron
	CronSchedule cron.Schedule
	// IsRunning indicates if the job is currently running
	IsRunning bool
}

type syncMapUint[T any] struct {
	m  map[uint32]T
	mu sync.Mutex
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

// SetScheduleStarted sets the IsRunning field of the jobSchedule with the given ID to true,
// updates the LastRun field to the current time, and sets the NextRun field based on the
// schedule type (cron or interval). This function is used to mark a jobSchedule as running. It Locks.
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
	globalScheduleSet.UpdateVal(id, s)
}

// SetScheduleEnded sets the IsRunning field of the jobSchedule with the given ID to false.
// It locks the mutex, finds the index of the jobSchedule in the globalScheduleSet, and sets the IsRunning field to false.
// This function is used to mark a jobSchedule as no longer running.
func SetScheduleEnded(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}
	s := globalScheduleSet.GetVal(id)
	if !s.IsRunning {
		return
	}
	s.IsRunning = false
	globalScheduleSet.UpdateVal(id, s)
}

// CreateCronWorker initializes the cron workers for data, feeds, and search.
// It configures each cron worker with the application logger, sets the timezone,
// adds error recovery and duplicate job prevention middleware,
// and enables running jobs at a per-second interval.
func CreateCronWorker() {
	loggerworker := wrappedLogger{}
	opts := []cron.Option{
		cron.WithLocation(logger.GetTimeZone()),
		cron.WithLogger(&loggerworker),
		cron.WithChain(cron.Recover(&loggerworker), cron.SkipIfStillRunning(&loggerworker)),
		cron.WithSeconds(),
	}
	cronWorkerData = cron.New(opts...)
	cronWorkerFeeds = cron.New(opts...)
	cronWorkerSearch = cron.New(opts...)
}

// StartCronWorker starts all cron workers.
func StartCronWorker() {
	cronWorkerData.Start()
	cronWorkerFeeds.Start()
	cronWorkerSearch.Start()
}

// StopCronWorker stops all cron workers.
func StopCronWorker() {
	cronWorkerData.Stop()
	cronWorkerFeeds.Stop()
	cronWorkerSearch.Stop()
}

// getcron returns the appropriate cron.Cron instance for the given queue.
// It checks the queue name and returns the corresponding cron worker instance.
// If the queue name is not recognized, it returns nil.
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

func newUUID() uint32 {
	return uuid.New().ID()
}

func validateJobConfig(cfgpstr, jobname string) bool {
	if cfgpstr == "" {
		return config.SettingsGeneral.Jobs[jobname] != nil
	}

	cfg := config.SettingsMedia[cfgpstr]
	return cfg != nil && cfg.Jobs[jobname] != nil
}

func TestWorker(cfgpstr string, name string, queue string, jobname string) {
	addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, 0)
}

// DispatchCron schedules a cron job to run the given function fn
// at the specified cron schedule cronStr.
// It adds the job to the worker queue specified by queue and gives it the name name.
// It returns any error from setting up the cron job.
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
	globalScheduleSet.Add(schedulerID, JobSchedule{
		JobName:        name,
		JobID:          newUUID(),
		ID:             schedulerID,
		ScheduleTyp:    ScheduleTypeCron,
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dcentry.Next,
		CronSchedule:   dcentry.Schedule,
		CronID:         cjob,
	})
	return nil
}

// addjob adds a new job with the given name, queue, function, and scheduler ID
// to the appropriate worker pool for processing. It checks if the job is already
// queued or if the queue is full before submitting.
func addjob(
	_ context.Context,
	cfgpstr string,
	id uint32,
	name string,
	jobname string,
	queue string,
	schedulerID uint32,
) {
	if jobname == "" {
		logger.LogDynamicany1String("error", "empty func", logger.StrJob, name)
		return
	}

	if checkQueue(name) {
		logger.LogDynamicany1String("error", "already queued", logger.StrJob, name)
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

	globalQueueSet.Add(id, Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		Queue:       queue,
		ID:          id,
		JobName:     jobname,
		Cfgpstr:     cfgpstr,
		SchedulerID: schedulerID,
	})

	if _, ok := workpool.TrySubmit(runjobcron(id)); !ok {
		logger.LogDynamicany1String("error", "not queued", logger.StrJob, name)
		globalQueueSet.Delete(id)
	}
}

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

func waitForQueueAvailability(added time.Time, name string) error {
	for idx := 0; idx <= maxQueueRetries; idx++ {
		if !logger.TimeAfter(added, time.Now().Add(queueCheckInterval)) {
			break
		}
		time.Sleep(queueCheckDelay)
		if idx == maxQueueRetries {
			logger.LogDynamicany1String("error", "queue recently added", logger.StrJob, name)
			return ErrNotQueued
		}
	}
	return nil
}

func cleanupCompletedJobs(workpool pond.Pool, queue string) {
	if workpool.SubmittedTasks() == workpool.CompletedTasks() ||
		workpool.SubmittedTasks()-workpool.WaitingTasks() == workpool.CompletedTasks() {
		DeleteQueueRunning(queue)
	}
}

// runjobcron is a closure function that runs a scheduled job. It checks if the job is still in the global queue set,
// retrieves the job details, sets the job as started, runs the job, and then deletes the job from the global queue set.
// If the job's configuration is not found, it logs an error message.
func runjobcron(id uint32) func() {
	return func() {
		if !globalQueueSet.Check(id) {
			return
		}

		defer func() {
			logger.HandlePanic()
			globalQueueSet.Delete(id)
		}()

		s := globalQueueSet.GetVal(id)
		SetScheduleStarted(s.SchedulerID)
		defer SetScheduleEnded(s.SchedulerID)

		s.Started = logger.TimeGetNow()
		globalQueueSet.UpdateVal(id, s)

		executeJob(s)
		if s.Cfgpstr == "" {
			if config.SettingsGeneral.Jobs[s.JobName] != nil {
				config.SettingsGeneral.Jobs[s.JobName](id)
				globalQueueSet.Delete(id)
			} else {
				logger.LogDynamicany2Str("error", "Cron Job not found", "job", s.JobName, "cfgp", s.Cfgpstr)
			}
		} else {
			if config.SettingsMedia[s.Cfgpstr] == nil {
				logger.LogDynamicany2Str("error", "Cron Job Config not found", "job", s.JobName, "cfgp", s.Cfgpstr)
			} else {
				if config.SettingsMedia[s.Cfgpstr].Jobs[s.JobName] != nil {
					config.SettingsMedia[s.Cfgpstr].Jobs[s.JobName](id)
					globalQueueSet.Delete(id)
				} else {
					logger.LogDynamicany2Str("error", "Cron Job not found", "job", s.JobName, "cfgp", s.Cfgpstr)
				}
			}
		}
	}
}

// RemoveQueueEntry removes the queue entry with the given ID from the global queue set.
// If the provided ID is 0, the function returns without taking any action.
func RemoveQueueEntry(id uint32) {
	if id != 0 {
		globalQueueSet.Delete(id)
	}
}

// DispatchEvery dispatches a job to run on a regular time interval.
// It takes in the interval duration, job name, queue, and function to run.
// It returns any error from setting up the ticker.
func DispatchEvery(
	cfgpstr string,
	interval time.Duration,
	name string,
	queue string,
	jobname string,
) error {
	schedulerID := newUUID()
	t := time.NewTicker(interval)

	go func() {
		for range t.C {
			addjob(context.Background(), cfgpstr, newUUID(), name, jobname, queue, schedulerID)
		}
	}()
	globalScheduleSet.Add(schedulerID, JobSchedule{
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

// Dispatch adds a new job with the given name, function, and queue to the
// worker pool. It generates a new UUID to associate with the job.
func Dispatch(name string, fn func(uint32), queue string) error {
	if fn == nil {
		logger.LogDynamicany1String("error", "empty func", logger.StrJob, name)
		return ErrNotQueued
	}

	if checkQueue(name) {
		logger.LogDynamicany1String("error", "already queued", logger.StrJob, name)
		return ErrNotQueued
	}

	workpool, added := getPoolAndLastAdded(queue)

	if workpool == nil {
		return ErrInvalidQueue
	}

	if err := checkQueueCapacity(workpool.QueueSize(), workpool.WaitingTasks(), queue, name); err != nil {
		return err
	}

	if err := waitForQueueAvailability(added, name); err != nil {
		return err
	}

	updateLastAdded(queue)

	id := newUUID()
	globalQueueSet.Add(id, Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		JobName:     name,
		Queue:       queue,
		ID:          id,
		SchedulerID: newUUID(),
	})
	if _, ok := workpool.TrySubmit(runjob(id, fn)); !ok {
		logger.LogDynamicany1String("error", "not queued", logger.StrJob, name)
		globalQueueSet.Delete(id)
		return ErrNotQueued
	}
	return nil
}

func executeJob(s Job) {
	if s.Cfgpstr == "" {
		if jobFunc := config.SettingsGeneral.Jobs[s.JobName]; jobFunc != nil {
			jobFunc(s.ID)
		} else {
			logger.LogDynamicany2Str("error", "Cron Job not found", "job", s.JobName, "cfgp", s.Cfgpstr)
		}
	} else {
		cfg := config.SettingsMedia[s.Cfgpstr]
		if cfg == nil {
			logger.LogDynamicany2Str("error", "Cron Job Config not found", "job", s.JobName, "cfgp", s.Cfgpstr)
			return
		}

		if jobFunc := cfg.Jobs[s.JobName]; jobFunc != nil {
			jobFunc(s.ID)
		} else {
			logger.LogDynamicany2Str("error", "Cron Job not found", "job", s.JobName, "cfgp", s.Cfgpstr)
		}
	}
}

// runjob is a closure that wraps a job function and handles the job lifecycle.
// It checks if the job is still in the global queue, retrieves the job details,
// updates the job's started time, and then calls the provided job function.
// After the job function completes, it removes the job from the global queue.
func runjob(id uint32, fn func(uint32)) func() {
	return func() {
		if !globalQueueSet.Check(id) {
			return
		}

		defer func() {
			logger.HandlePanic()
			globalQueueSet.Delete(id)
		}()

		s := globalQueueSet.GetVal(id)
		s.Started = logger.TimeGetNow()
		globalQueueSet.UpdateVal(id, s)
		fn(id)
	}
}

// InitWorkerPools initializes the worker pools for indexing, parsing,
// searching, downloading files, and updating metadata. It takes in the
// desired number of workers for each pool and defaults them to 1 if 0 is
// passed in. It configures the pools with balanced strategy and error
// handling function.
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

// CloseWorkerPools stops all worker pools and waits for workers
// to finish current jobs before returning. Waits up to 2 minutes
// per pool before timing out.
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
}

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
func Cleanqueue() {
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
}

// GetQueues returns a map of all currently configured queues, keyed by the queue name.
func GetQueues() map[uint32]Job {
	return globalQueueSet.GetMap()
}

// GetSchedules returns a map of all currently configured schedules,
// keyed by the job name.
func GetSchedules() map[uint32]JobSchedule {
	return globalScheduleSet.GetMap()
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

// checkQueue checks if a job with the given name is currently running in any
// of the global queues. It handles checking alternate name formats that may
// have been used when adding the job. Returns true if the job is found to be
// running in a queue, false otherwise.
func checkQueue(jobname string) bool {
	idx := strings.LastIndexByte(jobname, '_')
	if idx <= 0 || idx >= len(jobname)-1 {
		return globalQueueSet.ForFuncKey(func(_ uint32, val *Job) bool {
			return val.Name == jobname && val.Started.IsZero()
		})
	}

	prefix := jobname[:idx]
	suffix := jobname[idx+1:]

	alternatives, hasAlternatives := jobAlternatives[prefix]
	if !hasAlternatives {
		return globalQueueSet.ForFuncKey(func(_ uint32, val *Job) bool {
			return val.Name == jobname && val.Started.IsZero()
		})
	}

	return globalQueueSet.ForFuncKey(func(_ uint32, val *Job) bool {
		if val.Started.IsZero() {
			if val.Name == jobname {
				return true
			}

			if strings.HasSuffix(val.Name, suffix) {
				for _, alt := range alternatives {
					if val.Name == alt+suffix {
						return true
					}
				}
			}
		}
		return false
	})
}

// DeleteFuncKey deletes all entries in the SyncMap for which the provided function
// returns true, using both the key and value as arguments.
func (s *syncMapUint[T]) ForFuncKey(fn func(uint32, *T) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, v := range s.m {
		if fn(key, &v) {
			return true
		}
	}
	return false
}

// Check returns true if the given key exists in the SyncMap, false otherwise.
// The method acquires a read lock on the SyncMap before checking for the key,
// and releases the lock before returning.
func (s *syncMapUint[T]) Check(key uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[key]
	return ok
}

// Add adds the given key-value pair to the SyncMap. The method acquires a write lock
// on the SyncMap before adding the new entry, and releases the lock before returning.
func (s *syncMapUint[T]) Add(key uint32, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// UpdateVal updates the value associated with the given key in the SyncMap.
// The method acquires a write lock on the SyncMap before updating the value,
// and releases the lock before returning.
func (s *syncMapUint[T]) UpdateVal(key uint32, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// GetVal returns the value associated with the given key in the SyncMap.
// The method acquires a read lock on the SyncMap before retrieving the value,
// and releases the lock before returning.
func (s *syncMapUint[T]) GetVal(key uint32) T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key]
}

// GetMap returns a copy of the underlying map stored in the syncMapUint.
// The method acquires a read lock on the SyncMap before returning the map,
// and releases the lock before returning.
func (s *syncMapUint[T]) GetMap() map[uint32]T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m
}

// Delete removes the entry with the given id from the SyncMap. If the entry does not exist,
// the method simply returns. If the entry is successfully deleted but the key still exists
// in the map, a warning log is emitted.
func (s *syncMapUint[T]) Delete(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; !ok {
		return
	}
	delete(s.m, id)

	if _, ok := s.m[id]; ok {
		logger.Logtype("warn", 1).Uint32("id", id).Msg("Failed to delete job from queue")
	}
}

// DeleteFunc deletes all elements from the SyncMap that match the given predicate function fn.
// The method acquires a write lock on the SyncMap before iterating through the map and deleting
// any elements that satisfy the predicate. The lock is released before the method returns.
func DeleteJobQueue(queue string, isStarted bool) {
	globalQueueSet.mu.Lock()
	defer globalQueueSet.mu.Unlock()
	for key := range globalQueueSet.m {
		if globalQueueSet.m[key].Queue == queue {
			if isStarted {
				if !globalQueueSet.m[key].Started.IsZero() {
					delete(globalQueueSet.m, key)
				}
			} else {
				delete(globalQueueSet.m, key)
			}
		}
	}
}

// DeleteQueue deletes all jobs from the global queue set that match the given queue name.
func DeleteQueue(queue string) {
	DeleteJobQueue(queue, false)
}

// DeleteQueueRunning deletes all jobs from the global queue set that match the given queue name and have a non-zero start time.
func DeleteQueueRunning(queue string) {
	DeleteJobQueue(queue, true)
}
