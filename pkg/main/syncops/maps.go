package syncops

import (
	"maps"
	"slices"
	"sync"
	"time"
)

// smEntry holds a value together with its metadata in a single map slot so that
// every operation hashes the key once and the value and its metadata can never
// drift out of sync.
type smEntry[T any] struct {
	value    T
	expires  int64
	lastScan int64
	imdb     bool
}

// SyncMap is an optimized version that only needs read locks since all writes
// go through the SyncOpsManager single-writer system.
type SyncMap[T any] struct {
	m  map[string]smEntry[T]
	mu sync.RWMutex // Only needed for read protection during writes
}

// NewSyncMap creates a new SyncMap with the specified initial size.
// The SyncMap is a thread-safe map that stores key-value pairs along with
// additional metadata such as expiration time, IMDB flag, and last scan time.
func NewSyncMap[T any](size int) *SyncMap[T] {
	return &SyncMap[T]{
		m: make(map[string]smEntry[T], size),
	}
}

// Add stores a new key-value pair in the SyncMap with associated metadata.
// The expires parameter sets expiration time (0 for no expiration), imdb indicates IMDB data,
// and lastscan tracks the last scan timestamp. This operation is NOT thread-safe and
// should only be called from the SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) Add(key string, value T, expires int64, imdb bool, lastscan int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[key] = smEntry[T]{value: value, expires: expires, lastScan: lastscan, imdb: imdb}
}

// AddIfAbsent atomically stores the key-value pair only when the key is not
// already present (or its existing entry has expired) and returns true.
// Returns false when a live entry for the key already exists. This closes the
// check-then-add race of calling Check followed by Add from concurrent
// goroutines: exactly one caller wins for a given key.
func (s *SyncMap[T]) AddIfAbsent(key string, value T, expires int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.m[key]; ok {
		if e.expires == 0 || time.Now().UnixNano() < e.expires {
			return false
		}

		// The previous holder's entry expired (e.g. a crashed or hung job) -
		// take the key over.
	}

	s.m[key] = smEntry[T]{value: value, expires: expires}

	return true
}

// ModifyInPlace calls fn on the value stored for key.
// Intended for reference-type values (maps, slices) that need in-place mutation
// without replacing the outer SyncMap entry. No-ops if the key does not exist.
// fn is called outside any lock so it is safe for fn to call other SyncMap or
// QueueOperation functions without risking a deadlock.
func (s *SyncMap[T]) ModifyInPlace(key string, fn func(T)) {
	s.mu.RLock()

	e, ok := s.m[key]
	s.mu.RUnlock()

	if ok {
		fn(e.value)
	}
}

// UpdateVal modifies the value for an existing key in the SyncMap.
// Does not affect expiration time, IMDB flag, or last scan metadata.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateVal(key string, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := s.m[key]

	e.value = value
	s.m[key] = e
}

// UpdateExpire modifies the expiration timestamp for an existing key.
// Only updates if the key exists and currently has a non-zero expiration time.
// Pass 0 to disable expiration or a Unix timestamp for specific expiry.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateExpire(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.m[key]; ok && e.expires != 0 {
		e.expires = value
		s.m[key] = e
	}
}

// UpdateLastscan modifies the last scan timestamp for tracking purposes.
// Used to record when a key was last processed or accessed for maintenance operations.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateLastscan(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.m[key]; ok {
		e.lastScan = value
		s.m[key] = e
	}
}

// Delete removes a key and all its associated metadata from the SyncMap.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.m, key)
}

// Check tests whether a key exists in the SyncMap.
// Returns true if the key is present, false otherwise. This is a thread-safe
// read operation that can be called concurrently from multiple goroutines.
func (s *SyncMap[T]) Check(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.m[key]

	return ok
}

// GetVal retrieves the value associated with the specified key.
// Returns the zero value of type T if the key doesn't exist. This is a thread-safe
// read operation that can be called concurrently from multiple goroutines.
func (s *SyncMap[T]) GetVal(key string) T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key].value
}

// GetLastScan returns the last scan time for the given key.
// This is a read operation and is safe for concurrent access.
func (s *SyncMap[T]) GetLastScan(key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key].lastScan
}

// CheckExpires determines if a key has passed its expiration time and optionally extends it.
// Returns true if the key is expired or doesn't exist. If extend is true and the key is expired,
// the expiration is extended by dur hours from the current time. Keys with 0 expiration never expire.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) CheckExpires(key string, extend bool, dur int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, exists := s.m[key]
	if !exists || e.expires == 0 {
		return false
	}

	now := time.Now().UnixNano()
	if now >= e.expires {
		if extend {
			e.expires = now + int64(dur)*int64(time.Hour)
			s.m[key] = e
		}

		return true
	}

	return false
}

// DeleteFunc deletes all entries for which the provided function returns true.
// The whole scan-and-delete runs under the write lock, so fn must not call other
// SyncMap or QueueOperation functions. In the single-writer model that is already
// the contract: deletions only run inside the writer goroutine.
func (s *SyncMap[T]) DeleteFunc(fn func(T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, e := range s.m {
		if fn(e.value) {
			delete(s.m, key)
		}
	}
}

// DeleteFuncExpires deletes all entries for which the provided function returns true
// using the expiration time as the argument. Runs entirely under the write lock.
func (s *SyncMap[T]) DeleteFuncExpires(fn func(int64) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, e := range s.m {
		if fn(e.expires) {
			delete(s.m, key)
		}
	}
}

// SyncMapUint is an optimized version for uint32 keys.
type SyncMapUint[T any] struct {
	m  map[uint32]T
	mu sync.RWMutex // Only needed for read protection during writes
}

// NewSyncMapUint creates a new SyncMapUint with the specified initial size.
// SyncMapUint is optimized for uint32 keys and provides simpler functionality than SyncMap.
// It doesn't include expiration, IMDB flags, or last scan tracking features.
func NewSyncMapUint[T any](size int) *SyncMapUint[T] {
	return &SyncMapUint[T]{
		m: make(map[uint32]T, size),
	}
}

// Add adds a new key-value pair to the SyncMapUint.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMapUint[T]) Add(key uint32, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[key] = value
}

// UpdateVal modifies the value for an existing key in the SyncMapUint.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMapUint[T]) UpdateVal(key uint32, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m[key] = value
}

// Delete removes the given key from the SyncMapUint.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMapUint[T]) Delete(key uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.m, key)
}

// Check returns true if the given key exists in the SyncMapUint.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) Check(key uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.m[key]

	return ok
}

// GetVal returns the value associated with the given key.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) GetVal(key uint32) T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key]
}

// DeleteIf removes all entries for which the provided function returns true.
// The whole scan-and-delete runs under the write lock, so fn must not call other
// SyncMap or QueueOperation functions. In the single-writer model that is already
// the contract: deletions only run inside the writer goroutine.
func (s *SyncMapUint[T]) DeleteIf(fn func(uint32, T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, value := range s.m {
		if fn(key, value) {
			delete(s.m, key)
		}
	}
}

// ForEach executes a function for each key-value pair in the map while holding a read lock.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) ForEach(fn func(uint32, T)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for key, value := range s.m {
		fn(key, value)
	}
}

// GetMap returns a copy of the underlying map stored in the SyncMapUint.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) GetMap() map[uint32]T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Create a copy to prevent external modification
	cp := make(map[uint32]T, len(s.m))
	maps.Copy(cp, s.m)

	return cp
}

// FindFirst searches for the first element that matches the predicate function.
// Returns the key, value, and whether a match was found.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) FindFirst(predicate func(uint32, T) bool) (uint32, T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var zero T

	for k, v := range s.m {
		if predicate(k, v) {
			return k, v, true
		}
	}

	return 0, zero, false
}

// Exists checks if any entry matches the given predicate function.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) Exists(predicate func(uint32, T) bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for k, v := range s.m {
		if predicate(k, v) {
			return true
		}
	}

	return false
}

// AtomicAppendToStringSlice safely appends a string to a string slice stored in a SyncMap.
// Creates a new slice if the key doesn't exist, and checks for duplicates before appending.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func AtomicAppendToStringSlice(sm *SyncMap[[]string], key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	e, exists := sm.m[key]
	if !exists {
		sm.m[key] = smEntry[[]string]{value: []string{value}}
		return
	}

	// Check if value already exists
	if slices.Contains(e.value, value) {
		return // Already exists
	}

	e.value = append(e.value, value)
	sm.m[key] = e
}

// AtomicRemoveFromStringSlice safely removes a string from a string slice stored in a SyncMap.
// Does nothing if the key doesn't exist or the value is not found in the slice.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func AtomicRemoveFromStringSlice(sm *SyncMap[[]string], key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	e, exists := sm.m[key]
	if !exists {
		return
	}

	if !slices.Contains(e.value, value) {
		return
	}

	newSlice := make([]string, 0, len(e.value))
	for i := range e.value {
		if e.value[i] != value {
			newSlice = append(newSlice, e.value[i])
		}
	}

	e.value = newSlice
	sm.m[key] = e
}
