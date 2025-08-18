package syncops

import (
	"reflect"
	"slices"
	"sync"
	"time"
)

// SyncMap is an optimized version that only needs read locks since all writes
// go through the SyncOpsManager single-writer system
type SyncMap[T any] struct {
	m        map[string]T
	mp       map[string]*T
	expires  map[string]int64
	lastScan map[string]int64
	imdb     map[string]bool
	mu       sync.RWMutex // Only needed for read protection during writes
}

// NewSyncMap creates a new SyncMap with the specified initial size.
// The SyncMap is a thread-safe map that stores key-value pairs along with
// additional metadata such as expiration time, IMDB flag, and last scan time.
func NewSyncMap[T any](size int) *SyncMap[T] {
	return &SyncMap[T]{
		m:        make(map[string]T, size),
		mp:       make(map[string]*T, size),
		expires:  make(map[string]int64, size),
		lastScan: make(map[string]int64, size),
		imdb:     make(map[string]bool, size),
	}
}

// Add stores a new key-value pair in the SyncMap with associated metadata.
// The expires parameter sets expiration time (0 for no expiration), imdb indicates IMDB data,
// and lastscan tracks the last scan timestamp. This operation is NOT thread-safe and
// should only be called from the SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) Add(key string, value T, expires int64, imdb bool, lastscan int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	s.expires[key] = expires
	s.lastScan[key] = lastscan
	s.imdb[key] = imdb
}

// AddPointer stores a new key-value pair with automatic pointer management.
// If the value is not already a pointer, it creates a pointer reference for efficient access.
// Includes expiration, IMDB flag, and last scan metadata. This operation is NOT thread-safe
// and should only be called from the SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) AddPointer(key string, value T, expires int64, imdb bool, lastscan int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	if reflect.ValueOf(value).Kind() != reflect.Ptr {
		s.mp[key] = &value
	}
	s.expires[key] = expires
	s.lastScan[key] = lastscan
	s.imdb[key] = imdb
}

// UpdateVal modifies the value for an existing key in the SyncMap.
// Does not affect expiration time, IMDB flag, or last scan metadata.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateVal(key string, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// UpdateExpire modifies the expiration timestamp for an existing key.
// Only updates if the key exists and currently has a non-zero expiration time.
// Pass 0 to disable expiration or a Unix timestamp for specific expiry.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateExpire(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expires[key] != 0 {
		s.expires[key] = value
	}
}

// UpdateLastscan modifies the last scan timestamp for tracking purposes.
// Used to record when a key was last processed or accessed for maintenance operations.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) UpdateLastscan(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastScan[key] = value
}

// Delete removes a key and all its associated metadata from the SyncMap.
// This includes the value, pointer reference, expiration time, last scan time, and IMDB flag.
// This operation is NOT thread-safe and should only be called from the
// SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delete(key)
}

// delete is the internal helper that removes a key from all internal maps.
// Clears the value, pointer, expiration, last scan, and IMDB data for the key.
// This method assumes the caller already holds the appropriate lock.
func (s *SyncMap[T]) delete(key string) {
	delete(s.m, key)
	delete(s.mp, key)
	delete(s.expires, key)
	delete(s.lastScan, key)
	delete(s.imdb, key)
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
	return s.m[key]
}

// GetExpires returns the expiration time for the given key.
// This is a read operation and is safe for concurrent access.
func (s *SyncMap[T]) GetExpires(key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expires[key]
}

// GetLastScan returns the last scan time for the given key.
// This is a read operation and is safe for concurrent access.
func (s *SyncMap[T]) GetLastScan(key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastScan[key]
}

// GetIMDB returns the IMDB flag for the given key.
// This is a read operation and is safe for concurrent access.
func (s *SyncMap[T]) GetIMDB(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.imdb[key]
}

// GetExpire returns the expiration time for the given key (alias for GetExpires).
// This is a read operation and is safe for concurrent access.
func (s *SyncMap[T]) GetExpire(key string) int64 {
	return s.GetExpires(key)
}

// GetValP returns a pointer to the value associated with the specified key.
// Checks the pointer map first, then creates a pointer to the regular value if needed.
// Returns nil if the key doesn't exist. This is a thread-safe read operation
// that can be called concurrently from multiple goroutines.
func (s *SyncMap[T]) GetValP(key string) *T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ptr, exists := s.mp[key]; exists {
		return ptr
	}
	if val, exists := s.m[key]; exists {
		return &val
	}
	return nil
}

// CheckExpires determines if a key has passed its expiration time and optionally extends it.
// Returns true if the key is expired or doesn't exist. If extend is true and the key is expired,
// the expiration is extended by dur hours from the current time. Keys with 0 expiration never expire.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func (s *SyncMap[T]) CheckExpires(key string, extend bool, dur int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, exists := s.expires[key]
	if !exists || expiry == 0 {
		return false
	}

	now := time.Now().UnixNano()
	if now >= expiry {
		if extend {
			s.expires[key] = now + int64(dur)*int64(time.Hour)
		}
		return true
	}
	return false
}

// DeleteFunc deletes all entries for which the provided function returns true.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMap[T]) DeleteFunc(fn func(T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range s.m {
		if fn(value) {
			s.delete(key)
		}
	}
}

// DeleteFuncExpires deletes all entries for which the provided function returns true
// using the expiration time as the argument.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMap[T]) DeleteFuncExpires(fn func(int64) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.expires {
		if fn(s.expires[key]) {
			s.delete(key)
		}
	}
}

// DeleteFuncExpiresVal deletes all entries for which the provided function returns true
// and calls the value function with the value before deletion.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMap[T]) DeleteFuncExpiresVal(fn func(int64) bool, fnVal func(T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.expires {
		if fn(s.expires[key]) {
			fnVal(s.m[key])
			s.delete(key)
		}
	}
}

// DeleteFuncImdbVal deletes all entries for which the provided function returns true
// using the IMDB flag as the argument and calls the value function.
// This should only be called from the SyncOpsManager single writer.
func (s *SyncMap[T]) DeleteFuncImdbVal(fn func(bool) bool, fnVal func(T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.imdb {
		if fn(s.imdb[key]) {
			fnVal(s.m[key])
			s.delete(key)
		}
	}
}

// SyncMapUint is an optimized version for uint32 keys
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

// Range calls the provided function for each key-value pair in the map.
// This is a read operation and is safe for concurrent access.
func (s *SyncMapUint[T]) Range(fn func(uint32, T) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key, value := range s.m {
		if !fn(key, value) {
			break
		}
	}
}

// DeleteIf removes all entries for which the provided function returns true.
// This should only be called from the SyncOpsManager single writer.
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
	copy := make(map[uint32]T, len(s.m))
	for k, v := range s.m {
		copy[k] = v
	}
	return copy
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
// Pre-allocates extra capacity to reduce future allocations for better performance.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func AtomicAppendToStringSlice(sm *SyncMap[[]string], key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	current, exists := sm.m[key]
	if !exists {
		sm.m[key] = []string{value}
		return
	}

	// Check if value already exists
	for _, item := range current {
		if item == value {
			return // Already exists
		}
	}

	// Create new slice with extra capacity to reduce future allocations
	newSlice := make([]string, len(current), len(current)+10)
	copy(newSlice, current)
	newSlice = append(newSlice, value)
	sm.m[key] = newSlice
}

// AtomicRemoveFromStringSlice safely removes a string from a string slice stored in a SyncMap.
// Does nothing if the key doesn't exist or the value is not found in the slice.
// Creates a new slice without the specified value, maintaining order of remaining elements.
// This operation is NOT thread-safe and should only be called from the SyncOpsManager single writer goroutine.
func AtomicRemoveFromStringSlice(sm *SyncMap[[]string], key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	current, exists := sm.m[key]
	if !exists {
		return
	}

	// Check if value exists and create new slice without it
	if slices.Contains(current, value) {
		newSlice := make([]string, 0, len(current))
		for _, item := range current {
			if item != value {
				newSlice = append(newSlice, item)
			}
		}
		sm.m[key] = newSlice
	}
}
