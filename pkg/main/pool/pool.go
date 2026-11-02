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
	ErrNilObject        = errors.New("cannot put nil object")
	ErrPoolFull         = errors.New("pool is full")
	ErrInvalidOperation = errors.New("invalid pool operation")
)

// PoolStats provides observability metrics for pool operations. All counters
// are atomic.Int64 (rather than plain int64 with free-function atomic.*
// calls) so the struct is correctly 8-byte aligned on 32-bit platforms too -
// a plain int64 field only gets the alignment guarantee required by
// atomic.AddInt64/LoadInt64 on 32-bit ARM/x86 when it happens to be first in
// the struct, which was not guaranteed here. Because atomic.Int64 embeds a
// noCopy guard, PoolStats itself must never be copied by value - use
// Snapshot() to get a plain, freely-copyable point-in-time snapshot instead.
type PoolStats struct {
	// Basic pool metrics
	Gets    atomic.Int64 // Total Get() calls
	Puts    atomic.Int64 // Total Put() calls
	Creates atomic.Int64 // Total NewObj() calls
	Hits    atomic.Int64 // Get() calls that reused existing objects
	Misses  atomic.Int64 // Get() calls that created new objects
	Rejects atomic.Int64 // Put() calls rejected by destructor
	Fails   atomic.Int64 // Put() calls that failed (pool full)
	MaxSize int          // Maximum pool capacity (set once under Poolobj.mu, not atomic)

	// Performance metrics
	TotalGetTime    atomic.Int64 // Total time spent in Get() operations (nanoseconds)
	TotalPutTime    atomic.Int64 // Total time spent in Put() operations (nanoseconds)
	TotalCreateTime atomic.Int64 // Total time spent in NewObj() operations (nanoseconds)
}

// PoolStatsSnapshot is a point-in-time, plain-int64 copy of PoolStats - safe
// to pass around and copy by value (unlike PoolStats itself).
type PoolStatsSnapshot struct {
	Gets            int64
	Puts            int64
	Creates         int64
	Hits            int64
	Misses          int64
	Rejects         int64
	Fails           int64
	MaxSize         int
	CurrentSize     int64 // set by Poolobj.Stats from the channel length
	TotalGetTime    int64
	TotalPutTime    int64
	TotalCreateTime int64
}

// Snapshot returns a copy of the current stats to avoid race conditions.
func (s *PoolStats) Snapshot() PoolStatsSnapshot {
	return PoolStatsSnapshot{
		Gets:            s.Gets.Load(),
		Puts:            s.Puts.Load(),
		Creates:         s.Creates.Load(),
		Hits:            s.Hits.Load(),
		Misses:          s.Misses.Load(),
		Rejects:         s.Rejects.Load(),
		Fails:           s.Fails.Load(),
		MaxSize:         s.MaxSize,
		TotalGetTime:    s.TotalGetTime.Load(),
		TotalPutTime:    s.TotalPutTime.Load(),
		TotalCreateTime: s.TotalCreateTime.Load(),
	}
}

// HitRate returns the cache hit rate as a percentage (0.0-1.0).
func (s PoolStatsSnapshot) HitRate() float64 {
	if s.Gets == 0 {
		return 0.0
	}

	return float64(s.Hits) / float64(s.Gets)
}

// AverageGetTime returns the average time for Get() operations in nanoseconds.
func (s PoolStatsSnapshot) AverageGetTime() time.Duration {
	if s.Gets == 0 {
		return 0
	}

	return time.Duration(s.TotalGetTime / s.Gets)
}

// AveragePutTime returns the average time for Put() operations in nanoseconds.
func (s PoolStatsSnapshot) AveragePutTime() time.Duration {
	if s.Puts == 0 {
		return 0
	}

	return time.Duration(s.TotalPutTime / s.Puts)
}

// AverageCreateTime returns the average time for NewObj() operations in nanoseconds.
func (s PoolStatsSnapshot) AverageCreateTime() time.Duration {
	if s.Creates == 0 {
		return 0
	}

	return time.Duration(s.TotalCreateTime / s.Creates)
}

// String provides a human-readable representation of pool statistics.
func (s *PoolStats) String() string {
	return s.Snapshot().String()
}

// String provides a human-readable representation of a pool statistics snapshot.
func (s PoolStatsSnapshot) String() string {
	return fmt.Sprintf(
		"Pool Stats: Gets=%d, Puts=%d, Creates=%d, Hits=%d (%.1f%%), Misses=%d, Rejects=%d, Fails=%d, CurrentSize=%d/%d, AvgGetTime=%v, AvgPutTime=%v, AvgCreateTime=%v",
		s.Gets,
		s.Puts,
		s.Creates,
		s.Hits,
		s.HitRate()*100,
		s.Misses,
		s.Rejects,
		s.Fails,
		s.CurrentSize,
		s.MaxSize,
		s.AverageGetTime(),
		s.AveragePutTime(),
		s.AverageCreateTime(),
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
	maxSize int          // maximum pool capacity
	mu      sync.RWMutex // for pool state operations
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects. Uses non-blocking channel operation to avoid race conditions.
func (p *Poolobj[t]) Get() *t {
	start := time.Now()
	defer func() {
		p.stats.TotalGetTime.Add(int64(time.Since(start)))
	}()

	p.stats.Gets.Add(1)

	select {
	case obj := <-p.objs:
		p.stats.Hits.Add(1)
		return obj

	default:
		p.stats.Misses.Add(1)
		return p.NewObj()
	}
}

// NewObj creates a new object of type T, optionally initializing it using the pool's constructor function.
// If a constructor is defined, it is called with a pointer to the newly created object.
// Returns a pointer to the newly created object.
func (p *Poolobj[t]) NewObj() *t {
	start := time.Now()
	defer func() {
		p.stats.TotalCreateTime.Add(int64(time.Since(start)))
	}()

	p.stats.Creates.Add(1)

	var bo t
	if p.constructor != nil {
		p.constructor(&bo)
	}

	return &bo
}

// Put returns an object to the pool and reports whether it was kept.
// If the pool has room, the destructor (if any) resets the object and may
// reject it; a full pool drops the object without running the destructor.
func (p *Poolobj[t]) Put(bo *t) bool {
	err := p.PutWithError(bo)
	return err == nil
}

// PutWithError returns an object to the pool with explicit error handling.
// Returns nil on success, or an appropriate error on failure.
func (p *Poolobj[t]) PutWithError(bo *t) error {
	start := time.Now()
	defer func() {
		p.stats.TotalPutTime.Add(int64(time.Since(start)))
	}()

	p.stats.Puts.Add(1)

	if bo == nil {
		return ErrNilObject
	}

	// Cheap pre-check so a full pool doesn't pay for resetting an object it
	// is going to drop anyway; the non-blocking send below stays authoritative.
	if len(p.objs) >= cap(p.objs) {
		p.stats.Fails.Add(1)
		return ErrPoolFull
	}

	if p.destructor != nil {
		if p.destructor(bo) {
			p.stats.Rejects.Add(1)
			return ErrInvalidOperation
		}
	}

	select {
	case p.objs <- bo:
		return nil

	default:
		p.stats.Fails.Add(1)
		return ErrPoolFull
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
// destructor, if non-nil, is called on every Put that has room in the pool,
// before the object is stored, to reset it. Returning true rejects the
// object (it is dropped instead of pooled), e.g. to discard oversized buffers.
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
	return len(p.objs)
}

// Cap returns the maximum capacity of the pool.
func (p *Poolobj[t]) Cap() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSize
}

// Stats returns a snapshot of the current pool statistics.
// This provides comprehensive observability into pool performance.
func (p *Poolobj[t]) Stats() PoolStatsSnapshot {
	snap := p.stats.Snapshot()
	snap.CurrentSize = int64(len(p.objs))

	return snap
}

// IsHealthy returns true if the pool is in a healthy state.
// A pool is considered healthy if it has reasonable hit rates.
func (p *Poolobj[t]) IsHealthy() bool {
	stats := p.stats.Snapshot()
	// Consider the pool healthy if we have a reasonable hit rate (>= 10%)
	// or if we haven't had enough operations to judge yet
	if stats.Gets < 10 {
		return true
	}

	return stats.HitRate() >= 0.1
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
	// Resetting to the zero value would leave current nil and Size 0 - any
	// subsequent Add() would then block forever sending on a nil channel,
	// contradicting this method's own "allowing it to be reused" contract.
	// Reinitialize with the original size instead of zeroing it away.
	*s = NewSizedGroup(s.Size)
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
