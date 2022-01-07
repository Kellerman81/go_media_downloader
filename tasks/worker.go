package tasks

import "runtime/debug"

//Source: https://github.com/mborders/artifex

// Worker attaches to a provided worker pool, and
// looks for jobs on its job channel
type Worker struct {
	workerPool chan chan Job
	jobChannel chan Job
	quit       chan bool
}

// NewWorker creates a new worker using the given id and
// attaches to the provided worker pool. It also initializes
// the job/quit channels
func NewWorker(workerPool chan chan Job) *Worker {
	return &Worker{
		workerPool: workerPool,
		jobChannel: make(chan Job),
		quit:       make(chan bool),
	}
}

// Start initializes a select loop to listen for jobs to execute
func (w *Worker) start() {
	go func() {
		for {
			w.workerPool <- w.jobChannel

			select {
			case job := <-w.jobChannel:
				GlobalQueue.updateStartedQueue(job)
				updateIsRunningSchedule(job, true)
				job.Run()
				updateIsRunningSchedule(job, false)
				GlobalQueue.removeQueue(job)
				debug.FreeOSMemory()
			case <-w.quit:
				return
			}
		}
	}()
}

// Stop will end the job select loop for the worker
func (w *Worker) stop() {
	go func() {
		w.quit <- true
	}()
}
