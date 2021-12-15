package tasks

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

//Source: https://github.com/mborders/artifex
//Several Mods

// Dispatcher maintains a pool for available workers
// and a job queue that workers will process
type Dispatcher struct {
	maxWorkers int
	maxQueue   int
	workers    []*Worker
	Tickers    []*DispatchTicker
	Crons      []*DispatchCron
	workerPool chan chan Job
	jobQueue   chan Job

	quit          chan bool
	active        bool
	name          string
	DispatchQueue DispatcherQueue
}

type DispatcherQueue struct {
	Queue map[string]Job
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
	Schedule map[string]JobSchedule
}

var Mu sync.Mutex
var GlobalQueue DispatcherQueue
var GlobalSchedules JobSchedules

func (d *JobSchedules) AddQueue(job JobSchedule) {
	Mu.Lock()
	defer Mu.Unlock()
	d.Schedule[job.Id] = job
}
func (d *JobSchedules) RemoveQueue(job JobSchedule) {
	Mu.Lock()
	defer Mu.Unlock()
	delete(d.Schedule, job.Id)
}
func UpdateStartedSchedule(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	findname := job.SchedulerId
	if _, ok := GlobalSchedules.Schedule[findname]; ok {
		jobschedule := GlobalSchedules.Schedule[findname]
		if jobschedule.LastRun.IsZero() {
			jobschedule.LastRun = time.Now()
		}
		if jobschedule.ScheduleTyp == "cron" {
			jobschedule.NextRun = jobschedule.CronSchedule.Next(time.Now())
			jobschedule.LastRun = time.Now()
		} else {
			jobschedule.NextRun = jobschedule.LastRun.Add(jobschedule.Interval)
			jobschedule.LastRun = time.Now()
		}
		GlobalSchedules.Schedule[findname] = jobschedule
	}
}
func UpdateIsRunningSchedule(job Job, isrunning bool) {
	Mu.Lock()
	defer Mu.Unlock()
	findname := job.SchedulerId
	if _, ok := GlobalSchedules.Schedule[findname]; ok {
		jobschedule := GlobalSchedules.Schedule[findname]
		jobschedule.IsRunning = isrunning
		GlobalSchedules.Schedule[findname] = jobschedule
	}
}
func (d *DispatcherQueue) AddQueue(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	d.Queue[job.ID] = job
}
func (d *DispatcherQueue) RemoveQueue(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	delete(d.Queue, job.ID)
}
func (d *DispatcherQueue) UpdateStartedQueue(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	job.Started = time.Now()
	d.Queue[job.ID] = job
}

// NewDispatcher creates a new dispatcher with the given
// number of workers and buffers the job queue based on maxQueue.
// It also initializes the channels for the worker pool and job queue
func NewDispatcher(name string, maxWorkers int, maxQueue int) *Dispatcher {
	if GlobalQueue.Queue == nil {
		GlobalQueue.Queue = make(map[string]Job)
	}
	if GlobalSchedules.Schedule == nil {
		GlobalSchedules.Schedule = make(map[string]JobSchedule)
	}
	return &Dispatcher{
		name:       name,
		maxWorkers: maxWorkers,
		maxQueue:   maxQueue,
	}
}

// Start creates and starts workers, adding them to the worker pool.
// Then, it starts a select loop to wait for job to be dispatched
// to available workers
func (d *Dispatcher) Start() {
	d.workers = []*Worker{}
	d.Tickers = []*DispatchTicker{}
	d.Crons = []*DispatchCron{}
	d.workerPool = make(chan chan Job, d.maxWorkers)
	d.jobQueue = make(chan Job, d.maxQueue)
	d.DispatchQueue.Queue = make(map[string]Job, d.maxQueue)
	d.quit = make(chan bool)

	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(d.workerPool)
		worker.Start()
		d.workers = append(d.workers, worker)
	}

	d.active = true

	go func() {
		for {
			select {
			case job := <-d.jobQueue:
				if !CheckQueue(job.Name) {
					UpdateStartedSchedule(job)
					go func(job Job) {
						jobChannel := <-d.workerPool
						jobChannel <- job
						d.DispatchQueue.RemoveQueue(job)
					}(job)
				} else {
					d.DispatchQueue.RemoveQueue(job)
				}
			case <-d.quit:
				return
			}
		}
	}()
}

func CheckQueue(job string) bool {
	alternatequeuejobnames := make([]string, 0, 3)
	if strings.HasPrefix(job, "searchmissinginc_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginc_", "searchmissinginctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginc_", "searchmissingfull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginc_", "searchmissingfulltitle_", 1))
	} else if strings.HasPrefix(job, "searchmissinginctitle_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginctitle_", "searchmissinginc_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginctitle_", "searchmissingfull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissinginctitle_", "searchmissingfulltitle_", 1))
	} else if strings.HasPrefix(job, "searchmissingfull_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfull_", "searchmissinginctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfull_", "searchmissinginc_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfull_", "searchmissingfulltitle_", 1))
	} else if strings.HasPrefix(job, "searchmissingfulltitle_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfulltitle_", "searchmissinginctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfulltitle_", "searchmissingfull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchmissingfulltitle_", "searchmissinginc_", 1))
	} else if strings.HasPrefix(job, "searchupgradeinc_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinc_", "searchupgradeinctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinc_", "searchupgradefull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinc_", "searchupgradefulltitle_", 1))
	} else if strings.HasPrefix(job, "searchupgradeinctitle_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinctitle_", "searchupgradeinc_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinctitle_", "searchupgradefull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradeinctitle_", "searchupgradefulltitle_", 1))
	} else if strings.HasPrefix(job, "searchupgradefull_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefull_", "searchupgradeinctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefull_", "searchupgradeinc_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefull_", "searchupgradefulltitle_", 1))
	} else if strings.HasPrefix(job, "searchupgradefulltitle_") {
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefulltitle_", "searchupgradeinctitle_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefulltitle_", "searchupgradefull_", 1))
		alternatequeuejobnames = append(alternatequeuejobnames, strings.Replace(job, "searchupgradefulltitle_", "searchupgradeinc_", 1))
	}
	Mu.Lock()
	defer Mu.Unlock()
	for _, value := range GlobalQueue.Queue {
		if value.Name == job && !value.Started.IsZero() {
			return true
		}
		if len(alternatequeuejobnames) >= 1 {
			for idx := range alternatequeuejobnames {
				if value.Name == alternatequeuejobnames[idx] && !value.Started.IsZero() {
					return true
				}
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
		d.workers[i].Stop()
	}

	for i := range d.Tickers {
		d.Tickers[i].Stop()
	}

	for i := range d.Crons {
		d.Crons[i].Stop()
	}

	d.workers = []*Worker{}
	d.Tickers = []*DispatchTicker{}
	d.Crons = []*DispatchCron{}
	d.quit <- true
}

// Dispatch pushes the given job into the job queue.
// The first available worker will perform the job
func (d *Dispatcher) Dispatch(name string, run func()) error {
	if !d.active {
		return errors.New("dispatcher is not active")
	}
	job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now(), Name: name, Run: run}
	d.jobQueue <- job
	GlobalQueue.AddQueue(job)
	d.DispatchQueue.AddQueue(job)
	return nil
}

// DispatchIn pushes the given job into the job queue
// after the given duration has elapsed
func (d *Dispatcher) DispatchIn(name string, run func(), duration time.Duration) error {
	if !d.active {
		return errors.New("dispatcher is not active")
	}

	go func() {
		time.Sleep(duration)
		job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now(), Name: name, Run: run}
		d.jobQueue <- job
		GlobalQueue.AddQueue(job)
		d.DispatchQueue.AddQueue(job)
	}()

	return nil
}

// DispatchEvery pushes the given job into the job queue
// continuously at the given interval
func (d *Dispatcher) DispatchEvery(name string, run func(), interval time.Duration) (*DispatchTicker, error) {
	if !d.active {
		return nil, errors.New("dispatcher is not active")
	}

	t := time.NewTicker(interval)
	schedulerid := uuid.New().String()
	jobid := uuid.New().String()
	GlobalSchedules.AddQueue(JobSchedule{
		JobId:          jobid,
		Id:             schedulerid,
		JobName:        name,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        time.Now().Add(interval)})

	dt := &DispatchTicker{Ticker: t, quit: make(chan bool), schedulerid: schedulerid}
	d.Tickers = append(d.Tickers, dt)

	go func() {
		for {
			select {
			case <-t.C:
				job := Job{Queue: d.name, ID: jobid, Added: time.Now(), Name: name, Run: run, SchedulerId: schedulerid}
				d.jobQueue <- job
				GlobalQueue.AddQueue(job)
				d.DispatchQueue.AddQueue(job)
			case <-dt.quit:
				return
			}
		}
	}()

	return dt, nil
}

// DispatchEvery pushes the given job into the job queue
// each time the cron definition is met
func (d *Dispatcher) DispatchCron(name string, run func(), cronStr string) (*DispatchCron, error) {
	if !d.active {
		return nil, errors.New("dispatcher is not active")
	}

	schedulerid := uuid.New().String()
	dc := &DispatchCron{Cron: cron.New(cron.WithSeconds()), schedulerid: schedulerid}
	d.Crons = append(d.Crons, dc)

	jobid := uuid.New().String()
	cjob, err := dc.Cron.AddFunc(cronStr, func() {
		job := Job{Queue: d.name, ID: jobid, Added: time.Now(), Name: name, Run: run, SchedulerId: schedulerid}
		d.jobQueue <- job
		GlobalQueue.AddQueue(job)
		d.DispatchQueue.AddQueue(job)
	})
	if err != nil {
		return nil, errors.New("invalid cron definition")
	}

	dc.Cron.Start()

	GlobalSchedules.AddQueue(JobSchedule{
		JobName:        name,
		JobId:          jobid,
		Id:             schedulerid,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.Cron.Entry(cjob).Next,
		CronSchedule:   dc.Cron.Entry(cjob).Schedule,
		CronID:         cjob})

	return dc, nil
}

// DispatchTicker represents a dispatched job ticker
// that executes on a given interval. This provides
// a means for stopping the execution cycle from continuing.
type DispatchTicker struct {
	Ticker      *time.Ticker
	schedulerid string
	quit        chan bool
}

// Stop ends the execution cycle for the given ticker.
func (dt *DispatchTicker) Stop() {
	dt.Ticker.Stop()
	GlobalSchedules.RemoveQueue(JobSchedule{Id: dt.schedulerid})
	dt.quit <- true
}

// DispatchCron represents a dispatched cron job
// that executes using cron expression formats.
type DispatchCron struct {
	Cron        *cron.Cron
	schedulerid string
}

// Stops ends the execution cycle for the given cron.
func (c *DispatchCron) Stop() {
	c.Cron.Stop()
}
func (c *DispatchCron) Start() {
	c.Cron.Start()
}
func (c *DispatchCron) Remove(id cron.EntryID) {
	c.Cron.Remove(id)
	GlobalSchedules.RemoveQueue(JobSchedule{Id: c.schedulerid})
}
