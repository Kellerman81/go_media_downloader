package pool

type Poolobj[T any] struct {
	//objs is a channel of type T
	objs chan *T
	//Function will be run on Get() - include here your logic to create the initial object
	constructor func(*T)
	//Function will be run on Put() - include here your logic to reset the object
	destructor func(*T)
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
func NewPool[T any](maxsize int, initcreate int, constructor func(*T), destructor func(*T)) Poolobj[T] {
	var a Poolobj[T]
	a.constructor = constructor
	a.objs = make(chan *T, maxsize)
	if initcreate > 0 {
		for i := 0; i < initcreate; i++ {
			var bo T
			if a.constructor != nil {
				a.constructor(&bo)
			}
			a.objs <- &bo
		}
	}
	a.destructor = destructor
	return a
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects.
func (p *Poolobj[T]) Get() *T {
	if len(p.objs) >= 1 {
		return <-p.objs
	}
	var bo T
	if p.constructor != nil {
		p.constructor(&bo)
	}
	return &bo
}

// Put returns an object to the pool.
// If the pool is not at capacity, it calls the destructor function if provided,
// then sends the object back on the channel.
func (p *Poolobj[T]) Put(bo *T) {
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
