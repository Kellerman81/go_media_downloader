package logger

import (
	"sync"
	"time"
)

type SyncMap[T any] struct {
	m        map[string]T
	expires  map[string]int64
	lastScan map[string]int64
	imdb     map[string]bool
	mu       sync.Mutex
}

// NewSyncMap creates a new SyncMap with the specified initial size.
// The SyncMap is a thread-safe map that stores key-value pairs along with
// additional metadata such as expiration time, IMDB flag, and last scan time.
// The initial size of the underlying maps is set to the provided size parameter.
func NewSyncMap[T any](size int) *SyncMap[T] {
	return &SyncMap[T]{
		m:        make(map[string]T, size),
		expires:  make(map[string]int64, size),
		lastScan: make(map[string]int64, size),
		imdb:     make(map[string]bool, size),
	}
}

// Add adds a new key-value pair to the SyncMap, along with its expiration time, IMDB flag, and last scan time.
// The method acquires a write lock on the SyncMap before adding the new entry,
// and releases the lock before returning.
func (s *SyncMap[T]) Add(key string, value T, expires int64, imdb bool, lastscan int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	s.expires[key] = expires
	s.lastScan[key] = lastscan
	s.imdb[key] = imdb
}

// UpdateVal updates the value associated with the given key in the SyncMap.
func (s *SyncMap[T]) UpdateVal(key string, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// UpdateExpire updates the expiration time for the given key in the SyncMap.
func (s *SyncMap[T]) UpdateExpire(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expires[key] != 0 {
		s.expires[key] = value
	}
}

// UpdateLastscan updates the last scan time for the given key in the SyncMap.
func (s *SyncMap[T]) UpdateLastscan(key string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastScan[key] = value
}

// Delete removes the given key from the SyncMap, including its associated value, expiration time,
// last scan time, and IMDB flag. This is an internal helper function and is not part of the
// public API.
func (s *SyncMap[T]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delete(key)
}

// delete removes the given key from the SyncMap, including its associated value, expiration time,
// last scan time, and IMDB flag. This is an internal helper function and is not part of the
// public API.
func (s *SyncMap[T]) delete(key string) {
	// LogDynamicany1String("debug", "cache delete entry", "key", key)
	delete(s.m, key)
	delete(s.expires, key)
	delete(s.lastScan, key)
	delete(s.imdb, key)
}

// GetVal returns the value associated with the given key in the SyncMap.
// The method acquires a read lock on the SyncMap before retrieving the value,
// and releases the lock before returning the value.
func (s *SyncMap[T]) GetVal(key string) T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key]
}

// GetValP returns a pointer to the value associated with the given key in the SyncMap.
// If the key does not exist in the SyncMap, it returns nil.
// The method acquires a read lock on the SyncMap before retrieving the value,
// and releases the lock before returning the pointer.
func (s *SyncMap[T]) GetValP(key string) *T {
	if !s.Check(key) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	val := s.m[key]
	return &val
}

// CheckExpires checks if the given key in the SyncMap has expired. If the key has expired and
// extend is false, it logs a warning and returns true. If extend is true, it updates the
// expiration time of the key by adding the given duration (in hours) to the current time.
// If the key does not exist or has not expired, it returns false.
func (s *SyncMap[T]) CheckExpires(key string, extend bool, dur int) bool {
	if !s.Check(key) {
		return false
	}
	expires := s.GetExpire(key)
	if expires != 0 && expires < time.Now().UnixNano() {
		if !extend {
			LogDynamicany1String("warn", "refresh cache expired", "cache", key)
			return true
		}
		s.UpdateExpire(key, time.Now().Add(time.Duration(dur)*time.Hour).UnixNano())
	}
	return false
}

// GetExpire returns the expiration time for the given key in the SyncMap. If the key does not exist,
// it returns 0.
func (s *SyncMap[T]) GetExpire(key string) int64 {
	if !s.Check(key) {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.expires[key]
}

// GetLastscan returns the last scan time for the given key in the SyncMap. If the key does not exist,
// it returns 0.
func (s *SyncMap[T]) GetLastscan(key string) int64 {
	if !s.Check(key) {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastScan[key]
}

// Check returns true if the given key exists in the SyncMap, false otherwise.
// The method acquires a read lock on the SyncMap before checking for the key,
// and releases the lock before returning.
func (s *SyncMap[T]) Check(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.expires[key]
	if !ok {
		_, ok = s.m[key]
	}
	return ok
}

// DeleteFunc deletes all entries in the SyncMap for which the provided function
// returns true, using the value as the argument.
func (s *SyncMap[T]) DeleteFunc(fn func(T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.m {
		if fn(s.m[key]) {
			s.delete(key)
		}
	}
}

// DeleteFuncExpires deletes all entries in the SyncMap for which the provided function
// fn returns true, using the expiration time of the entry as the argument. It then calls
// the provided function fnVal with the value associated with the key before deleting the entry.
func (s *SyncMap[T]) DeleteFuncExpires(fn func(int64) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.expires {
		if fn(s.expires[key]) {
			s.delete(key)
		}
	}
}

// DeleteFuncExpiresVal deletes all entries in the SyncMap for which the provided function
// fn returns true, using the expiration time of the entry as the argument. It then calls
// the provided function fnVal with the value associated with the key before deleting the entry.
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

// DeleteFuncImdbVal deletes all entries in the SyncMap for which the provided function
// fn returns true, using both the imdb value and the value associated with the key
// as arguments. It then calls the provided function fnVal with the value associated
// with the key before deleting the entry.
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

// DeleteFuncKey deletes all entries in the SyncMap for which the provided function
// returns true, using both the key and value as arguments.
func (s *SyncMap[T]) DeleteFuncKey(fn func(string, T) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.m {
		if fn(key, s.m[key]) {
			s.delete(key)
		}
	}
}
