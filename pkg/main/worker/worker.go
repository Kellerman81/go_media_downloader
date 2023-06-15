package worker

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/alitto/pond"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	//"github.com/panjf2000/ants/v2"
	"github.com/robfig/cron/v3"
)

var (
	Crons []*cron.Cron
	//Cron   *cron.Cron = cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.New(os.Stdout, "cron: ", log.LstdFlags))), cron.WithChain(cron.Recover(cron.DefaultLogger), cron.DelayIfStillRunning(cron.DefaultLogger)), cron.WithSeconds())
	Ticker []*time.Ticker
	//WorkerPools        map[string]*pond.WorkerPool
	WorkerPoolIndexer  *pond.WorkerPool
	WorkerPoolParse    *pond.WorkerPool
	WorkerPoolSearch   *pond.WorkerPool
	WorkerPoolFiles    *pond.WorkerPool
	WorkerPoolMetadata *pond.WorkerPool
)

func stack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}
func runwrapper(job *Job) func() {
	return func() {
		defer func() { // recovers panic
			if e := recover(); e != nil {
				logger.Log.Error().Str("msg", stack()).Msgf("Recovered from panic (dispatcher1) %v", e)
			}
		}()
		//logger.Log.Debug().Msgf("Started func %s %s", job.Name, job.ID)
		globalQueueSet.add(&dispatcherQueue{Name: job.Name, Queue: job})
		globalScheduleSet.updateStartedSchedule(job.SchedulerID)
		globalQueueSet.update(job.ID)
		globalScheduleSet.updateIsRunningSchedule(job.SchedulerID, true)
		job.Run()
		globalScheduleSet.updateIsRunningSchedule(job.SchedulerID, false)
		globalQueueSet.removeID(job.ID)
		//logger.Log.Debug().Msgf("Ended func %s %s", job.Name, job.ID)
	}
}

type wrappedLogger struct {
	logger *zerolog.Logger
}

// Info logs routine messages about cron's operation.
func (wl *wrappedLogger) Info(msg string, keysAndValues ...interface{}) {
	//logger.LogAnyInfo("cron "+msg, logger.LoggerValue{Name: "values", Value: keysAndValues})
	wl.logger.Info().Any("values", keysAndValues).Msg("cron " + msg)
}

// Error logs an error condition.
func (wl *wrappedLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	//logger.LogAnyError(err, "cron "+msg, logger.LoggerValue{Name: "values", Value: keysAndValues})
	wl.logger.Error().Err(err).Any("values", keysAndValues).Msg("cron " + msg)
}

func DispatchCron(queue *pond.WorkerPool, cronStr string, job *Job) error {
	job.SchedulerID = uuid.New().String()
	dc := cron.New(cron.WithLocation(&logger.TimeZone), cron.WithChain(cron.Recover(&wrappedLogger{logger: &logger.Log}), cron.SkipIfStillRunning(&wrappedLogger{logger: &logger.Log})), cron.WithSeconds())
	Crons = append(Crons, dc)

	job.ID = uuid.New().String()
	cjob, err := dc.AddJob(cronStr, cron.FuncJob(func() {
		if checkQueue(job.Name) {
			logger.Log.Error().Err(nil).Str("Job", job.Name).Msg("Job already queued")
		} else {
			job.Added = logger.TimeGetNow()
			queue.Submit(runwrapper(job))
		}
	}))
	//cjob, err := Cron.AddJob(cronStr, j)
	if err != nil {
		return err
	}
	dc.Start()
	globalScheduleSet.add(&JobSchedule{
		JobName:        job.Name,
		JobID:          job.ID,
		ID:             job.SchedulerID,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.Entry(cjob).Next,
		CronSchedule:   dc.Entry(cjob).Schedule,
		CronID:         cjob})
	return nil
}
func DispatchEvery(queue *pond.WorkerPool, interval time.Duration, job *Job) error {
	job.SchedulerID = uuid.New().String()
	t := time.NewTicker(interval)
	Ticker = append(Ticker, t)
	job.ID = uuid.New().String()

	go func() {
		for range t.C {
			if checkQueue(job.Name) {
				logger.Log.Error().Err(nil).Str("Job", job.Name).Msg("Job already queued")
			} else {
				job.Added = logger.TimeGetNow()
				queue.Submit(runwrapper(job))
			}
		}
	}()

	globalScheduleSet.add(&JobSchedule{
		JobName:        job.Name,
		JobID:          job.ID,
		ID:             job.SchedulerID,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval)})
	return nil
}
func NewJob(name string, queue string) *Job {
	return &Job{Queue: queue, ID: uuid.New().String(), Added: logger.TimeGetNow(), Name: name, SchedulerID: uuid.New().String()}
}
func NewJobFunc(name string, run func(), queue string) *Job {
	return &Job{Queue: queue, ID: uuid.New().String(), Added: logger.TimeGetNow(), Name: name, Run: run, SchedulerID: uuid.New().String()}
}
func Dispatch(queue *pond.WorkerPool, job *Job) error {

	if checkQueue(job.Name) {
		logger.Log.Error().Err(nil).Str("Job", job.Name).Msg("Job already queued")
	} else {
		queue.Submit(runwrapper(job))
	}
	return nil
}
func DispatchDirect(queue *pond.WorkerPool, job *Job) error {
	if checkQueue(job.Name) {
		logger.Log.Error().Err(nil).Str("Job", job.Name).Msg("Job already queued")
	} else {
		queue.Submit(runwrapper(job))
	}
	return nil
}

func InitWorkerPools(workerindexer int, workerparse int, workersearch int, workerfiles int, workermeta int) {
	//WorkerPools = make(map[string]*pond.WorkerPool, 5)
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
	WorkerPoolIndexer = InitWorker(workerindexer, 200, 1*time.Hour)
	WorkerPoolParse = InitWorker(workerparse, 200, 1*time.Hour)
	WorkerPoolSearch = InitWorker(workersearch, 200, 1*time.Hour)
	WorkerPoolFiles = InitWorker(workerfiles, 200, 1*time.Hour)
	WorkerPoolMetadata = InitWorker(workermeta, 200, 1*time.Hour)
	//Cron.Start()
}

func CloseWorkerPools() {
	WorkerPoolIndexer.Stop()
	WorkerPoolParse.Stop()
	WorkerPoolSearch.Stop()
	WorkerPoolFiles.Stop()
	WorkerPoolMetadata.Stop()
}

type PreWorker struct {
	Queuegroup *pond.WorkerPool
}

// Init non blocking Worker
func InitWorker(workercount int, queuesize int, timeout time.Duration) *pond.WorkerPool {
	return pond.New(workercount, queuesize, pond.IdleTimeout(10*time.Second), pond.PanicHandler(func(p interface{}) {
		logger.Log.Error().Str("msg", stack()).Msgf("Recovered from panic (dispatcher2) %v", p)
	}), pond.Strategy(pond.Balanced())) //pond.PanicHandler(panicHandler),
}

type dispatcherQueue struct {
	Name  string
	Queue *Job
}
type Job struct {
	Queue       string
	ID          string
	Added       time.Time
	Started     time.Time
	Name        string
	SchedulerID string
	Run         func() `json:"-"`
}
type JobSchedule struct {
	JobName        string
	JobID          string
	ID             string
	ScheduleTyp    string
	ScheduleString string
	Interval       time.Duration
	CronSchedule   cron.Schedule
	CronID         cron.EntryID

	LastRun   time.Time
	NextRun   time.Time
	IsRunning bool
}

type scheduleSet struct {
	values []JobSchedule
	mu     *sync.Mutex
}
type queueSet struct {
	values []dispatcherQueue
	mu     *sync.Mutex
}

var (
	globalScheduleSet = newScheduleSet()
	globalQueueSet    = newQueueSet()
)

func newScheduleSet() *scheduleSet {
	return &scheduleSet{values: make([]JobSchedule, 0, 100), mu: &sync.Mutex{}}
}

func (s *scheduleSet) add(str *JobSchedule) {
	s.mu.Lock()
	s.values = append(s.values, *str)
	s.mu.Unlock()
}

func (s *scheduleSet) removeID(str string) {
	s.mu.Lock()
	intid := -1
	for idxi := range s.values {
		if s.values[idxi].JobID == str {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&s.values, func(c JobSchedule) bool { return c.JobID == str })
	if intid != -1 {
		logger.Delete(&s.values, intid)
	}
	s.mu.Unlock()
}

func newQueueSet() *queueSet {
	return &queueSet{values: make([]dispatcherQueue, 0, 100), mu: &sync.Mutex{}}
}

func (s *queueSet) add(str *dispatcherQueue) {
	s.mu.Lock()
	s.values = append(s.values, *str)
	s.mu.Unlock()
}

func (s *queueSet) removeID(str string) {
	s.mu.Lock()
	intid := -1
	for idxi := range s.values {
		if s.values[idxi].Queue.ID == str {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&s.values, func(c dispatcherQueue) bool { return c.Queue.ID == str })
	if intid != -1 {
		logger.Delete(&s.values, intid)
	}
	s.mu.Unlock()
}

func (s *queueSet) update(str string) {
	s.mu.Lock()
	intid := -1
	for idxi := range s.values {
		if s.values[idxi].Queue.ID == str {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&s.values, func(c dispatcherQueue) bool { return c.Queue.ID == str })
	if intid != -1 {
		s.values[intid].Queue.Started = logger.TimeGetNow()
	}
	s.mu.Unlock()
}

func Cleanqueue() {
	if (uint64(WorkerPoolFiles.RunningWorkers())+WorkerPoolFiles.WaitingTasks()+uint64(WorkerPoolMetadata.RunningWorkers())+WorkerPoolMetadata.WaitingTasks()+uint64(WorkerPoolSearch.RunningWorkers())+WorkerPoolSearch.WaitingTasks()) == 0 && globalQueueSet.values != nil {
		globalQueueSet.mu.Lock()
		globalQueueSet.values = nil
		globalQueueSet.mu.Unlock()
	}
}

func GetQueues() map[string]dispatcherQueue {
	globalQueue := make(map[string]dispatcherQueue, len(globalQueueSet.values))
	for idx := range globalQueueSet.values {
		globalQueue[globalQueueSet.values[idx].Name] = globalQueueSet.values[idx]
	}
	return globalQueue
}
func GetSchedules() map[string]JobSchedule {
	globalSchedules := make(map[string]JobSchedule, len(globalScheduleSet.values))
	for idx := range globalScheduleSet.values {
		globalSchedules[globalScheduleSet.values[idx].JobName] = globalScheduleSet.values[idx]
	}

	return globalSchedules
}

func (s *scheduleSet) updateStartedSchedule(str string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := -1
	for idxi := range s.values {
		if s.values[idxi].ID == str {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&s.values, func(c JobSchedule) bool { return c.ID == str })
	if intid != -1 {
		s.values[intid].LastRun = logger.TimeGetNow()
		if s.values[intid].ScheduleTyp == "cron" {
			s.values[intid].NextRun = s.values[intid].CronSchedule.Next(logger.TimeGetNow())
			return
		}
		s.values[intid].NextRun = logger.TimeGetNow().Add(s.values[intid].Interval)
	}
}
func (s *scheduleSet) updateIsRunningSchedule(str string, isrunning bool) {
	s.mu.Lock()
	intid := -1
	for idxi := range s.values {
		if s.values[idxi].ID == str {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(&s.values, func(c JobSchedule) bool { return c.ID == str })
	if intid != -1 {
		s.values[intid].IsRunning = isrunning
	}
	s.mu.Unlock()
}

// type PoolWait struct {
// 	Pool  *PreWorker
// 	Group *pond.TaskGroup
// 	//wg    sync.WaitGroup
// }

// func NewWaitGroup(pool *PreWorker) PoolWait {
// 	return PoolWait{Pool: pool, Group: pool.queuegroup.Group()}
// }

// func (p *PoolWait) waitfunwrapper(fun func()) func() {
// 	return func() {

// 		defer func() { // recovers panic
// 			//p.wg.Done()
// 			if e := recover(); e != nil {
// 				logger.Log.Error().Msgf("Recovered from panic (dispatcher2) %v", e)
// 			}
// 		}()
// 		//logger.Log.Debug().Msg("Started sub func")
// 		fun()
// 		//logger.Log.Debug().Msg("Ended sub func")
// 	}
// }
// func (p *PoolWait) Submit(fun func()) {
// 	p.Group.Submit(p.waitfunwrapper(fun))
// 	//p.wg.Add(1)
// 	//p.Pool.Submit(p.waitfunwrapper(fun))
// }
// func (p *PoolWait) Wait() {
// 	p.Group.Wait()
// 	//logger.Log.Debug().Msg("Waiting for jobs to finish")
// 	//p.wg.Wait()
// 	//logger.Log.Debug().Msg("Finished Waiting for jobs to finish")
// }

func checkQueue(job string) bool {
	var alt1, alt2, alt3 string
	if strings.HasPrefix(job, "searchmissinginc_") {
		alt1 = strings.Replace(job, "searchmissinginc_", "searchmissinginctitle_", 1)
		alt2 = strings.Replace(job, "searchmissinginc_", "searchmissingfull_", 1)
		alt3 = strings.Replace(job, "searchmissinginc_", "searchmissingfulltitle_", 1)
	} else if strings.HasPrefix(job, "searchmissinginctitle_") {
		alt1 = strings.Replace(job, "searchmissinginctitle_", "searchmissinginc_", 1)
		alt2 = strings.Replace(job, "searchmissinginctitle_", "searchmissingfull_", 1)
		alt3 = strings.Replace(job, "searchmissinginctitle_", "searchmissingfulltitle_", 1)
	} else if strings.HasPrefix(job, "searchmissingfull_") {
		alt1 = strings.Replace(job, "searchmissingfull_", "searchmissinginctitle_", 1)
		alt2 = strings.Replace(job, "searchmissingfull_", "searchmissinginc_", 1)
		alt3 = strings.Replace(job, "searchmissingfull_", "searchmissingfulltitle_", 1)
	} else if strings.HasPrefix(job, "searchmissingfulltitle_") {
		alt1 = strings.Replace(job, "searchmissingfulltitle_", "searchmissinginctitle_", 1)
		alt2 = strings.Replace(job, "searchmissingfulltitle_", "searchmissingfull_", 1)
		alt3 = strings.Replace(job, "searchmissingfulltitle_", "searchmissinginc_", 1)
	} else if strings.HasPrefix(job, "searchupgradeinc_") {
		alt1 = strings.Replace(job, "searchupgradeinc_", "searchupgradeinctitle_", 1)
		alt2 = strings.Replace(job, "searchupgradeinc_", "searchupgradefull_", 1)
		alt3 = strings.Replace(job, "searchupgradeinc_", "searchupgradefulltitle_", 1)
	} else if strings.HasPrefix(job, "searchupgradeinctitle_") {
		alt1 = strings.Replace(job, "searchupgradeinctitle_", "searchupgradeinc_", 1)
		alt2 = strings.Replace(job, "searchupgradeinctitle_", "searchupgradefull_", 1)
		alt3 = strings.Replace(job, "searchupgradeinctitle_", "searchupgradefulltitle_", 1)
	} else if strings.HasPrefix(job, "searchupgradefull_") {
		alt1 = strings.Replace(job, "searchupgradefull_", "searchupgradeinctitle_", 1)
		alt2 = strings.Replace(job, "searchupgradefull_", "searchupgradeinc_", 1)
		alt3 = strings.Replace(job, "searchupgradefull_", "searchupgradefulltitle_", 1)
	} else if strings.HasPrefix(job, "searchupgradefulltitle_") {
		alt1 = strings.Replace(job, "searchupgradefulltitle_", "searchupgradeinctitle_", 1)
		alt2 = strings.Replace(job, "searchupgradefulltitle_", "searchupgradefull_", 1)
		alt3 = strings.Replace(job, "searchupgradefulltitle_", "searchupgradeinc_", 1)
	}

	globalQueueSet.mu.Lock()
	defer globalQueueSet.mu.Unlock()
	for idx := range globalQueueSet.values {
		if (globalQueueSet.values[idx].Name == job || globalQueueSet.values[idx].Name == alt1 || globalQueueSet.values[idx].Name == alt2 || globalQueueSet.values[idx].Name == alt3) && !globalQueueSet.values[idx].Queue.Started.IsZero() {
			return true
		}
	}
	return false
}
