package tasks

import (
	"errors"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

//Source: https://github.com/mborders/artifex
//Several Mods

// Dispatcher maintains a pool for available workers
// and a job queue that workers will process
type Dispatcher struct {
	maxWorkers int
	maxQueue   int
	workers    []*Worker
	tickers    []*DispatchTicker
	crons      []*DispatchCron
	//Cron       *cron.Cron
	workerPool chan chan Job
	jobQueue   chan Job

	quit   chan bool
	active bool
	name   string
}

type DispatcherQueue struct {
	Name  string
	Queue Job
}

type JobSchedule struct {
	JobName        string
	JobId          string
	Id             string
	ScheduleTyp    string
	ScheduleString string
	Interval       time.Duration
	CronSchedule   cron.Schedule
	CronID         cron.EntryID

	LastRun   time.Time
	NextRun   time.Time
	IsRunning bool
}

type JobSchedules struct {
	Name     string
	Schedule JobSchedule
}

var globalScheduleSet *ScheduleSet = NewScheduleSet()
var globalQueueSet *QueueSet = NewQueueSet()

type ScheduleSet struct {
	Values []JobSchedule
}

func NewScheduleSet() *ScheduleSet {
	return &ScheduleSet{Values: make([]JobSchedule, 0, 200)}
}

func (s *ScheduleSet) Add(str JobSchedule) {
	s.Values = append(s.Values, str)
}

func (s *ScheduleSet) Length() int {
	return len(s.Values)
}

func (s *ScheduleSet) RemoveId(str string) {
	new := s.Values[:0]
	for _, value := range s.Values {
		if value.JobId != str {
			new = append(new, value)
		}
	}
	s.Values = new
}

func (s *ScheduleSet) ContainsId(str string) bool {
	for _, value := range s.Values {
		if value.Id == str {
			return true
		}
	}
	return false
}
func (s *ScheduleSet) GetId(str string) JobSchedule {
	for _, value := range s.Values {
		if value.Id == str {
			return value
		}
	}
	return JobSchedule{}
}
func (s *ScheduleSet) ContainsName(str string) bool {
	for _, value := range s.Values {
		if value.JobName == str {
			return true
		}
	}
	return false
}
func (s *ScheduleSet) GetName(str string) JobSchedule {
	for _, value := range s.Values {
		if value.JobName == str {
			return value
		}
	}
	return JobSchedule{}
}
func (s *ScheduleSet) Update(str JobSchedule) {
	for idx := range s.Values {
		if s.Values[idx].Id == str.Id {
			s.Values[idx] = str
			return
		}
	}
}

func (s *ScheduleSet) Clear() {
	s.Values = nil
	s = nil
}

type QueueSet struct {
	Values []DispatcherQueue
}

func NewQueueSet() *QueueSet {
	return &QueueSet{Values: make([]DispatcherQueue, 0, 200)}
}

func (s *QueueSet) Add(str DispatcherQueue) {
	s.Values = append(s.Values, str)
}

func (s *QueueSet) Length() int {
	return len(s.Values)
}

func (s *QueueSet) RemoveId(str string) {
	new := s.Values[:0]
	for _, value := range s.Values {
		if value.Queue.ID != str {
			new = append(new, value)
		}
	}
	s.Values = new
}

func (s *QueueSet) ContainsName(str string) bool {
	for _, value := range s.Values {
		if value.Name == str {
			return true
		}
	}
	return false
}
func (s *QueueSet) GetName(str string) DispatcherQueue {
	for _, value := range s.Values {
		if value.Name == str {
			return value
		}
	}
	return DispatcherQueue{}
}
func (s *QueueSet) GetId(str string) DispatcherQueue {
	for _, value := range s.Values {
		if value.Queue.ID == str {
			return value
		}
	}
	return DispatcherQueue{}
}
func (s *QueueSet) Update(str DispatcherQueue) {
	for idx := range s.Values {
		if s.Values[idx].Queue.ID == str.Queue.ID {
			s.Values[idx] = str
			return
		}
	}
}

func (s *QueueSet) Clear() {
	s.Values = nil
	s = nil
}

func GetQueues() map[string]DispatcherQueue {
	globalQueue := make(map[string]DispatcherQueue)
	for _, value := range globalQueueSet.Values {
		globalQueue[value.Name] = value
	}
	return globalQueue
}
func GetSchedules() map[string]JobSchedule {
	globalSchedules := make(map[string]JobSchedule)
	for _, value := range globalScheduleSet.Values {
		globalSchedules[value.JobName] = value
	}

	return globalSchedules
}

func updateStartedSchedule(findname string) {
	jobschedule := globalScheduleSet.GetId(findname)
	if jobschedule.Id != "" {
		if jobschedule.ScheduleTyp == "cron" {
			jobschedule.NextRun = jobschedule.CronSchedule.Next(time.Now().In(logger.TimeZone))
			jobschedule.LastRun = time.Now().In(logger.TimeZone)
		} else {
			jobschedule.NextRun = time.Now().In(logger.TimeZone).Add(jobschedule.Interval)
			jobschedule.LastRun = time.Now().In(logger.TimeZone)
		}
		globalScheduleSet.Update(jobschedule)
	}
}
func updateIsRunningSchedule(findname string, isrunning bool) {
	jobschedule := globalScheduleSet.GetId(findname)
	if jobschedule.Id != "" {
		jobschedule.IsRunning = isrunning
		globalScheduleSet.Update(jobschedule)
	}
}
func updateStartedQueue(findname string) {
	que := globalQueueSet.GetId(findname)
	if que.Name != "" {
		que.Queue.Started = time.Now().In(logger.TimeZone)
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
	d.workers = []*Worker{}
	d.tickers = []*DispatchTicker{}
	d.crons = []*DispatchCron{}
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
					updateStartedSchedule(job.SchedulerId)
					go func(jobadd Job) {
						jobChannel := <-d.workerPool
						jobChannel <- jobadd
					}(job)
				} else {
					logger.Log.GlobalLogger.Warn("Skip Job", zap.String("id", job.ID), zap.String("name", job.Name))
					globalQueueSet.RemoveId(job.ID)
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

	for _, value := range globalQueueSet.Values {
		if value.Name == job || value.Name == alt1 || value.Name == alt2 || value.Name == alt3 {
			if !value.Queue.Started.IsZero() {
				return true
			}
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

	d.workers = []*Worker{}
	d.tickers = []*DispatchTicker{}
	d.crons = []*DispatchCron{}
	d.quit <- true
}

var errDispatcherInactive error = errors.New("dispatcher is not active")
var errInvalidCron error = errors.New("invalid cron")

// Dispatch pushes the given job into the job queue.
// The first available worker will perform the job
func (d *Dispatcher) Dispatch(name string, run func()) error {
	if !d.active {
		return errDispatcherInactive
	}
	job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now().In(logger.TimeZone), Name: name, Run: run}
	d.jobQueue <- job

	globalQueueSet.Add(DispatcherQueue{Name: job.Name, Queue: job})
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
		job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now().In(logger.TimeZone), Name: name, Run: run}

		if d.TryEnqueue(job) {
			globalQueueSet.Add(DispatcherQueue{Name: job.Name, Queue: job})
		}

	}()

	return nil
}

// DispatchEvery pushes the given job into the job queue
// continuously at the given interval
func (d *Dispatcher) DispatchEvery(name string, run func(), interval time.Duration) (*DispatchTicker, error) {
	if !d.active {
		return nil, errDispatcherInactive
	}

	t := time.NewTicker(interval)
	schedulerid := uuid.New().String()
	jobid := uuid.New().String()

	globalScheduleSet.Add(JobSchedule{
		JobId:          jobid,
		Id:             schedulerid,
		JobName:        name,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        time.Now().In(logger.TimeZone).Add(interval)})
	dt := &DispatchTicker{ticker: t, quit: make(chan bool), schedulerid: schedulerid}
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
				job := Job{Queue: d.name, ID: jobid, Added: time.Now().In(logger.TimeZone), Name: name, Run: run, SchedulerId: schedulerid}

				if d.TryEnqueue(job) {
					globalQueueSet.Add(DispatcherQueue{Name: job.Name, Queue: job})
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
	dc := &DispatchCron{cron: cron.New(cron.WithSeconds()), schedulerid: schedulerid}
	d.crons = append(d.crons, dc)

	jobid := uuid.New().String()
	cjob, err := dc.cron.AddFunc(cronStr, func() {
		job := Job{Queue: d.name, ID: jobid, Added: time.Now().In(logger.TimeZone), Name: name, Run: run, SchedulerId: schedulerid}

		if d.TryEnqueue(job) {
			globalQueueSet.Add(DispatcherQueue{Name: job.Name, Queue: job})
		}
	})
	if err != nil {
		return errInvalidCron
	}

	globalScheduleSet.Add(JobSchedule{
		JobName:        name,
		JobId:          jobid,
		Id:             schedulerid,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.cron.Entry(cjob).Next,
		CronSchedule:   dc.cron.Entry(cjob).Schedule,
		CronID:         cjob})
	dc.cron.Start()

	return nil
}

// DispatchTicker represents a dispatched job ticker
// that executes on a given interval. This provides
// a means for stopping the execution cycle from continuing.
type DispatchTicker struct {
	ticker      *time.Ticker
	schedulerid string
	quit        chan bool
}

// Stop ends the execution cycle for the given ticker.
func (dt *DispatchTicker) stop() {
	dt.ticker.Stop()
	globalScheduleSet.RemoveId(dt.schedulerid)
	dt.quit <- true
}

// DispatchCron represents a dispatched cron job
// that executes using cron expression formats.
type DispatchCron struct {
	cron        *cron.Cron
	schedulerid string
}

// Stops ends the execution cycle for the given cron.
func (c *DispatchCron) Stop() {
	c.cron.Stop()
	globalScheduleSet.RemoveId(c.schedulerid)
}
