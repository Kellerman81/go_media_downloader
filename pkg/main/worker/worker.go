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

var (
	strMsg = "msg"
	// workerPoolIndexer is a WorkerPool for executing indexer tasks.
	WorkerPoolIndexer pond.Pool

	// workerPoolParse is a WorkerPool for executing parse tasks.
	WorkerPoolParse pond.Pool

	// workerPoolSearch is a WorkerPool for executing search tasks.
	workerPoolSearch pond.Pool

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
	globalScheduleSet = syncMapUint[jobSchedule]{
		m: make(map[uint32]jobSchedule, 100),
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

	ErrNotQueued = errors.New("not queued")
	// phandler is a panic handler function.
	// phandler = pond.PanicHandler(func(p any) {
	// 	logger.LogDynamicany2StrAny("error", "Recovered from panic (dispatcher)", strMsg, logger.Stack(), strMsg, p)
	// }).
)

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
	logger.Logtype("error", 0).Any("values", keysAndValues).Str(strMsg, msg).Err(err).Msg("cron error")
}

// CreateCronWorker initializes the cron workers for data, feeds, and search.
// It configures each cron worker with the application logger, sets the timezone,
// adds error recovery and duplicate job prevention middleware,
// and enables running jobs at a per-second interval.
func CreateCronWorker() {
	loggerworker := &wrappedLogger{}
	opts := []cron.Option{cron.WithLocation(logger.GetTimeZone()), cron.WithLogger(loggerworker), cron.WithChain(cron.Recover(loggerworker), cron.SkipIfStillRunning(loggerworker)), cron.WithSeconds()}
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
	case "Data":
		return cronWorkerData
	case "Feeds":
		return cronWorkerFeeds
	case "Search":
		return cronWorkerSearch
	}
	return nil
}

func TestWorker(cfgpstr string, name string, queue string, jobname string) {
	addjob(context.Background(), cfgpstr, newuuid(), name, jobname, queue, 0)
}

// DispatchCron schedules a cron job to run the given function fn
// at the specified cron schedule cronStr.
// It adds the job to the worker queue specified by queue and gives it the name name.
// It returns any error from setting up the cron job.
func DispatchCron(cfgpstr string, cronStr string, name string, queue string, jobname string) error {
	schedulerID := newuuid()

	dc := getcron(queue)
	if dc == nil {
		return errors.New("queue not found")
	}
	cjob, err := dc.AddFunc(cronStr, func() {
		addjob(context.Background(), cfgpstr, newuuid(), name, jobname, queue, schedulerID)
	})
	if err != nil {
		return err
	}
	dcentry := dc.Entry(cjob)
	globalScheduleSet.Add(schedulerID, jobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "cron",
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
func addjob(_ context.Context, cfgpstr string, id uint32, name string, jobname string, queue string, schedulerID uint32) {
	if jobname == "" {
		logger.LogDynamicany1String("error", "empty func", logger.StrJob, name)
		return
	}

	if checkQueue(name) {
		logger.LogDynamicany1String("error", "already queued", logger.StrJob, name)
		return
	}

	workpool, added := getpooladded(queue)
	capa := workpool.MaxConcurrency()
	if capa > 0 && uint64(capa) <= workpool.WaitingTasks() {
		logger.Logtype("error", 0).Str("queue", queue).Str(logger.StrJob, name).Msg("queue limit reached")
		return
	}
	for idx := range 11 {
		// if logger.CheckContextEnded(ctx) != nil {
		// 	return
		// }
		if !logger.TimeAfter(added, time.Now().Add(time.Millisecond*200)) {
			break
		}
		time.Sleep(time.Millisecond * 100)
		if idx == 10 {
			logger.LogDynamicany1String("error", "queue recently added", logger.StrJob, name)
			return
		}
	}
	switch queue {
	case "Data":
		lastaddeddata = time.Now()
	case "Feeds":
		lastaddedfeeds = time.Now()
	case "Search":
		lastaddedsearch = time.Now()
	default:
		lastadded = time.Now()
	}

	if workpool == nil {
		return
	}
	if cfgpstr == "" {
		if config.SettingsGeneral.Jobs[jobname] == nil {
			return
		}
	} else {
		if config.SettingsMedia[cfgpstr] == nil {
			return
		} else if config.SettingsMedia[cfgpstr].Jobs[jobname] == nil {
			return
		}
	}
	if workpool.SubmittedTasks() == workpool.CompletedTasks() {
		DeleteQueueRunning(queue)
	} else if workpool.SubmittedTasks()-workpool.WaitingTasks() == workpool.CompletedTasks() {
		DeleteQueueRunning(queue)
	}
	globalQueueSet.Add(id, Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		Queue:       queue,
		ID:          id,
		JobName:     jobname,
		Cfgpstr:     cfgpstr,
		SchedulerID: schedulerID,
	})
	_, ok := workpool.TrySubmit(runjobcron(id))
	if !ok {
		globalQueueSet.Delete(id)
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
		defer logger.HandlePanic()
		s := globalQueueSet.GetVal(id)
		defer globalQueueSet.Delete(id)
		SetScheduleStarted(s.SchedulerID)
		defer SetScheduleEnded(s.SchedulerID)
		s.Started = logger.TimeGetNow()
		globalQueueSet.UpdateVal(id, s)
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
	if id == 0 {
		return
	}
	globalQueueSet.Delete(id)
}

// getpooladded returns the appropriate worker pool and the last time a job was added to that pool based on the provided queue name.
// The function uses a switch statement to determine the correct worker pool and last added time for the given queue.
// If the queue name is not recognized, the function returns nil and the lastadded time.
func getpooladded(queue string) (pond.Pool, time.Time) {
	switch queue {
	case "Data":
		return workerPoolFiles, lastaddeddata
	case "Feeds":
		return workerPoolMetadata, lastaddedfeeds
	case "Search":
		return workerPoolSearch, lastaddedsearch
	default:
		return nil, lastadded
	}
}

// newuuid generates a new UUID string to use as a unique ID.
func newuuid() uint32 {
	return uuid.New().ID()
}

// DispatchEvery dispatches a job to run on a regular time interval.
// It takes in the interval duration, job name, queue, and function to run.
// It returns any error from setting up the ticker.
func DispatchEvery(cfgpstr string, interval time.Duration, name string, queue string, jobname string) error {
	schedulerID := newuuid()
	t := time.NewTicker(interval)

	go func() {
		for range t.C {
			addjob(context.Background(), cfgpstr, newuuid(), name, jobname, queue, schedulerID)
		}
	}()
	globalScheduleSet.Add(schedulerID, jobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "interval",
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

	workpool, added := getpooladded(queue)

	if workpool == nil {
		return ErrNotQueued
	}
	capa := workpool.MaxConcurrency()
	if capa > 0 && uint64(capa) <= workpool.WaitingTasks() {
		logger.Logtype("error", 0).Str("queue", queue).Str(logger.StrJob, name).Msg("queue limit reached")
		return ErrNotQueued
	}
	ctx := context.Background()
	for idx := range 11 {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}
		if !logger.TimeAfter(added, time.Now().Add(time.Millisecond*200)) {
			break
		}
		time.Sleep(time.Millisecond * 100)
		if idx == 10 {
			logger.LogDynamicany1String("error", "queue recently added", logger.StrJob, name)
			return ErrNotQueued
		}
	}
	ctx.Done()
	switch queue {
	case "Data":
		lastaddeddata = time.Now()
	case "Feeds":
		lastaddedfeeds = time.Now()
	case "Search":
		lastaddedsearch = time.Now()
	default:
		lastadded = time.Now()
	}

	id := newuuid()
	globalQueueSet.Add(id, Job{
		Added:       logger.TimeGetNow(),
		Name:        name,
		JobName:     name,
		Queue:       queue,
		ID:          id,
		SchedulerID: newuuid(),
	})
	_, ok := workpool.TrySubmit(runjob(id, fn))
	// if workpool.Go(runjob(id, fn)) != nil {
	if !ok {
		globalQueueSet.Delete(id)
		return ErrNotQueued
	}
	return nil
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
		defer logger.HandlePanic()
		s := globalQueueSet.GetVal(id)
		defer globalQueueSet.Delete(id)

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
func InitWorkerPools(workersearch int, workerfiles int, workermeta int) {
	if workersearch == 0 {
		workersearch = 1
	}
	if workerfiles == 0 {
		workerfiles = 1
	}
	if workermeta == 0 {
		workermeta = 1
	}
	workerPoolSearch = pond.NewPool(workersearch)
	workerPoolFiles = pond.NewPool(workerfiles)
	workerPoolMetadata = pond.NewPool(workermeta)
	WorkerPoolIndexer = pond.NewPool(workersearch)
	WorkerPoolParse = pond.NewPool(workerfiles)
}

// CloseWorkerPools stops all worker pools and waits for workers
// to finish current jobs before returning. Waits up to 2 minutes
// per pool before timing out.
func CloseWorkerPools() {
	workerPoolSearch.Stop()
	workerPoolFiles.Stop()
	workerPoolMetadata.Stop()
	WorkerPoolIndexer.Stop()
	WorkerPoolParse.Stop()
}

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
type jobSchedule struct {
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

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
func Cleanqueue() {
	if workerPoolFiles.CompletedTasks() == workerPoolFiles.SubmittedTasks() {
		DeleteQueue("Data")
	}
	if workerPoolMetadata.CompletedTasks() == workerPoolMetadata.SubmittedTasks() {
		DeleteQueue("Feeds")
	}
	if workerPoolSearch.CompletedTasks() == workerPoolSearch.SubmittedTasks() {
		DeleteQueue("Search")
	}
}

// GetQueues returns a map of all currently configured queues, keyed by the queue name.
func GetQueues() map[uint32]Job {
	return globalQueueSet.GetMap()
}

// GetSchedules returns a map of all currently configured schedules,
// keyed by the job name.
func GetSchedules() map[uint32]jobSchedule {
	return globalScheduleSet.GetMap()
}

// checkQueue checks if a job with the given name is currently running in any
// of the global queues. It handles checking alternate name formats that may
// have been used when adding the job. Returns true if the job is found to be
// running in a queue, false otherwise.
func checkQueue(jobname string) bool {
	var alt1, alt2, alt3 string
	idx := strings.LastIndexByte(jobname, '_')
	var pre string
	if idx == -1 || idx == 0 || idx == len(jobname) {
		pre = jobname
	} else {
		pre = jobname[:idx]
	}
	switch pre {
	case "searchmissinginc":
		alt1 = "searchmissinginctitle_"
		alt2 = "searchmissingfull_"
		alt3 = "searchmissingfulltitle_"
	case "searchmissinginctitle":
		alt1 = "searchmissinginc_"
		alt2 = "searchmissingfull_"
		alt3 = "searchmissingfulltitle_"
	case "searchmissingfull":
		alt1 = "searchmissinginctitle_"
		alt2 = "searchmissinginc_"
		alt3 = "searchmissingfulltitle_"
	case "searchmissingfulltitle":
		alt1 = "searchmissinginctitle_"
		alt2 = "searchmissingfull_"
		alt3 = "searchmissinginc_"
	case "searchupgradeinc":
		alt1 = "searchupgradeinctitle_"
		alt2 = "searchupgradefull_"
		alt3 = "searchupgradefulltitle_"
	case "searchupgradeinctitle":
		alt1 = "searchupgradeinc_"
		alt2 = "searchupgradefull_"
		alt3 = "searchupgradefulltitle_"
	case "searchupgradefull":
		alt1 = "searchupgradeinctitle_"
		alt2 = "searchupgradeinc_"
		alt3 = "searchupgradefulltitle_"
	case "searchupgradefulltitle":
		alt1 = "searchupgradeinctitle_"
		alt2 = "searchupgradefull_"
		alt3 = "searchupgradeinc_"
	default:
		return globalQueueSet.ForFuncKey(func(_ uint32, val Job) bool {
			return val.Name == jobname && val.Started.IsZero()
		})
	}

	end := jobname[idx+1:]

	return globalQueueSet.ForFuncKey(func(_ uint32, val Job) bool {
		if val.Name == jobname && val.Started.IsZero() {
			return true
		}
		if !strings.Contains(val.Name, end) {
			return false
		}
		if val.Name == (alt1+end) && val.Started.IsZero() {
			return true
		}
		if val.Name == (alt2+end) && val.Started.IsZero() {
			return true
		}
		if val.Name == (alt3+end) && val.Started.IsZero() {
			return true
		}
		return false
	})
}

type syncMapUint[T any] struct {
	m  map[uint32]T
	mu sync.Mutex
}

// DeleteFuncKey deletes all entries in the SyncMap for which the provided function
// returns true, using both the key and value as arguments.
func (s *syncMapUint[T]) ForFuncKey(fn func(uint32, T) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, v := range s.m {
		if fn(key, v) {
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

	if _, ok := s.m[id]; !ok {
		return
	}
	logger.Logtype("warn", 1).Uint32("id", id).Msg("Failed to delete job from queue")
}

// DeleteFunc deletes all elements from the SyncMap that match the given predicate function fn.
// The method acquires a write lock on the SyncMap before iterating through the map and deleting
// any elements that satisfy the predicate. The lock is released before the method returns.
func (s *syncMapUint[T]) DeleteFunc(fn func(T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, v := range s.m {
		if fn(v) {
			delete(s.m, key)
		}
	}
}

// DeleteQueue deletes all jobs from the global queue set that match the given queue name.
func DeleteQueue(queue string) {
	globalQueueSet.DeleteFunc(func(t Job) bool {
		return t.Queue == queue
	})
}

// DeleteQueueRunning deletes all jobs from the global queue set that match the given queue name and have a non-zero start time.
func DeleteQueueRunning(queue string) {
	globalQueueSet.DeleteFunc(func(t Job) bool {
		return t.Queue == queue && !t.Started.IsZero()
	})
}
