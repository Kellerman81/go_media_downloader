package tasks

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

//Source: https://github.com/mborders/artifex

// Dispatcher maintains a pool for available workers
// and a job queue that workers will process
type Dispatcher struct {
	maxWorkers int
	maxQueue   int
	workers    []*Worker
	tickers    []*DispatchTicker
	crons      []*DispatchCron
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

var Mu sync.Mutex
var GlobalQueue DispatcherQueue

func (d DispatcherQueue) AddQueue(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	d.Queue[job.ID] = job
}
func (d DispatcherQueue) RemoveQueue(job Job) {
	Mu.Lock()
	defer Mu.Unlock()
	delete(d.Queue, job.ID)
}
func (d DispatcherQueue) UpdateStartedQueue(job Job) {
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
	d.tickers = []*DispatchTicker{}
	d.crons = []*DispatchCron{}
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
				go func(job Job) {
					jobChannel := <-d.workerPool
					jobChannel <- job
					d.DispatchQueue.RemoveQueue(job)
				}(job)
			case <-d.quit:
				return
			}
		}
	}()
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

	for i := range d.tickers {
		d.tickers[i].Stop()
	}

	for i := range d.crons {
		d.crons[i].Stop()
	}

	d.workers = []*Worker{}
	d.tickers = []*DispatchTicker{}
	d.crons = []*DispatchCron{}
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
	dt := &DispatchTicker{ticker: t, quit: make(chan bool)}
	d.tickers = append(d.tickers, dt)

	go func() {
		for {
			select {
			case <-t.C:
				job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now(), Name: name, Run: run}
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

	dc := &DispatchCron{cron: cron.New(cron.WithSeconds())}
	d.crons = append(d.crons, dc)

	_, err := dc.cron.AddFunc(cronStr, func() {
		job := Job{Queue: d.name, ID: uuid.New().String(), Added: time.Now(), Name: name, Run: run}
		d.jobQueue <- job
		GlobalQueue.AddQueue(job)
		d.DispatchQueue.AddQueue(job)
	})

	if err != nil {
		return nil, errors.New("invalid cron definition")
	}

	dc.cron.Start()
	return dc, nil
}

// DispatchTicker represents a dispatched job ticker
// that executes on a given interval. This provides
// a means for stopping the execution cycle from continuing.
type DispatchTicker struct {
	ticker *time.Ticker
	quit   chan bool
}

// Stop ends the execution cycle for the given ticker.
func (dt *DispatchTicker) Stop() {
	dt.ticker.Stop()
	dt.quit <- true
}

// DispatchCron represents a dispatched cron job
// that executes using cron expression formats.
type DispatchCron struct {
	cron *cron.Cron
}

// Stops ends the execution cycle for the given cron.
func (c *DispatchCron) Stop() {
	c.cron.Stop()
}
