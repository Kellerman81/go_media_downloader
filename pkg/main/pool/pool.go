package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Common pool errors.
var (
	ErrPoolClosed       = errors.New("pool is closed")
	ErrNilObject        = errors.New("cannot put nil object")
	ErrPoolFull         = errors.New("pool is full")
	ErrInvalidOperation = errors.New("invalid pool operation")
	ErrTimeout          = errors.New("operation timed out")
)

// PoolStats provides observability metrics for pool operations.
type PoolStats struct {
	// Basic pool metrics
	Gets        int64 // Total Get() calls
	Puts        int64 // Total Put() calls
	Creates     int64 // Total NewObj() calls
	Hits        int64 // Get() calls that reused existing objects
	Misses      int64 // Get() calls that created new objects
	Rejects     int64 // Put() calls rejected by destructor
	Fails       int64 // Put() calls that failed (pool full)
	MaxSize     int   // Maximum pool capacity
	CurrentSize int64 // Current number of objects in pool

	// Performance metrics
	TotalGetTime    int64 // Total time spent in Get() operations (nanoseconds)
	TotalPutTime    int64 // Total time spent in Put() operations (nanoseconds)
	TotalCreateTime int64 // Total time spent in NewObj() operations (nanoseconds)
}

// Copy returns a copy of the current stats to avoid race conditions.
func (s *PoolStats) Copy() PoolStats {
	return PoolStats{
		Gets:            atomic.LoadInt64(&s.Gets),
		Puts:            atomic.LoadInt64(&s.Puts),
		Creates:         atomic.LoadInt64(&s.Creates),
		Hits:            atomic.LoadInt64(&s.Hits),
		Misses:          atomic.LoadInt64(&s.Misses),
		Rejects:         atomic.LoadInt64(&s.Rejects),
		Fails:           atomic.LoadInt64(&s.Fails),
		MaxSize:         s.MaxSize,
		CurrentSize:     atomic.LoadInt64(&s.CurrentSize),
		TotalGetTime:    atomic.LoadInt64(&s.TotalGetTime),
		TotalPutTime:    atomic.LoadInt64(&s.TotalPutTime),
		TotalCreateTime: atomic.LoadInt64(&s.TotalCreateTime),
	}
}

// HitRate returns the cache hit rate as a percentage (0.0-1.0).
func (s *PoolStats) HitRate() float64 {
	total := atomic.LoadInt64(&s.Gets)
	if total == 0 {
		return 0.0
	}

	return float64(atomic.LoadInt64(&s.Hits)) / float64(total)
}

// AverageGetTime returns the average time for Get() operations in nanoseconds.
func (s *PoolStats) AverageGetTime() time.Duration {
	gets := atomic.LoadInt64(&s.Gets)
	if gets == 0 {
		return 0
	}

	totalTime := atomic.LoadInt64(&s.TotalGetTime)

	return time.Duration(totalTime / gets)
}

// AveragePutTime returns the average time for Put() operations in nanoseconds.
func (s *PoolStats) AveragePutTime() time.Duration {
	puts := atomic.LoadInt64(&s.Puts)
	if puts == 0 {
		return 0
	}

	totalTime := atomic.LoadInt64(&s.TotalPutTime)

	return time.Duration(totalTime / puts)
}

// AverageCreateTime returns the average time for NewObj() operations in nanoseconds.
func (s *PoolStats) AverageCreateTime() time.Duration {
	creates := atomic.LoadInt64(&s.Creates)
	if creates == 0 {
		return 0
	}

	totalTime := atomic.LoadInt64(&s.TotalCreateTime)

	return time.Duration(totalTime / creates)
}

// String provides a human-readable representation of pool statistics.
func (s *PoolStats) String() string {
	stats := s.Copy()

	return fmt.Sprintf(
		"Pool Stats: Gets=%d, Puts=%d, Creates=%d, Hits=%d (%.1f%%), Misses=%d, Rejects=%d, Fails=%d, CurrentSize=%d/%d, AvgGetTime=%v, AvgPutTime=%v, AvgCreateTime=%v",
		stats.Gets,
		stats.Puts,
		stats.Creates,
		stats.Hits,
		stats.HitRate()*100,
		stats.Misses,
		stats.Rejects,
		stats.Fails,
		stats.CurrentSize,
		stats.MaxSize,
		stats.AverageGetTime(),
		stats.AveragePutTime(),
		stats.AverageCreateTime(),
	)
}

type Poolobj[t any] struct {
	// objs is a channel of type T
	objs chan *t
	// pool sync.Pool
	// Function will be run on Get() - include here your logic to create the initial object
	constructor func(*t)
	// Function will be run on Put() - include here your logic to reset the object
	destructor func(*t) bool

	// Enhanced features
	stats   PoolStats
	closed  int32        // atomic bool for closed state
	maxSize int          // maximum pool capacity
	mu      sync.RWMutex // for pool state operations
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects. Uses non-blocking channel operation to avoid race conditions.
func (p *Poolobj[t]) Get() *t {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.TotalGetTime, int64(time.Since(start)))
	}()

	atomic.AddInt64(&p.stats.Gets, 1)

	if atomic.LoadInt32(&p.closed) == 1 {
		return nil
	}

	select {
	case obj := <-p.objs:
		atomic.AddInt64(&p.stats.Hits, 1)
		atomic.AddInt64(&p.stats.CurrentSize, -1)
		return obj

	default:
		atomic.AddInt64(&p.stats.Misses, 1)
		return p.NewObj()
	}
}

// GetWithContext retrieves an object from the pool with context support.
// Returns an object and nil error on success, or nil object and error on failure.
func (p *Poolobj[t]) GetWithContext(ctx context.Context) (*t, error) {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.TotalGetTime, int64(time.Since(start)))
	}()

	atomic.AddInt64(&p.stats.Gets, 1)

	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, ErrPoolClosed
	}

	select {
	case obj := <-p.objs:
		atomic.AddInt64(&p.stats.Hits, 1)
		atomic.AddInt64(&p.stats.CurrentSize, -1)
		return obj, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		atomic.AddInt64(&p.stats.Misses, 1)
		return p.NewObj(), nil
	}
}

// GetWithTimeout retrieves an object from the pool with a timeout.
// If no object is available and timeout expires, returns nil.
func (p *Poolobj[t]) GetWithTimeout(timeout time.Duration) (*t, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.GetWithContext(ctx)
}

// NewObj creates a new object of type T, optionally initializing it using the pool's constructor function.
// If a constructor is defined, it is called with a pointer to the newly created object.
// Returns a pointer to the newly created object.
func (p *Poolobj[t]) NewObj() *t {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.TotalCreateTime, int64(time.Since(start)))
	}()

	atomic.AddInt64(&p.stats.Creates, 1)

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
	err := p.PutWithError(bo)
	return err == nil
}

// PutWithError returns an object to the pool with explicit error handling.
// Returns nil on success, or an appropriate error on failure.
func (p *Poolobj[t]) PutWithError(bo *t) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.TotalPutTime, int64(time.Since(start)))
	}()

	atomic.AddInt64(&p.stats.Puts, 1)

	if bo == nil {
		return ErrNilObject
	}

	if atomic.LoadInt32(&p.closed) == 1 {
		return ErrPoolClosed
	}

	// Call destructor if provided
	if p.destructor != nil {
		if p.destructor(bo) {
			atomic.AddInt64(&p.stats.Rejects, 1)
			return ErrInvalidOperation
		}
	}

	// Try to put object back in pool using non-blocking send
	select {
	case p.objs <- bo:
		atomic.AddInt64(&p.stats.CurrentSize, 1)
		return nil

	default:
		atomic.AddInt64(&p.stats.Fails, 1)
		return ErrPoolFull
	}
}

// PutWithContext returns an object to the pool with context support.
// Returns nil on success, or an appropriate error on failure.
func (p *Poolobj[t]) PutWithContext(ctx context.Context, bo *t) error {
	start := time.Now()
	defer func() {
		atomic.AddInt64(&p.stats.TotalPutTime, int64(time.Since(start)))
	}()

	atomic.AddInt64(&p.stats.Puts, 1)

	if bo == nil {
		return ErrNilObject
	}

	if atomic.LoadInt32(&p.closed) == 1 {
		return ErrPoolClosed
	}

	// Call destructor if provided
	if p.destructor != nil {
		if p.destructor(bo) {
			atomic.AddInt64(&p.stats.Rejects, 1)
			return ErrInvalidOperation
		}
	}

	// Try to put object back in pool
	select {
	case p.objs <- bo:
		atomic.AddInt64(&p.stats.CurrentSize, 1)
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Init initializes the Poolobj by setting the constructor and destructor functions,
// creating the object channel with a specified capacity, and optionally creating
// and adding the initial set of objects to the pool using the provided constructor.
func (p *Poolobj[t]) Init(maxsize, initcreate int, constructor func(*t), destructor func(*t) bool) {
	if maxsize <= 0 {
		maxsize = 200 // default capacity
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.constructor = constructor
	p.destructor = destructor
	p.maxSize = maxsize
	p.stats.MaxSize = maxsize
	atomic.StoreInt32(&p.closed, 0)

	p.objs = make(chan *t, maxsize)

	if initcreate == 0 {
		return
	}

	for range initcreate {
		p.Put(p.NewObj())
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

// Len returns the current number of objects in the pool.
// This provides visibility into pool utilization.
func (p *Poolobj[t]) Len() int {
	return int(atomic.LoadInt64(&p.stats.CurrentSize))
}

// Cap returns the maximum capacity of the pool.
func (p *Poolobj[t]) Cap() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSize
}

// Stats returns a copy of the current pool statistics.
// This provides comprehensive observability into pool performance.
func (p *Poolobj[t]) Stats() PoolStats {
	return p.stats.Copy()
}

// IsHealthy returns true if the pool is in a healthy state.
// A pool is considered healthy if it's not closed and has reasonable hit rates.
func (p *Poolobj[t]) IsHealthy() bool {
	if atomic.LoadInt32(&p.closed) == 1 {
		return false
	}

	stats := p.stats.Copy()
	// Consider the pool healthy if we have a reasonable hit rate (>= 10%)
	// or if we haven't had enough operations to judge yet
	if stats.Gets < 10 {
		return true
	}

	return stats.HitRate() >= 0.1
}

// IsClosed returns true if the pool has been closed.
func (p *Poolobj[t]) IsClosed() bool {
	return atomic.LoadInt32(&p.closed) == 1
}

// Close drains and closes the pool, preventing further operations.
// Returns the number of objects that were drained from the pool.
func (p *Poolobj[t]) Close() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return 0 // Already closed
	}

	atomic.StoreInt32(&p.closed, 1)

	// Drain the pool
	var drained int
	for {
		select {
		case <-p.objs:
			drained++

			atomic.AddInt64(&p.stats.CurrentSize, -1)

		default:
			return drained
		}
	}
}

// Drain removes all objects from the pool without closing it.
// Returns the number of objects that were drained.
// This can be useful for clearing the pool when object state might be stale.
func (p *Poolobj[t]) Drain() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return 0
	}

	var drained int
	for {
		select {
		case <-p.objs:
			drained++

			atomic.AddInt64(&p.stats.CurrentSize, -1)

		default:
			return drained
		}
	}
}

// Reset clears all statistics and drains the pool.
// The pool remains open and functional after reset.
func (p *Poolobj[t]) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return
	}

	// Drain the pool
	for {
		select {
		case <-p.objs:
			atomic.AddInt64(&p.stats.CurrentSize, -1)
		default:
			goto resetStats
		}
	}

resetStats:
	// Reset all statistics
	atomic.StoreInt64(&p.stats.Gets, 0)

	atomic.StoreInt64(&p.stats.Puts, 0)
	atomic.StoreInt64(&p.stats.Creates, 0)
	atomic.StoreInt64(&p.stats.Hits, 0)
	atomic.StoreInt64(&p.stats.Misses, 0)
	atomic.StoreInt64(&p.stats.Rejects, 0)
	atomic.StoreInt64(&p.stats.Fails, 0)
	atomic.StoreInt64(&p.stats.CurrentSize, 0)
	atomic.StoreInt64(&p.stats.TotalGetTime, 0)
	atomic.StoreInt64(&p.stats.TotalPutTime, 0)
	atomic.StoreInt64(&p.stats.TotalCreateTime, 0)
}

// UtilizationPercent returns the current pool utilization as a percentage (0.0-1.0).
func (p *Poolobj[t]) UtilizationPercent() float64 {
	current := float64(p.Len())

	maxv := float64(p.Cap())
	if maxv == 0 {
		return 0.0
	}

	return current / maxv
}

// IsAtCapacity returns true if the pool is at maximum capacity.
func (p *Poolobj[t]) IsAtCapacity() bool {
	return p.Len() >= p.Cap()
}

// Warmup pre-fills the pool with the specified number of objects.
// This can be useful for improving initial performance by avoiding
// object creation during the first requests.
func (p *Poolobj[t]) Warmup(count int) int {
	if atomic.LoadInt32(&p.closed) == 1 {
		return 0
	}

	var created int
	for i := 0; i < count && !p.IsAtCapacity(); i++ {
		obj := p.NewObj()
		if p.Put(obj) {
			created++
		}
	}

	return created
}

// SizedWaitGroupStats provides observability for SizedWaitGroup operations.
type SizedWaitGroupStats struct {
	Adds      int64 // Total Add() calls
	Dones     int64 // Total Done() calls
	Waits     int64 // Total Wait() calls
	Currently int64 // Currently active operations
	MaxSize   int   // Maximum concurrent operations allowed
}

// Copy returns a copy of the current stats.
func (s *SizedWaitGroupStats) Copy() SizedWaitGroupStats {
	return SizedWaitGroupStats{
		Adds:      atomic.LoadInt64(&s.Adds),
		Dones:     atomic.LoadInt64(&s.Dones),
		Waits:     atomic.LoadInt64(&s.Waits),
		Currently: atomic.LoadInt64(&s.Currently),
		MaxSize:   s.MaxSize,
	}
}

// UtilizationPercent returns current utilization as a percentage (0.0-1.0).
func (s *SizedWaitGroupStats) UtilizationPercent() float64 {
	current := float64(atomic.LoadInt64(&s.Currently))

	maxv := float64(s.MaxSize)
	if maxv == 0 {
		return 0.0
	}

	return current / maxv
}

type SizedWaitGroup struct {
	wg      sync.WaitGroup
	current chan struct{}
	Size    int
	stats   SizedWaitGroupStats
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
		stats:   SizedWaitGroupStats{MaxSize: limit},
	}
}

// Add increments the SizedWaitGroup counter by one. It also adds a token to the
// current channel, which limits the number of concurrent operations.
func (s *SizedWaitGroup) Add() {
	atomic.AddInt64(&s.stats.Adds, 1)

	s.current <- struct{}{}

	atomic.AddInt64(&s.stats.Currently, 1)

	s.wg.Add(1)
}

// AddWithContext increments the SizedWaitGroup counter with context support.
// Returns an error if the context is cancelled before acquiring a slot.
func (s *SizedWaitGroup) AddWithContext(ctx context.Context) error {
	atomic.AddInt64(&s.stats.Adds, 1)

	select {
	case s.current <- struct{}{}:
		atomic.AddInt64(&s.stats.Currently, 1)
		s.wg.Add(1)
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAdd attempts to increment the SizedWaitGroup counter without blocking.
// Returns true if successful, false if no slots are available.
func (s *SizedWaitGroup) TryAdd() bool {
	atomic.AddInt64(&s.stats.Adds, 1)

	select {
	case s.current <- struct{}{}:
		atomic.AddInt64(&s.stats.Currently, 1)
		s.wg.Add(1)
		return true

	default:
		return false
	}
}

// Done decrements the SizedWaitGroup counter by one. It also removes a token from the
// current channel, which limits the number of concurrent operations.
func (s *SizedWaitGroup) Done() {
	atomic.AddInt64(&s.stats.Dones, 1)
	<-s.current
	atomic.AddInt64(&s.stats.Currently, -1)
	s.wg.Done()
}

// Wait blocks until all operations added to the SizedWaitGroup have completed.
func (s *SizedWaitGroup) Wait() {
	atomic.AddInt64(&s.stats.Waits, 1)
	s.wg.Wait()
}

// WaitWithContext waits for all operations to complete with context support.
func (s *SizedWaitGroup) WaitWithContext(ctx context.Context) error {
	atomic.AddInt64(&s.stats.Waits, 1)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitWithTimeout waits for all operations to complete with a timeout.
func (s *SizedWaitGroup) WaitWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.WaitWithContext(ctx)
}

// Close resets the SizedWaitGroup to its initial state, allowing it to be reused.
// Note: This should only be called after Wait() has completed to avoid goroutine leaks.
func (s *SizedWaitGroup) Close() {
	*s = SizedWaitGroup{}
}

// Stats returns a copy of the current statistics.
func (s *SizedWaitGroup) Stats() SizedWaitGroupStats {
	return s.stats.Copy()
}

// CurrentlyActive returns the number of currently active operations.
func (s *SizedWaitGroup) CurrentlyActive() int {
	return int(atomic.LoadInt64(&s.stats.Currently))
}

// UtilizationPercent returns the current utilization as a percentage (0.0-1.0).
func (s *SizedWaitGroup) UtilizationPercent() float64 {
	return s.stats.UtilizationPercent()
}

// IsAtCapacity returns true if the SizedWaitGroup is at maximum capacity.
func (s *SizedWaitGroup) IsAtCapacity() bool {
	return s.CurrentlyActive() >= s.Size
}
