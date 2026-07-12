package syncops

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/robfig/cron/v3"
)

// DbstaticOneStringTwoInt and related types are database static types used in SyncMap operations.
type DbstaticOneStringTwoInt struct {
	Str  string `db:"str"`
	Num1 uint   `db:"num1"`
	Num2 uint   `db:"num2"`
}

type DbstaticTwoStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Num  uint   `db:"num"`
}

type DbstaticThreeStringTwoInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
	Num1 int    `db:"num1"`
	Num2 uint   `db:"num2"`
}

// Job represents a job to be run by a worker pool (from worker package).
type Job struct {
	Queue       string
	JobName     string `json:"-"`
	Cfgpstr     string `json:"-"`
	Name        string
	Added       time.Time
	Started     time.Time
	ID          uint32
	SchedulerID uint32
	Ctx         context.Context    `json:"-"`
	CancelFunc  context.CancelFunc `json:"-"`
}

// JobSchedule represents a scheduled job (from worker package).
type JobSchedule struct {
	CancelFunc     context.CancelFunc `json:"-"`
	CronSchedule   cron.Schedule
	JobName        string
	ScheduleTyp    string
	ScheduleString string
	LastRun        time.Time
	NextRun        time.Time
	Interval       time.Duration
	CronID         cron.EntryID
	JobID          uint32
	ID             uint32
	IsRunning      bool
}

// OpType defines operation types for the unified single-writer system.
type OpType string

const (
	// OpSyncMapAdd and related constants are SyncMap operations (from logger package).
	OpSyncMapAdd          OpType = "syncMapAdd"
	OpSyncMapUpdateVal    OpType = "syncMapUpdateVal"
	OpSyncMapUpdateExpire OpType = "syncMapUpdateExpire"
	OpSyncMapUpdateLscan  OpType = "syncMapUpdateLastscan"
	OpSyncMapDelete       OpType = "syncMapDelete"

	// OpWorkerMapAdd and related constants are SyncMapUint operations (from worker package).
	OpWorkerMapAdd         OpType = "workerMapAdd"
	OpWorkerMapUpdate      OpType = "workerMapUpdate"
	OpWorkerMapDelete      OpType = "workerMapDelete"
	OpWorkerMapDeleteQueue OpType = "workerMapDeleteQueue"

	// OpSliceAppend is an atomic slice operation.
	OpSliceAppend OpType = "sliceAppend"

	// OpSyncMapCheckExpires and related constants are advanced SyncMap operations.
	OpSyncMapCheckExpires      OpType = "syncMapCheckExpires"
	OpSyncMapDeleteFunc        OpType = "syncMapDeleteFunc"
	OpSyncMapDeleteFuncExpires OpType = "syncMapDeleteFuncExpires"
	OpAtomicAppendString       OpType = "atomicAppendString"
	OpAtomicRemoveString       OpType = "atomicRemoveString"

	// OpFunc runs an arbitrary function inside the writer goroutine,
	// serializing it with all other cache operations.
	OpFunc OpType = "func"
)

// MapType identifies which map to operate on.
type MapType string

const (
	// MapTypeString and related constants are logger SyncMap types.
	MapTypeString      MapType = "string"
	MapTypeTwoString   MapType = "twostring"
	MapTypeThreeString MapType = "threestring"
	MapTypeTwoInt      MapType = "twoint"
	MapTypeStructEmpty MapType = "structempty"

	// MapTypeSchedule and related constants are worker SyncMapUint types.
	MapTypeSchedule MapType = "schedule"
	MapTypeQueue    MapType = "queue"
)

// SyncOperation represents a synchronized operation to be processed by the single writer.
type SyncOperation struct {
	OpType    OpType
	MapType   MapType
	Key       string
	KeyUint32 uint32
	Value     any
	Expires   int64
	IMDB      bool
	Lastscan  int64
	ResultCh  chan bool

	// Additional fields for advanced operations
	Extend     bool
	Duration   int
	FilterFunc any // Function for DeleteFunc operations

	// Worker-specific fields
	Queue     string // Queue name for worker operations
	IsStarted bool   // For queue-based deletion

	Fn func() // For OpFunc operations
}

// SyncOpsManager manages all synchronized operations across the application.
type SyncOpsManager struct {
	opQueue      chan SyncOperation
	writerActive atomic.Bool
	ctx          context.Context
	cancel       context.CancelFunc

	// References to the actual data structures
	syncMaps   map[MapType]any
	syncMapsMu sync.RWMutex
}

var (
	// Global manager instance.
	manager  *SyncOpsManager
	initOnce sync.Once

	// resultChPool reuses the per-call result channels. The channels have a
	// buffer of 1 so the writer's result send can never block, and they are
	// always drained before being returned to the pool.
	resultChPool = sync.Pool{New: func() any { return make(chan bool, 1) }}
)

// InitSyncOps initializes the global synchronized operations manager using a singleton pattern.
// Creates a single-writer goroutine that processes all synchronization operations to ensure
// thread safety across the application. Must be called once before using any sync operations.
// Subsequent calls are ignored due to sync.Once protection.
func InitSyncOps() {
	initOnce.Do(func() {
		ctx, cancel := context.WithCancel(
			context.Background(),
		)

		manager = &SyncOpsManager{
			opQueue:  make(chan SyncOperation, 1024),
			ctx:      ctx,
			cancel:   cancel,
			syncMaps: make(map[MapType]any),
		}
		manager.startWriter()
	})
}

// RegisterSyncMap registers a SyncMap or SyncMapUint instance for synchronized operations.
// Associates the provided map with a specific MapType identifier, allowing the operations
// manager to route operations to the correct map. Automatically initializes the manager
// if not already done. Thread-safe and can be called from multiple goroutines.
//
// Re-registering a MapType replaces the previous map: queued operations for that
// MapType silently stop reaching the old map. That is almost never intended (each
// MapType should have exactly one owner), so an overwrite is logged as a warning.
func RegisterSyncMap(mapType MapType, syncMap any) {
	if manager == nil {
		InitSyncOps()
	}

	manager.syncMapsMu.Lock()
	defer manager.syncMapsMu.Unlock()

	if existing, ok := manager.syncMaps[mapType]; ok && existing != syncMap {
		logger.Logtype("warn", 0).
			Str("maptype", string(mapType)).
			Str("existing", reflect.TypeOf(existing).String()).
			Str("new", reflect.TypeOf(syncMap).String()).
			Msg("RegisterSyncMap overwrote an existing registration - queued operations now target the new map")
	}

	manager.syncMaps[mapType] = syncMap
}

// QueueOperation submits a synchronization operation to the single-writer goroutine and
// blocks until it completes. Returns true if the operation was processed successfully,
// false if it failed (map not registered, type mismatch) or the manager is shut down.
// Failures are also logged by the writer, so callers may ignore the return value.
func QueueOperation(op SyncOperation) bool {
	m := manager
	if m == nil || !m.writerActive.Load() {
		return false
	}

	ch := resultChPool.Get().(chan bool)

	op.ResultCh = ch

	select {
	case m.opQueue <- op:
	case <-m.ctx.Done():
		resultChPool.Put(ch)
		return false
	}

	select {
	case ok := <-ch:
		resultChPool.Put(ch)
		return ok

	case <-m.ctx.Done():
		// Shutdown raced with this submission. The writer drains the queue on
		// shutdown and signals every queued op, but if this op was enqueued
		// after the drain finished no signal will ever arrive - give up after
		// a grace period instead of blocking forever.
		select {
		case ok := <-ch:
			resultChPool.Put(ch)
			return ok

		case <-time.After(5 * time.Second):
			// Abandon ch: a late send by the writer goes into its buffer and
			// cannot block, but the channel must not be reused.
			return false
		}
	}
}

// QueueSyncMapAdd queues an Add operation for a SyncMap with full metadata.
// Stores a key-value pair along with expiration time, IMDB flag, and last scan timestamp.
// The operation is processed atomically by the single-writer goroutine to ensure thread safety.
func QueueSyncMapAdd(
	mapType MapType,
	key string,
	value any,
	expires int64,
	imdb bool,
	lastscan int64,
) {
	QueueOperation(SyncOperation{
		OpType:   OpSyncMapAdd,
		MapType:  mapType,
		Key:      key,
		Value:    value,
		Expires:  expires,
		IMDB:     imdb,
		Lastscan: lastscan,
	})
}

// QueueSyncMapUpdateVal queues a value update operation for an existing SyncMap entry.
// Modifies only the value while preserving expiration, IMDB flag, and last scan metadata.
// The operation is processed atomically by the single-writer goroutine to ensure thread safety.
func QueueSyncMapUpdateVal(mapType MapType, key string, value any) {
	QueueOperation(SyncOperation{
		OpType:  OpSyncMapUpdateVal,
		MapType: mapType,
		Key:     key,
		Value:   value,
	})
}

// QueueSyncMapUpdateExpire queues an expiration time update for an existing SyncMap entry.
// Modifies only the expiration timestamp while preserving the value, IMDB flag, and last scan data.
// Pass 0 to disable expiration or a Unix timestamp for specific expiry time.
func QueueSyncMapUpdateExpire(mapType MapType, key string, expires int64) {
	QueueOperation(SyncOperation{
		OpType:  OpSyncMapUpdateExpire,
		MapType: mapType,
		Key:     key,
		Expires: expires,
	})
}

// QueueSyncMapUpdateLastscan queues a last scan timestamp update for tracking purposes.
// Updates when a key was last processed or accessed for maintenance operations.
// Preserves the value, expiration time, and IMDB flag while updating scan metadata.
func QueueSyncMapUpdateLastscan(mapType MapType, key string, lastscan int64) {
	QueueOperation(SyncOperation{
		OpType:   OpSyncMapUpdateLscan,
		MapType:  mapType,
		Key:      key,
		Lastscan: lastscan,
	})
}

// QueueSyncMapDelete queues a complete removal operation for a SyncMap entry.
// Removes the key and all associated metadata including value, expiration, IMDB flag, and last scan time.
// The operation is processed atomically by the single-writer goroutine to ensure thread safety.
func QueueSyncMapDelete(mapType MapType, key string) {
	QueueOperation(SyncOperation{
		OpType:  OpSyncMapDelete,
		MapType: mapType,
		Key:     key,
	})
}

// QueueWorkerMapAdd queues an Add operation for worker pool SyncMapUint instances.
// Used for managing jobs and schedules with uint32 keys. Supports JobSchedule and Job types
// for worker pool management. The operation is processed atomically by the single-writer goroutine.
func QueueWorkerMapAdd(mapType MapType, key uint32, value any) {
	QueueOperation(SyncOperation{
		OpType:    OpWorkerMapAdd,
		MapType:   mapType,
		KeyUint32: key,
		Value:     value,
	})
}

// QueueWorkerMapUpdate queues a value update operation for worker pool SyncMapUint instances.
// Modifies existing job or schedule entries identified by uint32 keys. Used for updating
// job status, schedule information, or other worker pool related data atomically.
func QueueWorkerMapUpdate(mapType MapType, key uint32, value any) {
	QueueOperation(SyncOperation{
		OpType:    OpWorkerMapUpdate,
		MapType:   mapType,
		KeyUint32: key,
		Value:     value,
	})
}

// QueueWorkerMapDelete queues a removal operation for worker pool SyncMapUint entries.
// Removes jobs or schedules identified by uint32 keys from worker pool management maps.
// Used for cleanup when jobs complete or schedules are cancelled.
func QueueWorkerMapDelete(mapType MapType, key uint32) {
	QueueOperation(SyncOperation{
		OpType:    OpWorkerMapDelete,
		MapType:   mapType,
		KeyUint32: key,
	})
}

// QueueWorkerMapDeleteQueue queues conditional deletion of jobs from worker queues.
// Removes all jobs matching the specified queue name, with optional filtering by started status.
// Used for bulk cleanup operations when clearing queues or removing specific job categories.
func QueueWorkerMapDeleteQueue(mapType MapType, queue string, isStarted bool) {
	QueueOperation(SyncOperation{
		OpType:    OpWorkerMapDeleteQueue,
		MapType:   mapType,
		Queue:     queue,
		IsStarted: isStarted,
	})
}

// QueueSliceAppend queues an atomic append operation for slice-based SyncMap values.
// Safely adds elements to slices stored in SyncMaps while checking for duplicates.
// Supports string slices and database static type slices with type-safe operations.
func QueueSliceAppend(mapType MapType, key string, value any) {
	QueueOperation(SyncOperation{
		OpType:  OpSliceAppend,
		MapType: mapType,
		Key:     key,
		Value:   value,
	})
}

// QueueSyncMapCheckExpires queues an expiration check operation for SyncMap entries.
// Verifies if a key has expired and optionally extends the expiration time by the specified
// duration in hours. Used for cache management and automatic cleanup of stale entries.
func QueueSyncMapCheckExpires(mapType MapType, key string, extend bool, duration int) {
	QueueOperation(SyncOperation{
		OpType:   OpSyncMapCheckExpires,
		MapType:  mapType,
		Key:      key,
		Extend:   extend,
		Duration: duration,
	})
}

// QueueSyncMapDeleteFunc queues a conditional deletion operation using a custom filter function.
// Removes all entries where the filter function returns true when applied to the value.
// The filter function must not call QueueOperation or RegisterSyncMap - it runs inside
// the single writer goroutine, which would deadlock waiting on itself.
func QueueSyncMapDeleteFunc(mapType MapType, filterFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFunc,
		MapType:    mapType,
		FilterFunc: filterFunc,
	})
}

// QueueSyncMapDeleteFuncExpires queues deletion based on expiration time filtering.
// Removes entries where the filter function returns true when applied to the expiration timestamp.
// The filter function must not call QueueOperation or RegisterSyncMap - it runs inside
// the single writer goroutine, which would deadlock waiting on itself.
func QueueSyncMapDeleteFuncExpires(mapType MapType, filterFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFuncExpires,
		MapType:    mapType,
		FilterFunc: filterFunc,
	})
}

// QueueAtomicAppendString queues an atomic string append operation for string slice values.
// Safely appends a string to a string slice stored in a SyncMap while checking for duplicates.
// Pre-allocates extra capacity to reduce future allocations for better performance.
func QueueAtomicAppendString(mapType MapType, key string, value string) {
	QueueOperation(SyncOperation{
		OpType:  OpAtomicAppendString,
		MapType: mapType,
		Key:     key,
		Value:   value,
	})
}

// QueueAtomicRemoveString queues an atomic string removal operation for string slice values.
// Safely removes a specific string from a string slice stored in a SyncMap.
// Creates a new slice without the specified value, maintaining order of remaining elements.
func QueueAtomicRemoveString(mapType MapType, key string, value string) {
	QueueOperation(SyncOperation{
		OpType:  OpAtomicRemoveString,
		MapType: mapType,
		Key:     key,
		Value:   value,
	})
}

// QueueFunc runs fn inside the single writer goroutine, serializing it with all
// other cache operations. This prevents concurrent index rebuilds and races between
// index builds and concurrent slice appends.
// fn must not call QueueOperation or RegisterSyncMap - it runs inside the single
// writer goroutine, which would deadlock waiting on itself.
func QueueFunc(fn func()) {
	QueueOperation(SyncOperation{
		OpType: OpFunc,
		Fn:     fn,
	})
}

// Shutdown gracefully terminates the synchronized operations manager.
// Marks the writer inactive so new operations are rejected, then cancels the
// context: the writer goroutine drains any still-queued operations (signalling
// their waiting callers) before exiting. The operation channel is never closed,
// so concurrent QueueOperation calls cannot panic. Safe to call multiple times.
func Shutdown() {
	m := manager
	if m == nil {
		return
	}

	m.writerActive.Store(false)
	m.cancel()
}

// startWriter launches the single writer goroutine that processes all synchronization
// operations. The goroutine runs until the context is cancelled, then drains the queue
// (rejecting remaining operations) so no caller is left blocked on its result channel.
func (m *SyncOpsManager) startWriter() {
	if !m.writerActive.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer m.writerActive.Store(false)

		for {
			select {
			case <-m.ctx.Done():
				// Reject still-queued operations so their callers are not
				// left blocked waiting on ResultCh.
				for {
					select {
					case op := <-m.opQueue:
						sendResult(op.ResultCh, false)
					default:
						return
					}
				}

			case op := <-m.opQueue:
				sendResult(op.ResultCh, m.processOperation(op))
			}
		}
	}()
}

// sendResult delivers an operation result without ever blocking the writer.
// Result channels are buffered with capacity 1, so the default branch only
// triggers for an already-signalled (abandoned) channel.
func sendResult(ch chan bool, success bool) {
	if ch == nil {
		return
	}

	select {
	case ch <- success:
	default:
	}
}

// processOperation dispatches a single synchronization operation to the appropriate handler.
// The registered map is resolved under the read lock but the lock is released before the
// operation executes, so user-supplied callbacks (OpFunc, DeleteFunc filters) cannot
// deadlock against RegisterSyncMap. Failures (unregistered MapType, mismatched map or
// value type) are logged here - silent no-ops are how broken registrations stay hidden.
func (m *SyncOpsManager) processOperation(op SyncOperation) bool {
	switch op.OpType {
	case OpFunc:
		if op.Fn != nil {
			op.Fn()
		}

		return true

	default:
	}

	m.syncMapsMu.RLock()

	syncMapInterface, exists := m.syncMaps[op.MapType]
	m.syncMapsMu.RUnlock()

	if !exists {
		logger.Logtype("error", 0).
			Str("op", string(op.OpType)).
			Str("maptype", string(op.MapType)).
			Str("key", op.Key).
			Msg("syncops: no map registered for MapType - operation dropped")

		return false
	}

	if processRegisteredOp(syncMapInterface, op) {
		return true
	}

	logger.Logtype("error", 0).
		Str("op", string(op.OpType)).
		Str("maptype", string(op.MapType)).
		Str("key", op.Key).
		Any("keyuint", op.KeyUint32).
		Str("mapimpl", reflect.TypeOf(syncMapInterface).String()).
		Str("valuetype", typeName(op.Value)).
		Msg("syncops: operation dropped - registered map or value type does not match MapType")

	return false
}

// typeName returns the concrete type name of v for diagnostics, tolerating nil.
func typeName(v any) string {
	if v == nil {
		return "<nil>"
	}

	return reflect.TypeOf(v).String()
}

// processRegisteredOp executes an operation against the resolved map instance.
// Returns false when the map instance or operation value does not have the
// concrete type expected for the operation's MapType.
func processRegisteredOp(syncMapInterface any, op SyncOperation) bool {
	switch op.OpType {
	case OpSyncMapAdd:
		return processSyncMapAdd(syncMapInterface, op)
	case OpSyncMapUpdateVal:
		return processSyncMapUpdateVal(syncMapInterface, op)
	case OpSyncMapUpdateExpire:
		return processSyncMapUpdateExpire(syncMapInterface, op)
	case OpSyncMapUpdateLscan:
		return processSyncMapUpdateLastscan(syncMapInterface, op)
	case OpSyncMapDelete:
		return processSyncMapDelete(syncMapInterface, op)
	case OpWorkerMapAdd:
		return processWorkerMapAdd(syncMapInterface, op)
	case OpWorkerMapUpdate:
		return processWorkerMapUpdate(syncMapInterface, op)
	case OpWorkerMapDelete:
		return processWorkerMapDelete(syncMapInterface, op)
	case OpWorkerMapDeleteQueue:
		return processWorkerMapDeleteQueue(syncMapInterface, op)
	case OpSliceAppend:
		return processSliceAppend(syncMapInterface, op)
	case OpSyncMapCheckExpires:
		return processSyncMapCheckExpires(syncMapInterface, op)
	case OpSyncMapDeleteFunc:
		return processSyncMapDeleteFunc(syncMapInterface, op)
	case OpSyncMapDeleteFuncExpires:
		return processSyncMapDeleteFuncExpires(syncMapInterface, op)
	case OpAtomicAppendString:
		return processAtomicAppendString(syncMapInterface, op)
	case OpAtomicRemoveString:
		return processAtomicRemoveString(syncMapInterface, op)
	default:
		return false
	}
}

// syncMapAdd asserts the map and value to the concrete type for T and performs the Add.
func syncMapAdd[T any](syncMapInterface any, op SyncOperation) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[T])
	if !ok {
		return false
	}

	v, ok := op.Value.(T)
	if !ok {
		return false
	}

	syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)

	return true
}

// processSyncMapAdd handles Add operations for SyncMap instances.
func processSyncMapAdd(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapAdd[[]string](syncMapInterface, op)
	case MapTypeTwoString:
		return syncMapAdd[[]DbstaticTwoStringOneInt](syncMapInterface, op)
	case MapTypeThreeString:
		return syncMapAdd[[]DbstaticThreeStringTwoInt](syncMapInterface, op)
	case MapTypeTwoInt:
		return syncMapAdd[[]DbstaticOneStringTwoInt](syncMapInterface, op)
	case MapTypeStructEmpty:
		return syncMapAdd[struct{}](syncMapInterface, op)
	default:
		return false
	}
}

// syncMapUpdateVal asserts the map and value to the concrete type for T and updates the value.
func syncMapUpdateVal[T any](syncMapInterface any, op SyncOperation) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[T])
	if !ok {
		return false
	}

	v, ok := op.Value.(T)
	if !ok {
		return false
	}

	syncMap.UpdateVal(op.Key, v)

	return true
}

// processSyncMapUpdateVal handles value update operations for SyncMap instances.
func processSyncMapUpdateVal(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapUpdateVal[[]string](syncMapInterface, op)
	case MapTypeTwoString:
		return syncMapUpdateVal[[]DbstaticTwoStringOneInt](syncMapInterface, op)
	case MapTypeThreeString:
		return syncMapUpdateVal[[]DbstaticThreeStringTwoInt](syncMapInterface, op)
	case MapTypeTwoInt:
		return syncMapUpdateVal[[]DbstaticOneStringTwoInt](syncMapInterface, op)
	case MapTypeStructEmpty:
		return syncMapUpdateVal[struct{}](syncMapInterface, op)
	default:
		return false
	}
}

// syncMapMetaOp asserts the map to the concrete type for T and runs fn on it.
// Shared by the metadata-only operations (expire, lastscan, delete, check-expires)
// that do not need to assert a value type.
func syncMapMetaOp[T any](syncMapInterface any, fn func(*SyncMap[T])) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[T])
	if !ok {
		return false
	}

	fn(syncMap)

	return true
}

// dispatchSyncMapMetaOp routes a metadata-only operation to the concrete map type
// registered for op.MapType. The fns parameter bundles one closure per value type
// because Go generics cannot abstract over the type parameter at runtime.
func processSyncMapUpdateExpire(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]string]) {
			s.UpdateExpire(op.Key, op.Expires)
		})

	case MapTypeTwoString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticTwoStringOneInt]) {
			s.UpdateExpire(op.Key, op.Expires)
		})

	case MapTypeThreeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticThreeStringTwoInt]) {
			s.UpdateExpire(op.Key, op.Expires)
		})

	case MapTypeTwoInt:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticOneStringTwoInt]) {
			s.UpdateExpire(op.Key, op.Expires)
		})

	case MapTypeStructEmpty:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[struct{}]) {
			s.UpdateExpire(op.Key, op.Expires)
		})

	default:
		return false
	}
}

// processSyncMapUpdateLastscan handles last scan timestamp update operations for SyncMap instances.
func processSyncMapUpdateLastscan(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]string]) {
			s.UpdateLastscan(op.Key, op.Lastscan)
		})

	case MapTypeTwoString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticTwoStringOneInt]) {
			s.UpdateLastscan(op.Key, op.Lastscan)
		})

	case MapTypeThreeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticThreeStringTwoInt]) {
			s.UpdateLastscan(op.Key, op.Lastscan)
		})

	case MapTypeTwoInt:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticOneStringTwoInt]) {
			s.UpdateLastscan(op.Key, op.Lastscan)
		})

	case MapTypeStructEmpty:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[struct{}]) {
			s.UpdateLastscan(op.Key, op.Lastscan)
		})

	default:
		return false
	}
}

// processSyncMapDelete handles complete deletion operations for SyncMap instances.
func processSyncMapDelete(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]string]) {
			s.Delete(op.Key)
		})

	case MapTypeTwoString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticTwoStringOneInt]) {
			s.Delete(op.Key)
		})

	case MapTypeThreeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticThreeStringTwoInt]) {
			s.Delete(op.Key)
		})

	case MapTypeTwoInt:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticOneStringTwoInt]) {
			s.Delete(op.Key)
		})

	case MapTypeStructEmpty:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[struct{}]) {
			s.Delete(op.Key)
		})

	default:
		return false
	}
}

// workerMapOp asserts the map and value for SyncMapUint operations and applies fn.
func workerMapOp[T any](syncMapInterface any, op SyncOperation, fn func(*SyncMapUint[T], T)) bool {
	syncMap, ok := syncMapInterface.(*SyncMapUint[T])
	if !ok {
		return false
	}

	v, ok := op.Value.(T)
	if !ok {
		return false
	}

	fn(syncMap, v)

	return true
}

// processWorkerMapAdd handles Add operations for SyncMapUint instances used by worker pools.
func processWorkerMapAdd(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeSchedule:
		return workerMapOp(syncMapInterface, op, func(s *SyncMapUint[JobSchedule], v JobSchedule) {
			s.Add(op.KeyUint32, v)
		})

	case MapTypeQueue:
		return workerMapOp(syncMapInterface, op, func(s *SyncMapUint[Job], v Job) {
			s.Add(op.KeyUint32, v)
		})

	default:
		return false
	}
}

// processWorkerMapUpdate handles value update operations for SyncMapUint instances used by worker pools.
func processWorkerMapUpdate(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeSchedule:
		return workerMapOp(syncMapInterface, op, func(s *SyncMapUint[JobSchedule], v JobSchedule) {
			s.UpdateVal(op.KeyUint32, v)
		})

	case MapTypeQueue:
		return workerMapOp(syncMapInterface, op, func(s *SyncMapUint[Job], v Job) {
			s.UpdateVal(op.KeyUint32, v)
		})

	default:
		return false
	}
}

// processWorkerMapDelete handles deletion operations for SyncMapUint instances used by worker pools.
func processWorkerMapDelete(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeSchedule:
		if syncMap, ok := syncMapInterface.(*SyncMapUint[JobSchedule]); ok {
			syncMap.Delete(op.KeyUint32)
			return true
		}

	case MapTypeQueue:
		if syncMap, ok := syncMapInterface.(*SyncMapUint[Job]); ok {
			syncMap.Delete(op.KeyUint32)
			return true
		}

	default:
	}

	return false
}

// processWorkerMapDeleteQueue handles conditional deletion of jobs from worker queues.
// Removes jobs based on queue name and optionally filters by started status.
func processWorkerMapDeleteQueue(syncMapInterface any, op SyncOperation) bool {
	// Only queue operations support queue-based deletion
	if op.MapType != MapTypeQueue {
		return false
	}

	syncMap, ok := syncMapInterface.(*SyncMapUint[Job])
	if !ok {
		return false
	}

	syncMap.DeleteIf(func(_ uint32, job Job) bool {
		if job.Queue == op.Queue {
			if op.IsStarted {
				return !job.Started.IsZero()
			}

			return true
		}

		return false
	})

	return true
}

// sliceAppend asserts the map to SyncMap[[]T] and the value to element type T,
// then appends the value to the slice for op.Key unless it is already present.
// Keys without an existing entry are left untouched (matching historical behavior:
// appends only extend slices that a full Add created first).
func sliceAppend[T comparable](syncMapInterface any, op SyncOperation) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[[]T])
	if !ok {
		return false
	}

	value, ok := op.Value.(T)
	if !ok {
		return false
	}

	if syncMap.Check(op.Key) {
		current := syncMap.GetVal(op.Key)
		if slices.Contains(current, value) {
			return true // Already exists
		}

		syncMap.UpdateVal(op.Key, append(current, value))
	}

	return true
}

// processSliceAppend handles atomic append operations for slice-based SyncMap values.
func processSliceAppend(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return sliceAppend[string](syncMapInterface, op)
	case MapTypeTwoString:
		return sliceAppend[DbstaticTwoStringOneInt](syncMapInterface, op)
	case MapTypeThreeString:
		return sliceAppend[DbstaticThreeStringTwoInt](syncMapInterface, op)
	case MapTypeTwoInt:
		return sliceAppend[DbstaticOneStringTwoInt](syncMapInterface, op)
	default:
		return false
	}
}
