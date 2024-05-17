package worker

import (
	"runtime"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/alitto/pond"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/robfig/cron/v3"
)

// workerPoolIndexer is a WorkerPool for executing indexer tasks
var WorkerPoolIndexer *pond.WorkerPool

// workerPoolParse is a WorkerPool for executing parse tasks
var WorkerPoolParse *pond.WorkerPool

// workerPoolSearch is a WorkerPool for executing search tasks
var WorkerPoolSearch *pond.WorkerPool

// workerPoolFiles is a WorkerPool for executing file tasks
var WorkerPoolFiles *pond.WorkerPool

// workerPoolMetadata is a WorkerPool for executing metadata tasks
var WorkerPoolMetadata *pond.WorkerPool

// cronWorkerData is a Cron instance for scheduling data worker jobs
var cronWorkerData *cron.Cron

// cronWorkerFeeds is a Cron instance for scheduling feeds worker jobs
var cronWorkerFeeds *cron.Cron

// cronWorkerSearch is a Cron instance for scheduling search worker jobs
var cronWorkerSearch *cron.Cron

// globalScheduleSet is a sync.Map to store JobSchedule objects
var globalScheduleSet = sync.Map{}

// globalQueueSet is a sync.Map to store dispatcherQueue objects
var globalQueueSet = sync.Map{}

// lastadded is a timestamp for tracking last added time
var lastadded = time.Now().Add(time.Second - 1)

// lastaddeddata is a timestamp for tracking last added data time
var lastaddeddata = time.Now().Add(time.Second - 1)

// lastaddedfeeds is a timestamp for tracking last added feeds time
var lastaddedfeeds = time.Now().Add(time.Second - 1)

// lastaddedsearch is a timestamp for tracking last search time
var lastaddedsearch = time.Now().Add(time.Second - 1)

var mu = sync.Mutex{}

// pljobs is a Pool for tracking jobs
var pljobs = pool.NewPool(100, 0, func(b *Job) {}, func(b *Job) {
	b.Run = nil
	b.CronJob = nil
	*b = Job{}
})

// phandler is a panic handler function
var phandler = pond.PanicHandler(func(p any) {
	logger.LogDynamic("error", "Recovered from panic (dispatcher)", logger.NewLogField("msg", Stack()), logger.NewLogField("vap", p))
})

func Stack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

type wrappedLogger struct {
	logger *zerolog.Logger
}

// Info logs an informational message to the wrapped logger.
// The message and key/value pairs are passed through to the wrapped
// zerolog Logger's Info method.
func (wl *wrappedLogger) Info(_ string, _ ...any) {
	//wl.logger.Info().Any("values", keysAndValues).Str("msg", msg).Msg("cron")
}

// Error logs an error message with additional key-value pairs to the wrapped logger.
// It takes in an error, a message string, and any number of key-value pairs.
func (wl *wrappedLogger) Error(err error, msg string, keysAndValues ...any) {
	logger.LogDynamic("error", "cron error", logger.NewLogField("values", keysAndValues), logger.NewLogField("msg", msg), logger.NewLogFieldValue(err))
	//wl.logger.Error().Err(err).Any("values", keysAndValues).Str("msg", msg).Msg("cron")
}

// submit adds the given job to the provided worker pool for execution.
// It checks if the job has a valid run function, and if the queue has
// reached its maximum capacity before submitting. On submission, it sets
// the job start time, adds it to the global queue set, updates the
// schedule, runs the job, updates the schedule again when finished,
// and removes the job from the global queue set.
func submit(queue *pond.WorkerPool, job *Job) {
	if job.Run == nil {
		pljobs.Put(job)
		return
	}
	if queue.MaxCapacity() <= int(queue.WaitingTasks()) {
		logger.LogDynamic("error", "queue limit reached", logger.NewLogField("job", &job.Name), logger.NewLogField("queue", &job.Queue))
	}
	queue.Submit(func() {
		id := job.ID
		job.Started = logger.TimeGetNow()
		q := dispatcherQueue{Name: job.Name, Queue: job}
		globalQueueSet.Store(id, q)
		s, ok := database.MapLoadP[JobSchedule](&globalScheduleSet, job.SchedulerID)
		defer func() {
			if ok {
				s.IsRunning = false
			}
			globalQueueSet.Delete(id)
			pljobs.Put(job)
		}()
		if ok {
			s.IsRunning = true
			s.LastRun = logger.TimeGetNow()
			if s.ScheduleTyp == "cron" {
				s.NextRun = s.CronSchedule.Next(logger.TimeGetNow())
			} else {
				s.NextRun = logger.TimeGetNow().Add(s.Interval)
			}
		}
		job.Run()
	})
}

// CreateCronWorker initializes the cron workers for data, feeds, and search.
// It configures each cron worker with the application logger, sets the timezone,
// adds error recovery and duplicate job prevention middleware,
// and enables running jobs at a per-second interval.
func CreateCronWorker() {
	loggerworker := wrappedLogger{logger: &logger.Log}
	cronWorkerData = cron.New(cron.WithLocation(logger.GetTimeZone()), cron.WithLogger(&loggerworker), cron.WithChain(cron.Recover(&loggerworker), cron.SkipIfStillRunning(&loggerworker)), cron.WithSeconds())
	cronWorkerFeeds = cron.New(cron.WithLocation(logger.GetTimeZone()), cron.WithLogger(&loggerworker), cron.WithChain(cron.Recover(&loggerworker), cron.SkipIfStillRunning(&loggerworker)), cron.WithSeconds())
	cronWorkerSearch = cron.New(cron.WithLocation(logger.GetTimeZone()), cron.WithLogger(&loggerworker), cron.WithChain(cron.Recover(&loggerworker), cron.SkipIfStillRunning(&loggerworker)), cron.WithSeconds())
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
	for i := 0; i < 11; i++ {
		if added.After(time.Now().Add(time.Millisecond * 200)) {
			time.Sleep(time.Millisecond * 200)
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
	cjob, err := dc.AddFunc(cronStr, func() {
		mu.Lock()
		defer mu.Unlock()
		if checkQueue(name) {
			logger.LogDynamic("error", "Job already queued", logger.NewLogField("Job", &name))
		} else if checklastadded(queue) {
			job := pljobs.Get()
			job.Added = logger.TimeGetNow()
			job.Name = name
			job.Run = fn
			job.Queue = queue
			job.SchedulerID = schedulerID
			job.ID = newuuid()
			//job := Job{Queue: queue, Name: name, Run: fn, ID: newuuid(), SchedulerID: schedulerID, Added: logger.TimeGetNow()}
			switch queue {
			case "Data":
				submit(WorkerPoolFiles, job)
			case "Feeds":
				submit(WorkerPoolMetadata, job)
			case "Search":
				submit(WorkerPoolSearch, job)
			}
		} else {
			logger.LogDynamic("error", "Job skipped - too many starting", logger.NewLogField("Job", &name))
		}
	})
	if err != nil {
		return err
	}
	dcentry := dc.Entry(cjob)
	s := JobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dcentry.Next,
		CronSchedule:   dcentry.Schedule,
		CronID:         cjob}
	globalScheduleSet.Store(schedulerID, &s)
	// globalScheduleSet.values = append(globalScheduleSet.values, JobSchedule{
	// 	JobName:        name,
	// 	JobID:          newuuid(),
	// 	ID:             schedulerID,
	// 	ScheduleTyp:    "cron",
	// 	ScheduleString: cronStr,
	// 	LastRun:        time.Time{},
	// 	NextRun:        dc.Entry(cjob).Next,
	// 	CronSchedule:   dc.Entry(cjob).Schedule,
	// 	CronID:         cjob})
	return nil
}

// addjob adds a new job with the given name, queue, function, and scheduler ID
// to the appropriate worker pool for processing. It checks if the job is already
// queued or if the queue is full before submitting.
func addjob(name string, queue string, fn func(), schedulerID string) {
	mu.Lock()
	defer mu.Unlock()
	if checkQueue(name) {
		logger.LogDynamic("error", "Job already queued", logger.NewLogField("Job", &name))
	} else if checklastadded(queue) {
		job := pljobs.Get()
		job.Added = logger.TimeGetNow()
		job.Name = name
		job.Run = fn
		job.Queue = queue
		job.SchedulerID = schedulerID
		job.ID = newuuid()

		//job := Job{Queue: queue, Name: name, Run: fn, ID: newuuid(), SchedulerID: schedulerID, Added: logger.TimeGetNow()}
		switch queue {
		case "Data":
			submit(WorkerPoolFiles, job)
		case "Feeds":
			submit(WorkerPoolMetadata, job)
		case "Search":
			submit(WorkerPoolSearch, job)
		}
	} else {
		logger.LogDynamic("error", "Job skipped - too many starting", logger.NewLogField("Job", &name))
	}
}

// newuuid generates a new UUID string to use as a unique ID.
func newuuid() string {
	return uuid.New().String()
}

// DispatchEvery dispatches a job to run on a regular time interval. It takes in the interval duration, job name, queue, and function to run. It returns any error from setting up the ticker.
func DispatchEvery(interval time.Duration, name string, queue string, fn func()) error {
	schedulerID := newuuid()
	t := time.NewTicker(interval)
	//ticker = append(ticker, t)s

	go func() {
		for range t.C {
			addjob(name, queue, fn, schedulerID)
		}
	}()
	s := JobSchedule{
		JobName:        name,
		JobID:          newuuid(),
		ID:             schedulerID,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval)}
	globalScheduleSet.Store(schedulerID, &s)
	// globalScheduleSet.values = append(globalScheduleSet.values, JobSchedule{
	// 	JobName:        name,
	// 	JobID:          newuuid(),
	// 	ID:             schedulerID,
	// 	ScheduleTyp:    "interval",
	// 	ScheduleString: interval.String(),
	// 	LastRun:        time.Time{},
	// 	Interval:       interval,
	// 	NextRun:        logger.TimeGetNow().Add(interval)})
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
func InitWorkerPools(workerindexer int, workerparse int, workersearch int, workerfiles int, workermeta int) {
	if workerindexer == 0 {
		workerindexer = 1
	}
	if workerparse == 0 {
		workerparse = 1
	}
	if workersearch == 0 {
		workersearch = 1
	}
	if workerfiles == 0 {
		workerfiles = 1
	}
	if workermeta == 0 {
		workermeta = 1
	}
	WorkerPoolIndexer = pond.New(workerindexer, 100, phandler, pond.Strategy(pond.Balanced())) //, pond.MinWorkers(1)
	WorkerPoolParse = pond.New(workerparse, 100, phandler, pond.Strategy(pond.Balanced()))
	WorkerPoolSearch = pond.New(workersearch, 100, phandler, pond.Strategy(pond.Balanced()))
	WorkerPoolFiles = pond.New(workerfiles, 100, phandler, pond.Strategy(pond.Balanced()))
	WorkerPoolMetadata = pond.New(workermeta, 100, phandler, pond.Strategy(pond.Balanced()))
}

// CloseWorkerPools stops all worker pools and waits for workers
// to finish current jobs before returning. Waits up to 2 minutes
// per pool before timing out.
func CloseWorkerPools() {
	WorkerPoolIndexer.StopAndWaitFor(2 * time.Minute)
	WorkerPoolParse.StopAndWaitFor(2 * time.Minute)
	WorkerPoolSearch.StopAndWaitFor(2 * time.Minute)
	WorkerPoolFiles.StopAndWaitFor(2 * time.Minute)
	WorkerPoolMetadata.StopAndWaitFor(2 * time.Minute)
}

// dispatcherQueue is a struct that represents a queue for the dispatcher.
// It contains a Name field that is a string identifier for the queue
// and a Queue field that is a pointer to a Job struct that represents
// the job queue.
type dispatcherQueue struct {
	Name  string // Name is a string identifier for the queue
	Queue *Job   // Queue is a pointer to a Job struct representing the job queue
}

// Job represents a job to be run by a worker pool
type Job struct {
	// Queue is the name of the queue this job belongs to
	Queue string
	// ID is a unique identifier for this job
	ID string
	// Added is the time this job was added to the queue
	Added time.Time
	// Started is the time this job was started by a worker
	Started time.Time
	// Name is a descriptive name for this job
	Name string
	// SchedulerID is the ID of the scheduler that added this job
	SchedulerID string
	// Run is the function to execute for this job
	Run func() `json:"-"`
	// CronJob is the cron job instance if this is a recurring cron job
	CronJob cron.Job `json:"-"`
}

// JobSchedule represents a scheduled job
type JobSchedule struct {
	// JobName is the name of the job
	JobName string
	// JobID is the unique ID of the job
	JobID string
	// ID is the unique ID for this schedule
	ID string
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

// type scheduleSet struct {
// 	values []JobSchedule
// }
// type queueSet struct {
// 	values []dispatcherQueue
// }

// removeID removes the dispatcherQueue with the given ID str from the queueSet s.
// It locks the queueSet, loops through to find the matching ID, deletes
// that element from the values slice, and unlocks the queueSet before returning.
// func (s *queueSet) removeID(str string) {
// 	for idx := range s.values {
// 		if s.values[idx].Queue.ID == str {
// 			s.values = slices.Delete(s.values, idx, idx+1)
// 			break
// 		}
// 	}
// }

// Cleanqueue clears the global queue set if there are no running or waiting workers across all pools.
func Cleanqueue() {
	if (uint64(WorkerPoolFiles.RunningWorkers()) + WorkerPoolFiles.WaitingTasks() + uint64(WorkerPoolMetadata.RunningWorkers()) + WorkerPoolMetadata.WaitingTasks() + uint64(WorkerPoolSearch.RunningWorkers()) + WorkerPoolSearch.WaitingTasks()) == 0 {
		globalQueueSet.Range(func(key, value interface{}) bool {
			globalQueueSet.Delete(key)
			return true
		})
		//clear(globalQueueSet.values)
	}
}

// GetQueues returns a map of all currently configured queues, keyed by the queue name.
func GetQueues() map[string]dispatcherQueue {
	globalQueue := make(map[string]dispatcherQueue)
	globalQueueSet.Range(func(key, value interface{}) bool {
		s := value.(dispatcherQueue)
		globalQueue[key.(string)] = s
		return true
	})
	// for idx := range globalQueueSet.values {
	// 	globalQueue[globalQueueSet.values[idx].Name] = globalQueueSet.values[idx]
	// }
	return globalQueue
}

// GetSchedules returns a map of all currently configured schedules,
// keyed by the job name.
func GetSchedules() map[string]JobSchedule {
	globalSchedules := make(map[string]JobSchedule)
	globalScheduleSet.Range(func(key, value interface{}) bool {
		s := value.(*JobSchedule)
		globalSchedules[key.(string)] = *s
		return true
	})
	// for idx := range globalScheduleSet.values {
	// 	globalSchedules[globalScheduleSet.values[idx].JobName] = globalScheduleSet.values[idx]
	// }

	return globalSchedules
}

// updateStartedSchedule updates the IsRunning, LastRun, and NextRun fields
// for the schedule with the given ID str. It locks the schedule set, loops
// through to find the matching schedule, sets the IsRunning field to true,
// updates LastRun to the current time, and calculates the NextRun based on
// whether it is a cron or interval schedule before unlocking the set.
// func (s *scheduleSet) updateStartedSchedule(str string) {
// 	for idx := range s.values {
// 		if s.values[idx].ID != str {
// 			continue
// 		}
// 		s.values[idx].IsRunning = true
// 		s.values[idx].LastRun = logger.TimeGetNow()
// 		if s.values[idx].ScheduleTyp == "cron" {
// 			s.values[idx].NextRun = s.values[idx].CronSchedule.Next(logger.TimeGetNow())
// 			return
// 		}
// 		s.values[idx].NextRun = logger.TimeGetNow().Add(s.values[idx].Interval)

// 		break
// 	}
// }

// updateIsRunningSchedule updates the IsRunning field for the schedule with
// the given ID str. It locks the schedule set, loops through to find the
// matching schedule, sets the IsRunning field to the passed in boolean
// value, then unlocks the set before returning. This allows atomically
// updating the running state of a schedule.
// func (s *scheduleSet) updateIsRunningSchedule(str string, isrunning bool) {
// 	for idx := range s.values {
// 		if s.values[idx].ID == str {
// 			s.values[idx].IsRunning = isrunning
// 			break
// 		}
// 	}
// }

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
	default:
		return false
	}

	var bl bool
	globalQueueSet.Range(func(key, value interface{}) bool {
		c := value.(dispatcherQueue)
		if (c.Name == jobname || c.Name == (alt1+post) || c.Name == (alt2+post) || c.Name == (alt3+post)) && !c.Queue.Started.IsZero() {
			bl = true
			return false
		}
		return true
	})
	return bl
	// for _, c := range globalQueueSet.values {
	// 	if (c.Name == jobname || c.Name == (alt1+post) || c.Name == (alt2+post) || c.Name == (alt3+post)) && !c.Queue.Started.IsZero() {
	// 		return true
	// 	}
	// }
	// return false
}
