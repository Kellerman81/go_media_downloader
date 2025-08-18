package pool

import (
	"sync"
)

type Poolobj[t any] struct {
	// objs is a channel of type T
	objs chan *t
	// pool sync.Pool
	// Function will be run on Get() - include here your logic to create the initial object
	constructor func(*t)
	// Function will be run on Put() - include here your logic to reset the object
	destructor func(*t) bool
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects. Uses non-blocking channel operation to avoid race conditions.
func (p *Poolobj[t]) Get() *t {
	select {
	case obj := <-p.objs:
		return obj
	default:
		return p.NewObj()
	}
}

// NewObj creates a new object of type T, optionally initializing it using the pool's constructor function.
// If a constructor is defined, it is called with a pointer to the newly created object.
// Returns a pointer to the newly created object.
func (p *Poolobj[t]) NewObj() *t {
	var bo t
	if p.constructor != nil {
		p.constructor(&bo)
	}
	return &bo
}

// Put returns an object to the pool.
// If the pool is not at capacity, it calls the destructor function if provided,
// then sends the object back on the channel. Uses non-blocking channel operation to avoid race conditions.
func (p *Poolobj[t]) Put(bo *t) bool {
	if bo == nil {
		return false
	}

	// Call destructor if provided
	if p.destructor != nil {
		if p.destructor(bo) {
			return false
		}
	}

	// Try to put object back in pool using non-blocking send
	select {
	case p.objs <- bo:
		return true
	default:
		return false // Pool is full
	}
}

// Init initializes the Poolobj by setting the constructor and destructor functions,
// creating the object channel with a specified capacity, and optionally creating
// and adding the initial set of objects to the pool using the provided constructor.
func (p *Poolobj[t]) Init(maxsize, initcreate int, constructor func(*t), destructor func(*t) bool) {
	if maxsize <= 0 {
		maxsize = 200 // default capacity
	}
	
	p.constructor = constructor
	p.destructor = destructor

	p.objs = make(chan *t, maxsize)
	if initcreate > 0 {
		for range initcreate {
			p.Put(p.NewObj())
		}
	}
}

// NewPool creates a new Poolobj initialized with the given parameters.
//
// maxsize specifies the maximum number of objects that can be kept in the
// pool.
//
// initcreate specifies the initial number of objects to create in the pool
// on startup.
//
// constructor, if non-nil, is called whenever a new object needs to be
// created.
//
// destructor, if non-nil, is called whenever an object is removed from
// the pool.
func NewPool[t any](
	maxsize, initcreate int,
	constructor func(*t),
	destructor func(*t) bool,
) *Poolobj[t] {
	p := &Poolobj[t]{}
	p.Init(maxsize, initcreate, constructor, destructor)
	return p
}

type SizedWaitGroup struct {
	wg      sync.WaitGroup
	current chan struct{}
	Size    int
}

// NewSizedGroup creates a new SizedWaitGroup with the specified limit.
// If the limit is less than or equal to 0, it is set to 1.
// The SizedWaitGroup has a channel to limit the number of concurrent operations,
// and a sync.WaitGroup to track the completion of all operations.
func NewSizedGroup(limit int) SizedWaitGroup {
	if limit <= 0 {
		limit = 1
	}
	return SizedWaitGroup{
		Size:    limit,
		current: make(chan struct{}, limit),
		wg:      sync.WaitGroup{},
	}
}

// Add increments the SizedWaitGroup counter by one. It also adds a token to the
// current channel, which limits the number of concurrent operations.
func (s *SizedWaitGroup) Add() {
	s.current <- struct{}{}
	s.wg.Add(1)
}

// Done decrements the SizedWaitGroup counter by one. It also removes a token from the
// current channel, which limits the number of concurrent operations.
func (s *SizedWaitGroup) Done() {
	<-s.current
	s.wg.Done()
}

// Wait blocks until all operations added to the SizedWaitGroup have completed.
func (s *SizedWaitGroup) Wait() {
	s.wg.Wait()
}

// Close resets the SizedWaitGroup to its initial state, allowing it to be reused.
// Note: This should only be called after Wait() has completed to avoid goroutine leaks.
func (s *SizedWaitGroup) Close() {
	*s = SizedWaitGroup{}
}
