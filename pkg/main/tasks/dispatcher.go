package tasks

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

//Source: https://github.com/mborders/artifex
//Several Mods

// Dispatcher maintains a pool for available workers
// and a job queue that workers will process
type Dispatcher struct {
	maxWorkers int
	maxQueue   int
	workers    []*worker
	tickers    []*dispatchTicker
	crons      []*dispatchCron
	//Cron       *cron.Cron
	workerPool chan chan Job
	jobQueue   chan Job

	quit   chan bool
	active bool
	name   string
}

type dispatcherQueue struct {
	Name  string
	Queue Job
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

// DispatchTicker represents a dispatched job ticker
// that executes on a given interval. This provides
// a means for stopping the execution cycle from continuing.
type dispatchTicker struct {
	ticker      *time.Ticker
	schedulerid string
	quit        chan bool
}

// DispatchCron represents a dispatched cron job
// that executes using cron expression formats.
type dispatchCron struct {
	cron        *cron.Cron
	schedulerid string
}

var errDispatcherInactive = errors.New("dispatcher is not active")
var errInvalidCron = errors.New("invalid cron")
var globalScheduleSet = NewScheduleSet()
var globalQueueSet = NewQueueSet()

func NewScheduleSet() *scheduleSet {
	return &scheduleSet{values: make([]JobSchedule, 0, 200), mu: &sync.Mutex{}}
}

func (s *scheduleSet) Add(str JobSchedule) {
	s.mu.Lock()
	s.values = append(s.values, str)
	s.mu.Unlock()
}

func (s *scheduleSet) Length() int {
	return len(s.values)
}

func (s *scheduleSet) RemoveID(str string) {
	s.mu.Lock()
	new := s.values[:0]
	for idx := range s.values {
		if s.values[idx].JobID != str {
			new = append(new, s.values[idx])
		}
	}
	s.values = new
	s.mu.Unlock()
}

func (s *scheduleSet) ContainsID(str string) bool {
	return slices.ContainsFunc(s.values, func(c JobSchedule) bool { return c.ID == str })
	// for idx := range s.values {
	// 		if s.values[idx].ID == str {
	// 			return true
	// 		}
	// 	}
	// 	return false
}
func (s *scheduleSet) GetID(str string) *JobSchedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c JobSchedule) bool { return c.ID == str })
	if intid != -1 {
		return &s.values[intid]
	}
	// for idx := range s.values {
	// 	if s.values[idx].ID == str {
	// 		return &s.values[idx]
	// 	}
	// }
	return nil
}
func (s *scheduleSet) ContainsName(str string) bool {
	return slices.ContainsFunc(s.values, func(c JobSchedule) bool { return c.JobName == str })
	// for idx := range s.values {
	// 	if s.values[idx].JobName == str {
	// 		return true
	// 	}
	// }
	// return false
}
func (s *scheduleSet) GetName(str string) *JobSchedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c JobSchedule) bool { return c.JobName == str })
	if intid != -1 {
		return &s.values[intid]
	}
	// for idx := range s.values {
	// 	if s.values[idx].JobName == str {
	// 		return &s.values[idx]
	// 	}
	// }
	return nil
}
func (s *scheduleSet) Update(str *JobSchedule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c JobSchedule) bool { return c.ID == str.ID })
	if intid != -1 {
		s.values[intid] = *str
	}
	// for idx := range s.values {
	// 	if s.values[idx].ID == str.ID {
	// 		s.values[idx] = *str
	// 		return
	// 	}
	// }
}

func (s *scheduleSet) Clear() {
	s.values = nil
}

func NewQueueSet() *queueSet {
	return &queueSet{values: make([]dispatcherQueue, 0, 200), mu: &sync.Mutex{}}
}

func (s *queueSet) Add(str dispatcherQueue) {
	s.mu.Lock()
	s.values = append(s.values, str)
	s.mu.Unlock()
}

func (s *queueSet) Length() int {
	return len(s.values)
}

func (s *queueSet) RemoveID(str string) {
	s.mu.Lock()
	new := s.values[:0]
	for idx := range s.values {
		if s.values[idx].Queue.ID != str {
			new = append(new, s.values[idx])
		}
	}
	s.values = new
	s.mu.Unlock()
}

func (s *queueSet) ContainsName(str string) bool {
	return slices.ContainsFunc(s.values, func(c dispatcherQueue) bool { return c.Name == str })

	// for idx := range s.values {
	// 	if s.values[idx].Name == str {
	// 		return true
	// 	}
	// }
	// return false
}
func (s *queueSet) GetName(str string) *dispatcherQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c dispatcherQueue) bool { return c.Name == str })
	if intid != -1 {
		return &s.values[intid]
	}
	// for idx := range s.values {
	// 	if s.values[idx].Name == str {
	// 		return &s.values[idx]
	// 	}
	// }
	return nil
}
func (s *queueSet) GetID(str string) *dispatcherQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c dispatcherQueue) bool { return c.Queue.ID == str })
	if intid != -1 {
		return &s.values[intid]
	}
	// for idx := range s.values {
	// 	if s.values[idx].Queue.ID == str {
	// 		return &s.values[idx]
	// 	}
	// }
	return nil
}
func (s *queueSet) Update(str *dispatcherQueue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intid := slices.IndexFunc(s.values, func(c dispatcherQueue) bool { return c.Queue.ID == str.Queue.ID })
	if intid != -1 {
		s.values[intid] = *str
	}
	// for idx := range s.values {
	// 	if s.values[idx].Queue.ID == str.Queue.ID {
	// 		s.values[idx] = *str
	// 		return
	// 	}
	// }
}

func (s *queueSet) Clear() {
	s.values = nil
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

func updateStartedSchedule(findname string) {
	jobschedule := globalScheduleSet.GetID(findname)
	if jobschedule == nil {
		return
	}
	if jobschedule.ScheduleTyp == "cron" {
		jobschedule.NextRun = jobschedule.CronSchedule.Next(logger.TimeGetNow())
		jobschedule.LastRun = logger.TimeGetNow()
	} else {
		jobschedule.NextRun = logger.TimeGetNow().Add(jobschedule.Interval)
		jobschedule.LastRun = logger.TimeGetNow()
	}
	globalScheduleSet.Update(jobschedule)
}
func updateIsRunningSchedule(findname string, isrunning bool) {
	jobschedule := globalScheduleSet.GetID(findname)
	if jobschedule != nil {
		jobschedule.IsRunning = isrunning
		globalScheduleSet.Update(jobschedule)
	}
}
func updateStartedQueue(findname string) {
	que := globalQueueSet.GetID(findname)
	if que != nil {
		que.Queue.Started = logger.TimeGetNow()
		globalQueueSet.Update(que)
	}
}

// NewDispatcher creates a new dispatcher with the given
// number of workers and buffers the job queue based on maxQueue.
// It also initializes the channels for the worker pool and job queue
func NewDispatcher(name string, maxWorkers int, maxQueue int) *Dispatcher {
	return &Dispatcher{
		name:       name,
		maxWorkers: maxWorkers,
		maxQueue:   maxQueue,
		//Cron:       cron.New(cron.WithSeconds()),
	}
}

// Start creates and starts workers, adding them to the worker pool.
// Then, it starts a select loop to wait for job to be dispatched
// to available workers
func (d *Dispatcher) Start() {
	d.workers = []*worker{}
	d.tickers = []*dispatchTicker{}
	d.crons = []*dispatchCron{}
	//d.Cron = &DispatchCron{}
	d.workerPool = make(chan chan Job, d.maxWorkers)
	d.jobQueue = make(chan Job, d.maxQueue)
	d.quit = make(chan bool)

	//d.Cron = &DispatchCron{Cron: cron.New(cron.WithSeconds()), schedulerid: uuid.New().String()}

	//d.Cron.Cron.Start()
	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(d.workerPool)
		worker.start()
		d.workers = append(d.workers, worker)
	}

	d.active = true

	go func() {
		// defer func() { // recovers panic
		// 	if e := recover(); e != nil {
		// 		logger.Log.GlobalLogger.Error("Recovered from panic (dispatcher1) ", e)
		// 	}
		// }()
		for {
			select {
			case job := <-d.jobQueue:
				if !checkQueue(job.Name) {
					updateStartedSchedule(job.SchedulerID)
					go func(jobadd Job) {
						jobChannel := <-d.workerPool
						jobChannel <- jobadd
					}(job)
				} else {
					logger.Log.GlobalLogger.Warn("Skip Job", zap.String("id", job.ID), zap.String("name", job.Name))
					globalQueueSet.RemoveID(job.ID)
				}
			case <-d.quit:
				return
			}
		}
	}()
}

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
	for _, value := range globalQueueSet.values {
		if (value.Name == job || value.Name == alt1 || value.Name == alt2 || value.Name == alt3) && !value.Queue.Started.IsZero() {
			return true
		}
	}
	return false
}

// Stop ends execution for all workers/tickers and
// closes all channels, then removes all workers/tickers
func (d *Dispatcher) Stop() {
	if !d.active {
		return
	}

	d.active = false

	for i := range d.workers {
		d.workers[i].stop()
	}

	for i := range d.tickers {
		d.tickers[i].stop()
	}

	for i := range d.crons {
		d.crons[i].Stop()
	}
	//d.Cron.Stop()

	d.workers = []*worker{}
	d.tickers = []*dispatchTicker{}
	d.crons = []*dispatchCron{}
	d.quit <- true
}

// Dispatch pushes the given job into the job queue.
// The first available worker will perform the job
func (d *Dispatcher) Dispatch(name string, run func()) error {
	if !d.active {
		return errDispatcherInactive
	}
	job := Job{Queue: d.name, ID: uuid.New().String(), Added: logger.TimeGetNow(), Name: name, Run: run}
	d.jobQueue <- job

	globalQueueSet.Add(dispatcherQueue{Name: job.Name, Queue: job})
	return nil
}

func (d *Dispatcher) TryEnqueue(job Job) bool {
	select {
	case d.jobQueue <- job:
		return true
	default:
		return false
	}
}

// DispatchIn pushes the given job into the job queue
// after the given duration has elapsed
func (d *Dispatcher) DispatchIn(name string, run func(), duration time.Duration) error {
	if !d.active {
		return errDispatcherInactive
	}

	go func() {
		defer func() { // recovers panic
			if e := recover(); e != nil {
				logger.Log.GlobalLogger.Error("Recovered from panic (dispatcher3) ", zap.Any("", e))
			}
		}()
		time.Sleep(duration)
		job := Job{Queue: d.name, ID: uuid.New().String(), Added: logger.TimeGetNow(), Name: name, Run: run}

		if d.TryEnqueue(job) {
			globalQueueSet.Add(dispatcherQueue{Name: job.Name, Queue: job})
		}

	}()

	return nil
}

// DispatchEvery pushes the given job into the job queue
// continuously at the given interval
func (d *Dispatcher) DispatchEvery(name string, run func(), interval time.Duration) (*dispatchTicker, error) {
	if !d.active {
		return nil, errDispatcherInactive
	}

	t := time.NewTicker(interval)
	schedulerid := uuid.New().String()
	jobid := uuid.New().String()

	globalScheduleSet.Add(JobSchedule{
		JobID:          jobid,
		ID:             schedulerid,
		JobName:        name,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        logger.TimeGetNow().Add(interval)})
	dt := &dispatchTicker{ticker: t, quit: make(chan bool), schedulerid: schedulerid}
	d.tickers = append(d.tickers, dt)

	go func() {
		// defer func() { // recovers panic
		// 	if e := recover(); e != nil {
		// 		logger.Log.GlobalLogger.Error("Recovered from panic (dispatcher4) ", e)
		// 	}
		// }()
		for {
			select {
			case <-t.C:
				job := Job{Queue: d.name, ID: jobid, Added: logger.TimeGetNow(), Name: name, Run: run, SchedulerID: schedulerid}

				if d.TryEnqueue(job) {
					globalQueueSet.Add(dispatcherQueue{Name: job.Name, Queue: job})
				}

			case <-dt.quit:
				return
			}
		}
	}()

	return dt, nil
}

// DispatchEvery pushes the given job into the job queue
// each time the cron definition is met
func (d *Dispatcher) DispatchCron(name string, run func(), cronStr string) error {
	if !d.active {
		return errDispatcherInactive
	}

	schedulerid := uuid.New().String()
	dc := &dispatchCron{cron: cron.New(cron.WithSeconds()), schedulerid: schedulerid}
	d.crons = append(d.crons, dc)

	jobid := uuid.New().String()
	cjob, err := dc.cron.AddFunc(cronStr, func() {
		job := Job{Queue: d.name, ID: jobid, Added: logger.TimeGetNow(), Name: name, Run: run, SchedulerID: schedulerid}

		if d.TryEnqueue(job) {
			globalQueueSet.Add(dispatcherQueue{Name: job.Name, Queue: job})
		}
	})
	if err != nil {
		return errInvalidCron
	}

	globalScheduleSet.Add(JobSchedule{
		JobName:        name,
		JobID:          jobid,
		ID:             schedulerid,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.cron.Entry(cjob).Next,
		CronSchedule:   dc.cron.Entry(cjob).Schedule,
		CronID:         cjob})
	dc.cron.Start()

	return nil
}

// Stop ends the execution cycle for the given ticker.
func (dt *dispatchTicker) stop() {
	dt.ticker.Stop()
	globalScheduleSet.RemoveID(dt.schedulerid)
	dt.quit <- true
}

// Stops ends the execution cycle for the given cron.
func (c *dispatchCron) Stop() {
	c.cron.Stop()
	globalScheduleSet.RemoveID(c.schedulerid)
}
