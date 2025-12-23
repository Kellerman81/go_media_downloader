package syncops

import (
	"context"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"
)

// Database static types used in SyncMap operations.
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
	JobName        string
	ScheduleTyp    string
	ScheduleString string
	LastRun        time.Time
	NextRun        time.Time
	Interval       time.Duration
	CronID         cron.EntryID
	JobID          uint32
	ID             uint32
	CronSchedule   cron.Schedule
	IsRunning      bool
}

type SyncAny struct {
	Value any
}

// Operation types for the unified single-writer system.
type OpType string

const (
	// SyncMap operations (from logger package).
	OpSyncMapAdd          OpType = "syncMapAdd"
	OpSyncMapUpdateVal    OpType = "syncMapUpdateVal"
	OpSyncMapUpdateExpire OpType = "syncMapUpdateExpire"
	OpSyncMapUpdateLscan  OpType = "syncMapUpdateLastscan"
	OpSyncMapDelete       OpType = "syncMapDelete"

	// SyncMapUint operations (from worker package).
	OpWorkerMapAdd         OpType = "workerMapAdd"
	OpWorkerMapUpdate      OpType = "workerMapUpdate"
	OpWorkerMapDelete      OpType = "workerMapDelete"
	OpWorkerMapDeleteQueue OpType = "workerMapDeleteQueue"

	// Cache operations (from database package).
	OpCacheAppend OpType = "cacheAppend"
	OpCacheRemove OpType = "cacheRemove"
	OpCacheDelete OpType = "cacheDelete"
	OpCacheRegex  OpType = "cacheRegex"
	OpCacheStmt   OpType = "cacheStmt"

	// Atomic slice operations.
	OpSliceAppend OpType = "sliceAppend"
	OpSliceRemove OpType = "sliceRemove"

	// Advanced SyncMap operations.
	OpSyncMapCheckExpires         OpType = "syncMapCheckExpires"
	OpSyncMapDeleteFunc           OpType = "syncMapDeleteFunc"
	OpSyncMapDeleteFuncExpires    OpType = "syncMapDeleteFuncExpires"
	OpSyncMapDeleteFuncExpiresVal OpType = "syncMapDeleteFuncExpiresVal"
	OpSyncMapDeleteFuncImdbVal    OpType = "syncMapDeleteFuncImdbVal"
	OpAtomicAppendString          OpType = "atomicAppendString"
	OpAtomicRemoveString          OpType = "atomicRemoveString"

	// Control operations.
	OpFlush OpType = "flush"
)

// MapType identifies which map to operate on.
type MapType string

const (
	// Logger SyncMap types.
	MapTypeString      MapType = "string"
	MapTypeTwoString   MapType = "twostring"
	MapTypeThreeString MapType = "threestring"
	MapTypeTwoInt      MapType = "twoint"
	MapTypeXStmt       MapType = "xstmt"
	MapTypeRegex       MapType = "regex"
	MapTypeStructEmpty MapType = "structempty"
	// MapTypeInterface   MapType = "interface".
	MapTypeAny MapType = "any"

	// API Client SyncMap types.
	MapTypeNewznab    MapType = "newznab"
	MapTypeApprise    MapType = "apprise"
	MapTypeGotify     MapType = "gotify"
	MapTypePushbullet MapType = "pushbullet"
	MapTypePushover   MapType = "pushover"

	// Worker SyncMapUint types.
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
	UseSeries bool
	Force     bool

	// Additional fields for advanced operations
	Extend     bool
	Duration   int
	FilterFunc any // Function for DeleteFunc operations
	ValueFunc  any // Function for DeleteFuncExpiresVal operations

	// Worker-specific fields
	Queue     string // Queue name for worker operations
	IsStarted bool   // For queue-based deletion
}

// SyncOpsManager manages all synchronized operations across the application.
type SyncOpsManager struct {
	opQueue      chan SyncOperation
	writerActive bool
	writerMu     sync.Mutex
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

	// Pool for reusing SyncOperation objects.
	plSyncOp pool.Poolobj[SyncOperation]
)

// init initializes the SyncOperation object pool for efficient memory reuse.
// Pool is configured to maintain 100 objects with 10 pre-allocated, following
// the established pooling pattern used throughout the application.
func init() {
	plSyncOp.Init(100, 10,
		func(op *SyncOperation) {
			// Initialize the operation object
			if op.ResultCh == nil {
				op.ResultCh = make(chan bool, 1)
			}
			// Reset all fields to zero values
			op.OpType = ""
			op.MapType = ""
			op.Key = ""
			op.KeyUint32 = 0
			op.Value = nil
			op.Expires = 0
			op.IMDB = false
			op.Lastscan = 0
			op.UseSeries = false
			op.Force = false
			op.Extend = false
			op.Duration = 0
			op.FilterFunc = nil
			op.ValueFunc = nil
			op.Queue = ""
			op.IsStarted = false
		},
		func(op *SyncOperation) bool {
			// Cleanup before returning to pool
			// Ensure channel is drained to prevent blocking
			select {
			case <-op.ResultCh:
			default:
			}

			// Keep the object in pool (return false to retain)
			return false
		})
}

// InitSyncOps initializes the global synchronized operations manager using a singleton pattern.
// Creates a single-writer goroutine that processes all synchronization operations to ensure
// thread safety across the application. Must be called once before using any sync operations.
// Subsequent calls are ignored due to sync.Once protection.
func InitSyncOps() {
	initOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())

		manager = &SyncOpsManager{
			opQueue:      make(chan SyncOperation, 10000),
			writerActive: false,
			ctx:          ctx,
			cancel:       cancel,
			syncMaps:     make(map[MapType]any),
		}
		manager.startWriter()
	})
}

// RegisterSyncMap registers a SyncMap or SyncMapUint instance for synchronized operations.
// Associates the provided map with a specific MapType identifier, allowing the operations
// manager to route operations to the correct map. Automatically initializes the manager
// if not already done. Thread-safe and can be called from multiple goroutines.
func RegisterSyncMap(mapType MapType, syncMap any) {
	if manager == nil {
		InitSyncOps()
	}

	manager.syncMapsMu.Lock()
	defer manager.syncMapsMu.Unlock()

	manager.syncMaps[mapType] = syncMap
}

// QueueOperation submits a synchronization operation to the single-writer goroutine for processing.
// Blocks until the operation completes, ensuring atomic execution. Uses pooled operation objects
// for efficient memory reuse. Uses a fallback retry mechanism if the queue is full to guarantee delivery.
// This is the core function that ensures all map operations are serialized and thread-safe.
func QueueOperation(op SyncOperation) {
	if manager == nil || !manager.writerActive {
		return
	}

	// Get a pooled operation object
	pooledOp := plSyncOp.Get()
	defer plSyncOp.Put(pooledOp)

	// Copy operation data to pooled object
	*pooledOp = op

	// Ensure result channel is available
	if pooledOp.ResultCh == nil {
		pooledOp.ResultCh = make(chan bool, 1)
	}

	// Send operation and wait for completion
	select {
	case manager.opQueue <- *pooledOp:
		<-pooledOp.ResultCh // Wait for operation to complete
	default:
		// Queue is full, keep trying until we can insert the operation
		for {
			select {
			case manager.opQueue <- *pooledOp:
				<-pooledOp.ResultCh // Wait for operation to complete
				return

			default:
				time.Sleep(time.Millisecond)
			}
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

// QueueSliceRemove queues an atomic removal operation for slice-based SyncMap values.
// Safely removes specific elements from slices stored in SyncMaps while maintaining order.
// Creates new slices without the specified value, optimizing memory allocation for performance.
func QueueSliceRemove(mapType MapType, key string, value any) {
	QueueOperation(SyncOperation{
		OpType:  OpSliceRemove,
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
// Provides flexible cleanup capabilities based on complex business logic conditions.
func QueueSyncMapDeleteFunc(mapType MapType, filterFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFunc,
		MapType:    mapType,
		FilterFunc: filterFunc,
	})
}

// QueueSyncMapDeleteFuncExpires queues deletion based on expiration time filtering.
// Removes entries where the filter function returns true when applied to the expiration timestamp.
// Useful for implementing custom cache eviction policies and time-based cleanup strategies.
func QueueSyncMapDeleteFuncExpires(mapType MapType, filterFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFuncExpires,
		MapType:    mapType,
		FilterFunc: filterFunc,
	})
}

// QueueSyncMapDeleteFuncExpiresVal queues deletion with expiration filtering and value processing.
// Combines expiration-based filtering with value callback execution before deletion.
// The filter function checks expiration times, and the value function processes each deleted entry.
func QueueSyncMapDeleteFuncExpiresVal(mapType MapType, filterFunc any, valueFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFuncExpiresVal,
		MapType:    mapType,
		FilterFunc: filterFunc,
		ValueFunc:  valueFunc,
	})
}

// QueueSyncMapDeleteFuncImdbVal queues deletion based on IMDB flag filtering with value processing.
// Designed specifically for IMDB data management with boolean condition filtering.
// The filter function receives IMDB flags and the value function processes deleted entries.
func QueueSyncMapDeleteFuncImdbVal(mapType MapType, filterFunc any, valueFunc any) {
	QueueOperation(SyncOperation{
		OpType:     OpSyncMapDeleteFuncImdbVal,
		MapType:    mapType,
		FilterFunc: filterFunc,
		ValueFunc:  valueFunc,
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

// Shutdown gracefully terminates the synchronized operations manager and its writer goroutine.
// Cancels the context to signal shutdown, closes the operation queue channel, and marks
// the writer as inactive. Should be called during application shutdown to ensure clean exit.
// Thread-safe and can be called multiple times without adverse effects.
func Shutdown() {
	if manager != nil {
		manager.cancel()
		manager.writerMu.Lock()

		if manager.writerActive && manager.opQueue != nil {
			close(manager.opQueue)

			manager.writerActive = false
		}

		manager.writerMu.Unlock()
	}
}

// startWriter launches the single writer goroutine that processes all synchronization operations.
// Ensures only one writer goroutine is active at a time using mutex protection. The goroutine
// runs until the context is cancelled, processing operations from the queue and sending
// results back to callers. This is the core of the thread-safety mechanism.
func (m *SyncOpsManager) startWriter() {
	m.writerMu.Lock()
	defer m.writerMu.Unlock()

	if m.writerActive {
		return
	}

	m.writerActive = true

	go func() {
		defer func() {
			m.writerMu.Lock()

			m.writerActive = false
			m.writerMu.Unlock()
		}()

		for {
			select {
			case <-m.ctx.Done():
				return
			case op := <-m.opQueue:
				success := m.processOperation(op)
				if op.ResultCh != nil {
					select {
					case op.ResultCh <- success:
					default:
						// Non-blocking send to avoid deadlocks
					}
				}
			}
		}
	}()
}

// processOperation dispatches a single synchronization operation to the appropriate handler.
// Uses the operation type to determine which specific processing function to call.
// Returns true if the operation was successfully processed, false otherwise.
// This is the central dispatcher for all operation types in the single-writer system.
func (m *SyncOpsManager) processOperation(op SyncOperation) bool {
	m.syncMapsMu.RLock()
	defer m.syncMapsMu.RUnlock()

	switch op.OpType {
	case OpSyncMapAdd:
		return m.processSyncMapAdd(op)
	case OpSyncMapUpdateVal:
		return m.processSyncMapUpdateVal(op)
	case OpSyncMapUpdateExpire:
		return m.processSyncMapUpdateExpire(op)
	case OpSyncMapUpdateLscan:
		return m.processSyncMapUpdateLastscan(op)
	case OpSyncMapDelete:
		return m.processSyncMapDelete(op)
	case OpWorkerMapAdd:
		return m.processWorkerMapAdd(op)
	case OpWorkerMapUpdate:
		return m.processWorkerMapUpdate(op)
	case OpWorkerMapDelete:
		return m.processWorkerMapDelete(op)
	case OpWorkerMapDeleteQueue:
		return m.processWorkerMapDeleteQueue(op)
	case OpSliceAppend:
		return m.processSliceAppend(op)
	case OpSliceRemove:
		return m.processSliceRemove(op)
	case OpSyncMapCheckExpires:
		return m.processSyncMapCheckExpires(op)
	case OpSyncMapDeleteFunc:
		return m.processSyncMapDeleteFunc(op)
	case OpSyncMapDeleteFuncExpires:
		return m.processSyncMapDeleteFuncExpires(op)
	case OpSyncMapDeleteFuncExpiresVal:
		return m.processSyncMapDeleteFuncExpiresVal(op)
	case OpSyncMapDeleteFuncImdbVal:
		return m.processSyncMapDeleteFuncImdbVal(op)
	case OpAtomicAppendString:
		return m.processAtomicAppendString(op)
	case OpAtomicRemoveString:
		return m.processAtomicRemoveString(op)
	case OpFlush:
		return true // Flush operation - just return success
	default:
		return false
	}
}

// processSyncMapAdd handles Add operations for SyncMap instances.
// Performs type checking to ensure the operation matches the registered map type,
// then delegates to the appropriate SyncMap's Add method. Supports all SyncMap types
// including string slices, database static types, SQL statements, and regex patterns.
func (m *SyncOpsManager) processSyncMapAdd(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if v, ok := op.Value.([]string); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if v, ok := op.Value.([]DbstaticTwoStringOneInt); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if v, ok := op.Value.([]DbstaticThreeStringTwoInt); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if v, ok := op.Value.([]DbstaticOneStringTwoInt); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			if v, ok := op.Value.(*sqlx.Stmt); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			if v, ok := op.Value.(*regexp.Regexp); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeStructEmpty:
		if syncMap, ok := syncMapInterface.(*SyncMap[struct{}]); ok {
			if v, ok := op.Value.(struct{}); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			}
		}

	case MapTypeAny,
		MapTypeNewznab,
		MapTypeApprise,
		MapTypeGotify,
		MapTypePushbullet,
		MapTypePushover:
		if syncMap, ok := syncMapInterface.(*SyncMap[SyncAny]); ok {
			if v, ok := op.Value.(SyncAny); ok {
				syncMap.Add(op.Key, v, op.Expires, op.IMDB, op.Lastscan)
				return true
			} else {
				logger.Logtype("error", 0).Any("type", reflect.TypeOf(op.Value).String()).Msg("Value is not of Type SyncAny")
			}
		} else {
			t := reflect.TypeOf(syncMapInterface)
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}

			logger.Logtype("error", 0).Any("type", t.String()).Any("Kind", t.Kind().String()).Msg("Queue is not of Type SyncMap[SyncAny]")
		}
	}

	return false
}

// processSyncMapUpdateVal handles value update operations for SyncMap instances.
// Performs type checking and delegates to the appropriate SyncMap's UpdateVal method.
// Preserves all metadata (expiration, IMDB flag, last scan) while updating only the value.
func (m *SyncOpsManager) processSyncMapUpdateVal(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if v, ok := op.Value.([]string); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if v, ok := op.Value.([]DbstaticTwoStringOneInt); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if v, ok := op.Value.([]DbstaticThreeStringTwoInt); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if v, ok := op.Value.([]DbstaticOneStringTwoInt); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			if v, ok := op.Value.(*sqlx.Stmt); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			if v, ok := op.Value.(*regexp.Regexp); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeStructEmpty:
		if syncMap, ok := syncMapInterface.(*SyncMap[struct{}]); ok {
			if v, ok := op.Value.(struct{}); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}
		}

	case MapTypeAny,
		MapTypeNewznab,
		MapTypeApprise,
		MapTypeGotify,
		MapTypePushbullet,
		MapTypePushover:
		if syncMap, ok := syncMapInterface.(*SyncMap[SyncAny]); ok {
			if v, ok := op.Value.(SyncAny); ok {
				syncMap.UpdateVal(op.Key, v)
				return true
			}

			return true
		}
	}

	return false
}

// processSyncMapUpdateExpire handles expiration time update operations for SyncMap instances.
// Updates only the expiration timestamp while preserving values, IMDB flags, and last scan data.
// Supports all SyncMap types with consistent behavior across different data structures.
func (m *SyncOpsManager) processSyncMapUpdateExpire(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeStructEmpty:
		if syncMap, ok := syncMapInterface.(*SyncMap[struct{}]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}

	case MapTypeAny,
		MapTypeNewznab,
		MapTypeApprise,
		MapTypeGotify,
		MapTypePushbullet,
		MapTypePushover:
		if syncMap, ok := syncMapInterface.(*SyncMap[SyncAny]); ok {
			syncMap.UpdateExpire(op.Key, op.Expires)
			return true
		}
	}

	return false
}

// processSyncMapUpdateLastscan handles last scan timestamp update operations for SyncMap instances.
// Updates tracking metadata for maintenance operations while preserving values, expiration, and IMDB flags.
// Used for recording when entries were last processed or accessed for monitoring purposes.
func (m *SyncOpsManager) processSyncMapUpdateLastscan(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeStructEmpty:
		if syncMap, ok := syncMapInterface.(*SyncMap[struct{}]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}

	case MapTypeAny,
		MapTypeNewznab,
		MapTypeApprise,
		MapTypeGotify,
		MapTypePushbullet,
		MapTypePushover:
		if syncMap, ok := syncMapInterface.(*SyncMap[SyncAny]); ok {
			syncMap.UpdateLastscan(op.Key, op.Lastscan)
			return true
		}
	}

	return false
}

// processSyncMapDelete handles complete deletion operations for SyncMap instances.
// Removes keys and all associated metadata including values, expiration, IMDB flags, and last scan data.
// Supports all SyncMap types with consistent cleanup behavior across different data structures.
func (m *SyncOpsManager) processSyncMapDelete(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeStructEmpty:
		if syncMap, ok := syncMapInterface.(*SyncMap[struct{}]); ok {
			syncMap.Delete(op.Key)
			return true
		}

	case MapTypeAny,
		MapTypeNewznab,
		MapTypeApprise,
		MapTypeGotify,
		MapTypePushbullet,
		MapTypePushover:
		if syncMap, ok := syncMapInterface.(*SyncMap[SyncAny]); ok {
			syncMap.Delete(op.Key)
			return true
		}
	}

	return false
}

// processWorkerMapAdd handles Add operations for SyncMapUint instances used by worker pools.
// Supports job queue and schedule management with uint32 keys. Performs type validation
// to ensure Job and JobSchedule types are correctly stored in their respective maps.
func (m *SyncOpsManager) processWorkerMapAdd(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeSchedule:
		if syncMap, ok := syncMapInterface.(*SyncMapUint[JobSchedule]); ok {
			if v, ok := op.Value.(JobSchedule); ok {
				syncMap.Add(op.KeyUint32, v)
				return true
			}
		}

	case MapTypeQueue:
		t := reflect.TypeOf(syncMapInterface)
		// If it's a pointer, get the underlying type
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if syncMap, ok := syncMapInterface.(*SyncMapUint[Job]); ok {
			if v, ok := op.Value.(Job); ok {
				syncMap.Add(op.KeyUint32, v)
				return true
			} else {
				logger.Logtype("error", 0).Any("type", reflect.TypeOf(op.Value)).Msg("Value is not of Type Job")
			}
		} else {
			logger.Logtype("error", 0).Any("type", t.String()).Any("Kind", t.Kind().String()).Msg("Queue is not of Type SyncMapUint[Job]")
		}
	}

	return false
}

// processWorkerMapUpdate handles value update operations for SyncMapUint instances used by worker pools.
// Updates existing job or schedule entries with new values while maintaining uint32 key associations.
// Supports JobSchedule and Job types with type validation for worker pool data integrity.
func (m *SyncOpsManager) processWorkerMapUpdate(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeSchedule:
		if syncMap, ok := syncMapInterface.(*SyncMapUint[JobSchedule]); ok {
			if v, ok := op.Value.(JobSchedule); ok {
				syncMap.UpdateVal(op.KeyUint32, v)
				return true
			}
		}

	case MapTypeQueue:
		if syncMap, ok := syncMapInterface.(*SyncMapUint[Job]); ok {
			if v, ok := op.Value.(Job); ok {
				syncMap.UpdateVal(op.KeyUint32, v)
				return true
			}
		}
	}

	return false
}

// processWorkerMapDelete handles deletion operations for SyncMapUint instances used by worker pools.
// Removes jobs or schedules identified by uint32 keys from worker pool management maps.
// Used for cleanup operations when jobs complete or schedules are cancelled.
func (m *SyncOpsManager) processWorkerMapDelete(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

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
	}

	return false
}

// processSliceAppend handles atomic append operations for slice-based SyncMap values.
// Checks for duplicates before appending to prevent redundant entries. Creates new slices
// with extra capacity for performance optimization. Supports string slices and various
// database static type slices with type-safe operations.
func (m *SyncOpsManager) processSliceAppend(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if value, ok := op.Value.(string); ok {
				// Atomic append operation
				if syncMap.Check(op.Key) {
					current := syncMap.GetVal(op.Key)
					for _, item := range current {
						if item == value {
							return true // Already exists
						}
					}

					newSlice := make([]string, len(current), len(current)+10)
					copy(newSlice, current)

					newSlice = append(newSlice, value)
					syncMap.UpdateVal(op.Key, newSlice)
				}

				return true
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if value, ok := op.Value.(DbstaticThreeStringTwoInt); ok {
				if syncMap.Check(op.Key) {
					current := syncMap.GetVal(op.Key)
					for _, item := range current {
						if item == value {
							return true // Already exists
						}
					}

					newSlice := make([]DbstaticThreeStringTwoInt, len(current), len(current)+10)
					copy(newSlice, current)

					newSlice = append(newSlice, value)
					syncMap.UpdateVal(op.Key, newSlice)
				}

				return true
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if value, ok := op.Value.(DbstaticTwoStringOneInt); ok {
				if syncMap.Check(op.Key) {
					current := syncMap.GetVal(op.Key)
					for _, item := range current {
						if item == value {
							return true // Already exists
						}
					}

					newSlice := make([]DbstaticTwoStringOneInt, len(current), len(current)+10)
					copy(newSlice, current)

					newSlice = append(newSlice, value)
					syncMap.UpdateVal(op.Key, newSlice)
				}

				return true
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if value, ok := op.Value.(DbstaticOneStringTwoInt); ok {
				if syncMap.Check(op.Key) {
					current := syncMap.GetVal(op.Key)
					for _, item := range current {
						if item == value {
							return true // Already exists
						}
					}

					newSlice := make([]DbstaticOneStringTwoInt, len(current), len(current)+10)
					copy(newSlice, current)

					newSlice = append(newSlice, value)
					syncMap.UpdateVal(op.Key, newSlice)
				}

				return true
			}
		}
	}

	return false
}

// processSliceRemove handles atomic removal operations for slice-based SyncMap values.
// Searches for the specified value in the slice and creates a new slice without it.
// Maintains order of remaining elements and optimizes memory allocation. Currently
// supports string slices with potential for extension to other slice types.
func (m *SyncOpsManager) processSliceRemove(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if value, ok := op.Value.(string); ok {
				if syncMap.Check(op.Key) {
					current := syncMap.GetVal(op.Key)

					found := false
					for _, item := range current {
						if item == value {
							found = true
							break
						}
					}

					if found {
						newSlice := make([]string, 0, len(current))
						for _, item := range current {
							if item != value {
								newSlice = append(newSlice, item)
							}
						}

						syncMap.UpdateVal(op.Key, newSlice)
					}
				}

				return true
			}
		}
	}

	return false
}

// processWorkerMapDeleteQueue handles conditional deletion of jobs from worker queues.
// Removes jobs based on queue name and optionally filters by started status.
// Uses the SyncMapUint's DeleteIf method to efficiently remove matching entries.
// Specifically designed for worker pool management and job cleanup operations.
func (m *SyncOpsManager) processWorkerMapDeleteQueue(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	// Only queue operations support queue-based deletion
	if op.MapType == MapTypeQueue {
		if syncMap, ok := syncMapInterface.(*SyncMapUint[Job]); ok {
			syncMap.DeleteIf(func(key uint32, job Job) bool {
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
	}

	return false
}
