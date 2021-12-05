package tasks

import "time"

//Source: https://github.com/mborders/artifex

// Job represents a runnable process, where Start
// will be executed by a worker via the dispatch queue
type Job struct {
	Queue       string
	ID          string
	Added       time.Time
	Started     time.Time
	Name        string
	SchedulerId string
	Run         func() `json:"-"`
}
