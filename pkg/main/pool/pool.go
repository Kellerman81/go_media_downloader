package pool

import (
	"fmt"
	"sync"
)

type Poolobj[t any] struct {
	//objs is a channel of type T
	objs chan *t
	//Function will be run on Get() - include here your logic to create the initial object
	constructor func(*t)
	//Function will be run on Put() - include here your logic to reset the object
	destructor func(*t)
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects.
func (p *Poolobj[t]) Get() *t {
	if len(p.objs) >= 1 {
		return <-p.objs
	}
	var bo t
	fmt.Printf("Creating new pool object %T", bo)
	//fmt.Println(reflect.TypeOf(bo))
	if p.constructor != nil {
		p.constructor(&bo)
	}
	return &bo
}

// Put returns an object to the pool.
// If the pool is not at capacity, it calls the destructor function if provided,
// then sends the object back on the channel.
func (p *Poolobj[t]) Put(bo *t) {
	if bo == nil {
		return
	}
	if len(p.objs) < cap(p.objs) {
		if p.destructor != nil {
			p.destructor(bo)
		}
		p.objs <- bo
	}
}

// func Clear(bo any) {
// 	reflect.ValueOf(bo).Elem().SetZero()
// }

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
func NewPool[t any](maxsize int, initcreate int, constructor func(*t), destructor func(*t)) Poolobj[t] {
	var a Poolobj[t]
	a.constructor = constructor
	a.objs = make(chan *t, maxsize)
	if initcreate > 0 {
		for range initcreate {
			var bo t
			if a.constructor != nil {
				a.constructor(&bo)
			}
			a.objs <- &bo
		}
	}
	a.destructor = destructor
	return a
}

type SizedWaitGroup struct {
	Size    int
	current chan struct{}
	wg      sync.WaitGroup
}

func NewSizedGroup(limit int) *SizedWaitGroup {
	return &SizedWaitGroup{
		Size:    limit,
		current: make(chan struct{}, limit),
		wg:      sync.WaitGroup{},
	}
}
func (s *SizedWaitGroup) Add() {
	s.current <- struct{}{}
	s.wg.Add(1)
}

func (s *SizedWaitGroup) Done() {
	<-s.current
	s.wg.Done()
}

func (s *SizedWaitGroup) Wait() {
	s.wg.Wait()
}
func (s *SizedWaitGroup) Close() {
	s.current = nil
	*s = SizedWaitGroup{}
}
