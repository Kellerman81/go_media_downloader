package tasks

import (
	"time"
)

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
		// defer func() { // recovers panic
		// 	if e := recover(); e != nil {
		// 		fmt.Println("Recovered from panic (worker) ", e)
		// 	}
		// }()
		for {
			w.workerPool <- w.jobChannel
			ret := func() bool {
				select {
				case job := <-w.jobChannel:
					func(job Job) {
						updateStartedQueue(job.ID)
						updateIsRunningSchedule(job.SchedulerId, true)
						job.Run()
						updateIsRunningSchedule(job.SchedulerId, false)
						time.Sleep(time.Duration(2) * time.Second)

						taskmu.Lock()
						delete(globalQueue, job.Name)
						taskmu.Unlock()
					}(job)
					return false
				case <-w.quit:
					return true
				}
			}()
			if ret {
				break
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
