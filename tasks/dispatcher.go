package tasks

import (
	"errors"
	"strings"
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

var GlobalQueue []DispatcherQueue
var GlobalSchedules []JobSchedules

func findAndDeleteQueue(item string) {
	new := GlobalQueue[:0]
	for _, i := range GlobalQueue {
		if i.Name != item {
			new = append(new, i)
		}
	}
	GlobalQueue = new
}

func findAndDeleteSchedule(item string) {
	new := GlobalSchedules[:0]
	for _, i := range GlobalSchedules {
		if i.Name != item {
			new = append(new, i)
		}
	}
	GlobalSchedules = new
}

func checkSchedule(find string) bool {
	for idx := range GlobalSchedules {
		if GlobalSchedules[idx].Name == find {
			return true
		}
	}
	return false
}

func getSchedule(find string) JobSchedules {
	for idx := range GlobalSchedules {
		if GlobalSchedules[idx].Name == find {
			return GlobalSchedules[idx]
		}
	}
	return JobSchedules{}
}

func updateSchedule(find string, schedule JobSchedule) {
	for idx := range GlobalSchedules {
		if GlobalSchedules[idx].Name == find {
			GlobalSchedules[idx].Schedule = schedule
		}
	}
}
func addScheduleQueue(job JobSchedule) {
	GlobalSchedules = append(GlobalSchedules, JobSchedules{Name: job.Id, Schedule: job})
}
func removeScheduleQueue(job JobSchedule) {
	findAndDeleteSchedule(job.Id)
}
func updateStartedSchedule(job Job) {
	findname := job.SchedulerId
	if checkSchedule(findname) {
		jobschedule := getSchedule(findname).Schedule
		if jobschedule.ScheduleTyp == "cron" {
			jobschedule.NextRun = jobschedule.CronSchedule.Next(time.Now())
			jobschedule.LastRun = time.Now()
		} else {
			jobschedule.NextRun = time.Now().Add(jobschedule.Interval)
			jobschedule.LastRun = time.Now()
		}
		updateSchedule(findname, jobschedule)
	}
}
func updateIsRunningSchedule(job Job, isrunning bool) {
	findname := job.SchedulerId
	if checkSchedule(findname) {
		jobschedule := getSchedule(findname).Schedule
		jobschedule.IsRunning = isrunning
		updateSchedule(findname, jobschedule)
	}
}
func (d *Dispatcher) addDispatcherQueue(job Job) {
	d.DispatchQueue = append(d.DispatchQueue, DispatcherQueue{Name: job.Name, Queue: job})
}
func (d *Dispatcher) removeDispatcherQueue(job Job) {
	new := d.DispatchQueue[:0]
	for _, i := range d.DispatchQueue {
		if i.Name != job.Name {
			new = append(new, i)
		}
	}
	d.DispatchQueue = new
}
func updateStartedQueue(job Job) {
	for idx := range GlobalQueue {
		if GlobalQueue[idx].Name == job.Name {
			GlobalQueue[idx].Queue.Started = time.Now()
		}
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
	d.DispatchQueue = []DispatcherQueue{}
	d.quit = make(chan bool)

	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(d.workerPool)
		worker.start()
		d.workers = append(d.workers, worker)
	}

	d.active = true

	go func() {
		for {
			select {
			case job := <-d.jobQueue:
				if !checkQueue(job.Name) {
					updateStartedSchedule(job)
					go func(job Job) {
						jobChannel := <-d.workerPool
						jobChannel <- job
						d.removeDispatcherQueue(job)
					}(job)
				} else {
					d.removeDispatcherQueue(job)
					findAndDeleteQueue(job.Name)
				}
			case <-d.quit:
				return
			}
		}
	}()
}

func checkQueue(job string) bool {
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
	for _, value := range GlobalQueue {
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
	GlobalQueue = append(GlobalQueue, DispatcherQueue{Name: job.Name, Queue: job})
	d.addDispatcherQueue(job)
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
		GlobalQueue = append(GlobalQueue, DispatcherQueue{Name: job.Name, Queue: job})
		d.addDispatcherQueue(job)
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
	addScheduleQueue(JobSchedule{
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
				GlobalQueue = append(GlobalQueue, DispatcherQueue{Name: job.Name, Queue: job})
				d.addDispatcherQueue(job)
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
		GlobalQueue = append(GlobalQueue, DispatcherQueue{Name: job.Name, Queue: job})
		d.addDispatcherQueue(job)
	})
	if err != nil {
		return nil, errors.New("invalid cron definition")
	}

	dc.Cron.Start()

	addScheduleQueue(JobSchedule{
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
func (dt *DispatchTicker) stop() {
	dt.Ticker.Stop()
	removeScheduleQueue(JobSchedule{Id: dt.schedulerid})
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
	removeScheduleQueue(JobSchedule{Id: c.schedulerid})
}
