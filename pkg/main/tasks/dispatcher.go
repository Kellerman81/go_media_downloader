package tasks

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
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
	//Cron       *DispatchCron
	workerPool chan chan Job
	jobQueue   chan Job

	quit          chan bool
	active        bool
	name          string
	DispatchQueue []DispatcherQueue
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

var globalQueue map[string]DispatcherQueue
var globalSchedules map[string]JobSchedule
var taskmu *sync.Mutex

func GetQueues() map[string]DispatcherQueue {
	return globalQueue
}
func GetSchedules() map[string]JobSchedule {
	return globalSchedules
}
func checkSchedule(find string) bool {
	taskmu.Lock()
	defer taskmu.Unlock()
	_, ok := globalSchedules[find]
	return ok
}

func getSchedule(find string) JobSchedule {
	taskmu.Lock()
	defer taskmu.Unlock()
	sched, _ := globalSchedules[find]
	return sched
}

func updateStartedSchedule(findname string) {
	taskmu.Lock()
	defer taskmu.Unlock()
	jobschedule := globalSchedules[findname]
	if jobschedule.Id != "" {
		if jobschedule.ScheduleTyp == "cron" {
			jobschedule.NextRun = jobschedule.CronSchedule.Next(time.Now())
			jobschedule.LastRun = time.Now()
		} else {
			jobschedule.NextRun = time.Now().Add(jobschedule.Interval)
			jobschedule.LastRun = time.Now()
		}
		globalSchedules[findname] = jobschedule
	}
}
func updateIsRunningSchedule(findname string, isrunning bool) {
	taskmu.Lock()
	defer taskmu.Unlock()
	jobschedule := globalSchedules[findname]
	if jobschedule.Id != "" {
		jobschedule.IsRunning = isrunning
		globalSchedules[findname] = jobschedule
	}
}
func updateStartedQueue(findname string) {
	taskmu.Lock()
	defer taskmu.Unlock()
	que := globalQueue[findname]
	if que.Name != "" {
		que.Queue.Started = time.Now()
		globalQueue[findname] = que
	}
}

// NewDispatcher creates a new dispatcher with the given
// number of workers and buffers the job queue based on maxQueue.
// It also initializes the channels for the worker pool and job queue
func NewDispatcher(name string, maxWorkers int, maxQueue int) *Dispatcher {
	if taskmu == nil {
		taskmu = &sync.Mutex{}
	}
	taskmu.Lock()
	if globalQueue == nil {
		globalQueue = make(map[string]DispatcherQueue)
	}
	if globalSchedules == nil {
		globalSchedules = make(map[string]JobSchedule)
	}
	taskmu.Unlock()
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
		defer func() { // recovers panic
			if e := recover(); e != nil {
				fmt.Println("Recovered from panic (dispatcher1) ", e)
			}
		}()
		for {
			select {
			case job := <-d.jobQueue:
				if !checkQueue(job.Name) {
					updateStartedSchedule(job.SchedulerId)
					go func(job Job) {
						defer func() { // recovers panic
							if e := recover(); e != nil {
								fmt.Println("Recovered from panic (dispatcher2) ", e)
							}
						}()
						jobChannel := <-d.workerPool
						jobChannel <- job
					}(job)
				} else {
					taskmu.Lock()
					delete(globalQueue, job.ID)
					taskmu.Unlock()
				}
			case <-d.quit:
				return
			}
		}
	}()
}

func checkQueue(job string) bool {
	alternatequeuejobnames := make([]string, 0, 3)
	defer logger.ClearVar(&alternatequeuejobnames)
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

	taskmu.Lock()
	defer taskmu.Unlock()
	for _, value := range globalQueue {
		if value.Name == job && !value.Queue.Started.IsZero() {
			return true
		}
		if len(alternatequeuejobnames) >= 1 {
			for idx := range alternatequeuejobnames {
				if value.Name == alternatequeuejobnames[idx] && !value.Queue.Started.IsZero() {
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
		d.workers[i].stop()
	}

	for i := range d.Tickers {
		d.Tickers[i].stop()
	}

	for i := range d.Crons {
		d.Crons[i].stop()
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

	taskmu.Lock()
	globalQueue[job.ID] = DispatcherQueue{Name: job.Name, Queue: job}
	taskmu.Unlock()
	return nil
}

// DispatchIn pushes the given job into the job queue
// after the given duration has elapsed
func (d *Dispatcher) DispatchIn(name string, run func(), duration time.Duration) error {
	if !d.active {
		return errors.New("dispatcher is not active")
	}

	go func() {
		defer func() { // recovers panic
			if e := recover(); e != nil {
				fmt.Println("Recovered from panic (dispatcher3) ", e)
			}
		}()
		time.Sleep(duration)
		job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now(), Name: name, Run: run}
		d.jobQueue <- job

		taskmu.Lock()
		globalQueue[job.ID] = DispatcherQueue{Name: job.Name, Queue: job}
		taskmu.Unlock()
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

	taskmu.Lock()
	globalSchedules[schedulerid] = JobSchedule{
		JobId:          jobid,
		Id:             schedulerid,
		JobName:        name,
		ScheduleTyp:    "interval",
		ScheduleString: interval.String(),
		LastRun:        time.Time{},
		Interval:       interval,
		NextRun:        time.Now().Add(interval)}
	taskmu.Unlock()
	dt := &DispatchTicker{Ticker: t, quit: make(chan bool), schedulerid: schedulerid}
	d.Tickers = append(d.Tickers, dt)

	go func() {
		defer func() { // recovers panic
			if e := recover(); e != nil {
				fmt.Println("Recovered from panic (dispatcher4) ", e)
			}
		}()
		for {
			select {
			case <-t.C:
				job := Job{Queue: d.name, ID: jobid, Added: time.Now(), Name: name, Run: run, SchedulerId: schedulerid}
				d.jobQueue <- job

				taskmu.Lock()
				globalQueue[job.ID] = DispatcherQueue{Name: job.Name, Queue: job}
				taskmu.Unlock()
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
		job := Job{Queue: d.name, ID: jobid, Added: time.Now(), Name: name, Run: run, SchedulerId: dc.schedulerid}
		d.jobQueue <- job
		taskmu.Lock()
		globalQueue[job.ID] = DispatcherQueue{Name: job.Name, Queue: job}
		taskmu.Unlock()
	})
	if err != nil {
		return nil, errors.New("invalid cron definition")
	}
	dc.start()

	taskmu.Lock()
	defer taskmu.Unlock()
	globalSchedules[dc.schedulerid] = JobSchedule{
		JobName:        name,
		JobId:          jobid,
		Id:             dc.schedulerid,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.Cron.Entry(cjob).Next,
		CronSchedule:   dc.Cron.Entry(cjob).Schedule,
		CronID:         cjob}

	return dc, nil
}

// DispatchEvery pushes the given job into the job queue
// each time the cron definition is met
func (d *Dispatcher) DispatchCronOld(name string, run func(), cronStr string) (*DispatchCron, error) {
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
		taskmu.Lock()
		globalQueue[job.ID] = DispatcherQueue{Name: job.Name, Queue: job}
		taskmu.Unlock()
	})
	if err != nil {
		return nil, errors.New("invalid cron definition")
	}

	dc.Cron.Start()

	taskmu.Lock()
	defer taskmu.Unlock()
	globalSchedules[schedulerid] = JobSchedule{
		JobName:        name,
		JobId:          jobid,
		Id:             schedulerid,
		ScheduleTyp:    "cron",
		ScheduleString: cronStr,
		LastRun:        time.Time{},
		NextRun:        dc.Cron.Entry(cjob).Next,
		CronSchedule:   dc.Cron.Entry(cjob).Schedule,
		CronID:         cjob}

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
func (dt *DispatchTicker) stop() {
	dt.Ticker.Stop()
	taskmu.Lock()
	defer taskmu.Unlock()
	delete(globalSchedules, dt.schedulerid)
	dt.quit <- true
}

// DispatchCron represents a dispatched cron job
// that executes using cron expression formats.
type DispatchCron struct {
	Cron        *cron.Cron
	schedulerid string
}

// Stops ends the execution cycle for the given cron.
func (c *DispatchCron) stop() {
	c.Cron.Stop()
}
func (c *DispatchCron) start() {
	c.Cron.Start()
}
func (c *DispatchCron) remove(id cron.EntryID) {
	c.Cron.Remove(id)
	taskmu.Lock()
	defer taskmu.Unlock()
	delete(globalSchedules, c.schedulerid)
}
