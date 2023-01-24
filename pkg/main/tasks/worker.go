package tasks

//Source: https://github.com/mborders/artifex

// Worker attaches to a provided worker pool, and
// looks for jobs on its job channel
type worker struct {
	workerPool chan chan Job
	jobChannel chan Job
	quit       chan bool
}

// NewWorker creates a new worker using the given id and
// attaches to the provided worker pool. It also initializes
// the job/quit channels
func NewWorker(workerPool chan chan Job) *worker {
	return &worker{
		workerPool: workerPool,
		jobChannel: make(chan Job),
		quit:       make(chan bool),
	}
}

// Start initializes a select loop to listen for jobs to execute
func (w *worker) start() {
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
					updateStartedQueue(job.ID)
					updateIsRunningSchedule(job.SchedulerID, true)
					job.Run()
					updateIsRunningSchedule(job.SchedulerID, false)
					globalQueueSet.RemoveID(job.ID)
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
func (w *worker) stop() {
	go func() {
		w.quit <- true
	}()
}
