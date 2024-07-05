package worker

import (
	"errors"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/alitto/pond"
	"github.com/google/uuid"

	"github.com/robfig/cron/v3"
)

var (
	strMsg = "msg"
	// workerPoolIndexer is a WorkerPool for executing indexer tasks
	//workerPoolIndexer *pond.WorkerPool

	// workerPoolParse is a WorkerPool for executing parse tasks
	//workerPoolParse *pond.WorkerPool

	// workerPoolSearch is a WorkerPool for executing search tasks
	workerPoolSearch *pond.WorkerPool

	// workerPoolFiles is a WorkerPool for executing file tasks
	workerPoolFiles *pond.WorkerPool

	// workerPoolMetadata is a WorkerPool for executing metadata tasks
	workerPoolMetadata *pond.WorkerPool

	// cronWorkerData is a Cron instance for scheduling data worker jobs
	cronWorkerData *cron.Cron

	// cronWorkerFeeds is a Cron instance for scheduling feeds worker jobs
	cronWorkerFeeds *cron.Cron

	// cronWorkerSearch is a Cron instance for scheduling search worker jobs
	cronWorkerSearch *cron.Cron

	// globalScheduleSet is a sync.Map to store jobSchedule objects
	globalScheduleSet = logger.NewSynchedMapuint32[jobSchedule](100)

	// globalQueueSet is a sync.Map to store dispatcherQueue objects
	globalQueueSet = logger.NewSynchedMapuint32[dispatcherQueue](100)

	// lastadded is a timestamp for tracking last added time
	lastadded = time.Now().Add(time.Second - 1)

	// lastaddeddata is a timestamp for tracking last added data time
	lastaddeddata = time.Now().Add(time.Second - 1)

	// lastaddedfeeds is a timestamp for tracking last added feeds time
	lastaddedfeeds = time.Now().Add(time.Second - 1)

	// lastaddedsearch is a timestamp for tracking last search time
	lastaddedsearch = time.Now().Add(time.Second - 1)

	// phandler is a panic handler function
	phandler = pond.PanicHandler(func(p any) {
		logger.LogDynamicany("error", "Recovered from panic (dispatcher)", &strMsg, logger.Stack(), "vap", p)
	})
)

// SetScheduleStarted sets the IsRunning field of the jobSchedule with the given ID to true,
// updates the LastRun field to the current time, and sets the NextRun field based on the
// schedule type (cron or interval). This function is used to mark a jobSchedule as running. It Locks
func SetScheduleStarted(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}
	s := globalScheduleSet.Get(id)
	s.IsRunning = true
	s.LastRun = logger.TimeGetNow()
	if s.ScheduleTyp == "cron" {
		s.NextRun = s.CronSchedule.Next(logger.TimeGetNow())
	} else {
		s.NextRun = logger.TimeGetNow().Add(s.Interval)
	}
	globalScheduleSet.Set(id, s)
}

// SetScheduleEnded sets the IsRunning field of the jobSchedule with the given ID to false.
// It locks the mutex, finds the index of the jobSchedule in the globalScheduleSet, and sets the IsRunning field to false.
// This function is used to mark a jobSchedule as no longer running.
func SetScheduleEnded(id uint32) {
	if !globalScheduleSet.Check(id) {
		return
	}
	s := globalScheduleSet.Get(id)
	s.IsRunning = false
	globalScheduleSet.Set(id, s)
}

// cleanupqueue removes any dispatcher queues from the globalQueueSet that have an empty Name field and match the provided queue name.
// This function is used to clean up dispatcher queues that are no longer in use.
func cleanupqueue(queue string) {
	toremove := globalQueueSet.FuncMap(func(value dispatcherQueue) bool {
		if value.Name == "" && value.Queue.Queue == queue {
			return true
		}
		return false
	})
	if len(toremove) == 0 {
		return
	}
	logger.LogDynamicany("debug", "Dispatcher clean empty name", "queue", queue)
	for idx := range toremove {
		globalQueueSet.Delete(toremove[idx])
	}
}

// RemoveQueue removes a dispatcher queue from the globalQueueSet by its ID. It locks the mutex, finds the index of the queue in the globalQueueSet, sets the Name field to an empty string, and then deletes the queue from the slice. This function is used to clean up dispatcher queues that are no longer in use.
func RemoveQueue(id uint32) {
	if !globalQueueSet.Check(id) {
		return
	}
	s := globalQueueSet.Get(id)
	s.Name = ""
	globalQueueSet.Set(id, s)
	globalQueueSet.Delete(id)
}

// QueueSetStarted sets the Started field of the dispatcher queue with the given ID to the current time.
// It returns the SchedulerID of the dispatcher queue if it is found, or -1 if the queue is not found.
// This function is used to track when a dispatcher queue was started. It Locks
func QueueSetStarted(id uint32) int32 {
	if !globalQueueSet.Check(id) {
		return -1
	}
	s := globalQueueSet.Get(id)
	s.Queue.Started = logger.TimeGetNow()
	globalQueueSet.Set(id, s)
	return int32(s.Queue.SchedulerID)
}

// QueueRun runs the dispatcher queue with the given ID. It locks the mutex, finds the index of the queue in the globalQueueSet, and then runs the queue. If the queue is not found, it returns without doing anything. After running the queue, it defers the removal of the queue from the globalQueueSet.
func QueueRun(id uint32) {
	if !globalQueueSet.Check(id) {
		return
	}
	defer RemoveQueue(id)
	s := globalQueueSet.Get(id)
	s.Queue.Run()
}

type wrappedLogger struct {
	//cron.Logger
}

// Info logs an informational message to the wrapped logger.
// The message and key/value pairs are passed through to the wrapped
// zerolog Logger's Info method.
func (*wrappedLogger) Info(_ string, _ ...any) {
	//wl.logger.Info().Any("values", keysAndValues).Str("msg", msg).Msg("cron")
}

// Error logs an error message with additional key-value pairs to the wrapped logger.
// It takes in an error, a message string, and any number of key-value pairs.
func (*wrappedLogger) Error(err error, msg string, keysAndValues ...any) {
	logger.LogDynamicany("error", "cron error", "values", &keysAndValues, &strMsg, &msg, err)
	//wl.logger.Error().Err(err).Any("values", keysAndValues).Str("msg", msg).Msg("cron")
}

// CreateCronWorker initializes the cron workers for data, feeds, and search.
// It configures each cron worker with the application logger, sets the timezone,
// adds error recovery and duplicate job prevention middleware,
// and enables running jobs at a per-second interval.
func CreateCronWorker() {
	loggerworker := &wrappedLogger{}
	var opts []cron.Option
	opts = append(opts, cron.WithLocation(logger.GetTimeZone()), cron.WithLogger(loggerworker), cron.WithChain(cron.Recover(loggerworker), cron.SkipIfStillRunning(loggerworker)), cron.WithSeconds())
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

// getcronstuff returns the cron.Cron instance for the given queue name.
// It returns:
//   - cronWorkerData for "Data"
//   - cronWorkerFeeds for "Feeds"
//   - cronWorkerSearch for "Search"
//   - nil if the name does not match
func getcronstuff(str string) *cron.Cron {
	switch str {
	case "Data":
		return cronWorkerData
	case "Feeds":
		return cronWorkerFeeds
	case "Search":
		return cronWorkerSearch
	}
	return nil
}

// checklastadded checks if a job was recently added for the given queue
// to rate limit job submission. It returns true if a job can be added,
// false if one was added too recently.
func checklastadded(qu string) bool {
	var added time.Time
	switch qu {
	case "Data":
		added = lastaddeddata
	case "Feeds":
		added = lastaddedfeeds
	case "Search":
		added = lastaddedsearch
	default:
		added = lastadded
	}
	for range 11 {
		if added.After(time.Now().Add(time.Millisecond * 200)) {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		switch qu {
		case "Data":
			lastaddeddata = time.Now()
		case "Feeds":
			lastaddedfeeds = time.Now()
		case "Search":
			lastaddedsearch = time.Now()
		default:
			lastadded = time.Now()
		}
		return true
	}
	return false
}

// DispatchCron schedules a cron job to run the given function fn
// at the specified cron schedule cronStr.
// It adds the job to the worker queue specified by queue and gives it the name name.
// It returns any error from setting up the cron job.
func DispatchCron(cronStr string, name string, queue string, fn func()) error {
	schedulerID := newuuid()

	dc := getcronstuff(queue)
	cjob, err := dc.AddFunc(cronStr, func() { addjob(name, queue, fn, schedulerID) })
	if err != nil {
		return err
	}
	dcentry := dc.Entry(cjob)
	globalScheduleSet.Set(schedulerID, jobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dcentry.Next,
		CronSchedule:   dcentry.Schedule,
		CronID:         cjob})
	return nil
}

// addjob adds a new job with the given name, queue, function, and scheduler ID
// to the appropriate worker pool for processing. It checks if the job is already
// queued or if the queue is full before submitting.
func addjob(name string, queue string, fn func(), schedulerID uint32) {
	id, err := checkaddjob(name, queue, fn, schedulerID)
	if err != nil {
		logger.LogDynamicany("error", err.Error(), &logger.StrJob, &name)
		return
	}
	var workpool *pond.WorkerPool
	switch queue {
	case "Data":
		workpool = workerPoolFiles
	case "Feeds":
		workpool = workerPoolMetadata
	case "Search":
		workpool = workerPoolSearch
	}
	if workpool.MaxCapacity() <= int(workpool.WaitingTasks()) {
		logger.LogDynamicany("error", "queue limit reached", &logger.StrJob, name, "queue", queue)
		RemoveQueue(id)
		return
	}
	addjobrun(id, queue)
}

func addjobrun(id uint32, queue string) {
	var workpool *pond.WorkerPool
	switch queue {
	case "Data":
		workpool = workerPoolFiles
	case "Feeds":
		workpool = workerPoolMetadata
	case "Search":
		workpool = workerPoolSearch
	}
	if !workpool.TrySubmit(func() {
		defer RemoveQueue(id)
		sid := QueueSetStarted(id)
		if sid != -1 {
			SetScheduleStarted(uint32(sid))
		}
		QueueRun(id)
		if sid != -1 {
			SetScheduleEnded(uint32(sid))
		}
	}) {
		RemoveQueue(id)
	}
}

// checkaddjob adds a new job to the work queue if it doesn't already exist and the queue is not at the maximum number of running jobs. It returns the unique ID of the added job, or an error if the job could not be added.
//
// name is the name of the job to add.
// queue is the name of the queue to add the job to.
// fn is the function to execute for the job.
// schedulerID is the ID of the scheduler that requested the job.
// workpool is the worker pool to use for executing the job.
func checkaddjob(name string, queue string, fn func(), schedulerID uint32) (uint32, error) {
	if fn == nil {
		return 0, errors.New("empty func")
	}
	if checkQueue(name) {
		return 0, errors.New("already queued")
	} else if checklastadded(queue) {
		cleanupqueue(queue)
		id := newuuid()
		globalQueueSet.Set(id, dispatcherQueue{Name: name, Queue: job{Added: logger.TimeGetNow(), Name: name, Run: fn, Queue: queue, ID: id, SchedulerID: schedulerID}})
		return id, nil
	} else {
		return 0, errors.New("too many starting")
	}
}

// newuuid generates a new UUID string to use as a unique ID.
func newuuid() uint32 {
	return uuid.New().ID()
}

// DispatchEvery dispatches a job to run on a regular time interval. It takes in the interval duration, job name, queue, and function to run. It returns any error from setting up the ticker.
func DispatchEvery(interval time.Duration, name string, queue string, fn func()) error {
	schedulerID := newuuid()
	t := time.NewTicker(interval)
	//ticker = append(ticker, t)s

	go func() {
		defer logger.HandlePanic()
		for range t.C {
			addjob(name, queue, fn, schedulerID)
		}
	}()
	globalScheduleSet.Set(schedulerID, jobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval)})
	return nil
}

// Dispatch adds a new job with the given name, function, and queue to the
// worker pool. It generates a new UUID to associate with the job.
func Dispatch(name string, fn func(), queue string) error {
	addjob(name, queue, fn, newuuid())
	return nil
}

// InitWorkerPools initializes the worker pools for indexing, parsing,
// searching, downloading files, and updating metadata. It takes in the
// desired number of workers for each pool and defaults them to 1 if 0 is
// passed in. It configures the pools with balanced strategy and error
// handling function.
func InitWorkerPools(workersearch uint8, workerfiles uint8, workermeta uint8) {
	if workersearch == 0 {
		workersearch = 1
	}
	if workerfiles == 0 {
		workerfiles = 1
	}
	if workermeta == 0 {
		workermeta = 1
	}
	workerPoolSearch = pond.New(int(workersearch), 100, phandler, pond.Strategy(pond.Balanced()))
	workerPoolFiles = pond.New(int(workerfiles), 100, phandler, pond.Strategy(pond.Balanced()))
	workerPoolMetadata = pond.New(int(workermeta), 100, phandler, pond.Strategy(pond.Balanced()))
}

// CloseWorkerPools stops all worker pools and waits for workers
// to finish current jobs before returning. Waits up to 2 minutes
// per pool before timing out.
func CloseWorkerPools() {
	workerPoolSearch.StopAndWaitFor(2 * time.Minute)
	workerPoolFiles.StopAndWaitFor(2 * time.Minute)
	workerPoolMetadata.StopAndWaitFor(2 * time.Minute)
}

// dispatcherQueue is a struct that represents a queue for the dispatcher.
// It contains a Name field that is a string identifier for the queue
// and a Queue field that is a pointer to a Job struct that represents
// the job queue.
type dispatcherQueue struct {
	Name  string // Name is a string identifier for the queue
	Queue job    // Queue is a pointer to a Job struct representing the job queue
}

// Job represents a job to be run by a worker pool
type job struct {
	// Queue is the name of the queue this job belongs to
	Queue string
	// ID is a unique identifier for this job
	ID uint32
	// Added is the time this job was added to the queue
	Added time.Time
	// Started is the time this job was started by a worker
	Started time.Time
	// Name is a descriptive name for this job
	Name string
	// SchedulerID is the ID of the scheduler that added this job
	SchedulerID uint32
	// Run is the function to execute for this job
	Run func() `json:"-"`
	// CronJob is the cron job instance if this is a recurring cron job
	CronJob cron.Job `json:"-"`
}

// jobSchedule represents a scheduled job
type jobSchedule struct {
	// JobName is the name of the job
	JobName string
	// JobID is the unique ID of the job
	JobID uint32
	// ID is the unique ID for this schedule
	ID uint32
	// ScheduleTyp is the type of schedule (cron, interval, etc)
	ScheduleTyp string
	// ScheduleString is the schedule string (cron expression, interval, etc)
	ScheduleString string
	// Interval is the interval duration if schedule type is interval
	Interval time.Duration
	// CronSchedule is the parsed cron.Schedule if type is cron
	CronSchedule cron.Schedule
	// CronID is the cron scheduler ID if scheduled as cron job
	CronID cron.EntryID

	// LastRun is the last time this job ran
	LastRun time.Time
	// NextRun is the next scheduled run time
	NextRun time.Time
	// IsRunning indicates if the job is currently running
	IsRunning bool
}

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
func Cleanqueue() {
	if (uint64(workerPoolFiles.RunningWorkers()) + workerPoolFiles.WaitingTasks() + uint64(workerPoolMetadata.RunningWorkers()) + workerPoolMetadata.WaitingTasks() + uint64(workerPoolSearch.RunningWorkers()) + workerPoolSearch.WaitingTasks()) == 0 {
		globalQueueSet = nil
		//clear(globalQueueSet.values)
	}
}

// GetQueues returns a map of all currently configured queues, keyed by the queue name.
func GetQueues() map[uint32]dispatcherQueue {
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
	pre, post := logger.SplitByLR(jobname, '_')
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
	}

	return globalQueueSet.IterateMap(func(value dispatcherQueue) bool {
		if value.Name == jobname {
			return true
		}
		if alt1 == "" {
			return false
		}
		if (value.Name == logger.JoinStrings(alt1, post) || value.Name == logger.JoinStrings(alt2, post) || value.Name == logger.JoinStrings(alt3, post)) && !value.Queue.Started.IsZero() {
			return true
		}
		return false
	})
	// m := globalQueueSet.GetMap()
	// for key := range m {
	// 	if m[key].Name == jobname {
	// 		return true
	// 	}
	// 	if alt1 == "" {
	// 		continue
	// 	}
	// 	if m[key].Name == logger.JoinStrings(alt1, post) || m[key].Name == logger.JoinStrings(alt2, post) || m[key].Name == logger.JoinStrings(alt3, post) && !m[key].Queue.Started.IsZero() {
	// 		return true
	// 	}
	// }
	// return false
}
