package logger

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSyncMap_NewSyncMap(t *testing.T) {
	sm := NewSyncMap[string](10)
	if sm == nil {
		t.Error("NewSyncMap returned nil")
	}
	if sm.m == nil || sm.expires == nil || sm.lastScan == nil || sm.imdb == nil {
		t.Error("NewSyncMap internal maps not initialized")
	}
}

func TestSyncMap_AddAndGetVal(t *testing.T) {
	sm := NewSyncMap[string](10)
	key := "testKey"
	value := "testValue"
	expires := time.Now().Add(time.Hour).UnixNano()

	sm.Add(key, value, expires, true, time.Now().UnixNano())

	result := sm.GetVal(key)
	if result != value {
		t.Errorf("GetVal() = %v, want %v", result, value)
	}
}

func TestSyncMap_GetValP(t *testing.T) {
	sm := NewSyncMap[int](10)

	tests := []struct {
		name     string
		key      string
		setValue int
		wantNil  bool
	}{
		{"existing key", "test1", 42, false},
		{"non-existing key", "test2", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantNil {
				sm.Add(tt.key, tt.setValue, 0, false, 0)
			}

			result := sm.GetValP(tt.key)
			if (result == nil) != tt.wantNil {
				t.Errorf("GetValP() nil = %v, want %v", result == nil, tt.wantNil)
			}
			if result != nil && *result != tt.setValue {
				t.Errorf("GetValP() = %v, want %v", *result, tt.setValue)
			}
		})
	}
}

func TestSyncMap_DeleteFuncKey(t *testing.T) {
	sm := NewSyncMap[int](10)
	sm.Add("key1", 1, 0, false, 0)
	sm.Add("key2", 2, 0, false, 0)
	sm.Add("key3", 3, 0, false, 0)

	sm.DeleteFuncKey(func(key string, val int) bool {
		return val > 1
	})

	if !sm.Check("key1") {
		t.Error("key1 should not be deleted")
	}
	if sm.Check("key2") {
		t.Error("key2 should be deleted")
	}
	if sm.Check("key3") {
		t.Error("key3 should be deleted")
	}
}

func TestSyncMap_DeleteFuncExpiresVal(t *testing.T) {
	sm := NewSyncMap[string](10)
	now := time.Now().UnixNano()

	sm.Add("expired", "val1", now-1000, false, 0)
	sm.Add("valid", "val2", now+1000, false, 0)

	var deletedVal string
	sm.DeleteFuncExpiresVal(
		func(expires int64) bool { return expires < now },
		func(val string) { deletedVal = val },
	)

	if sm.Check("expired") {
		t.Error("expired key should be deleted")
	}
	if !sm.Check("valid") {
		t.Error("valid key should not be deleted")
	}
	if deletedVal != "val1" {
		t.Errorf("DeleteFuncExpiresVal callback got %v, want val1", deletedVal)
	}
}

func TestSyncMap_CheckExpires(t *testing.T) {
	sm := NewSyncMap[string](10)
	now := time.Now()

	tests := []struct {
		name    string
		expires time.Time
		extend  bool
		dur     int
		want    bool
	}{
		{"not expired", now.Add(time.Hour), false, 1, false},
		{"expired no extend", now.Add(-time.Hour), false, 1, true},
		{"expired with extend", now.Add(-time.Hour), true, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm.Add("test", "value", tt.expires.UnixNano(), false, 0)
			got := sm.CheckExpires("test", tt.extend, tt.dur)
			if got != tt.want {
				t.Errorf("CheckExpires() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncMap_DeleteFuncImdbVal(t *testing.T) {
	sm := NewSyncMap[string](10)
	sm.Add("imdb1", "val1", 0, true, 0)
	sm.Add("imdb2", "val2", 0, false, 0)

	var deletedVals []string
	sm.DeleteFuncImdbVal(
		func(isImdb bool) bool { return isImdb },
		func(val string) { deletedVals = append(deletedVals, val) },
	)

	if sm.Check("imdb1") {
		t.Error("imdb1 should be deleted")
	}
	if !sm.Check("imdb2") {
		t.Error("imdb2 should not be deleted")
	}
	if len(deletedVals) != 1 || deletedVals[0] != "val1" {
		t.Errorf("DeleteFuncImdbVal callback got %v, want [val1]", deletedVals)
	}
}

func TestSyncMap_Add(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expires  int64
		imdb     bool
		lastscan int64
	}{
		{
			name:     "Add new entry",
			key:      "test1",
			value:    "value1",
			expires:  time.Now().Add(time.Hour).Unix(),
			imdb:     true,
			lastscan: time.Now().Unix(),
		},
		{
			name:     "Add with zero expiry",
			key:      "test2",
			value:    "value2",
			expires:  0,
			imdb:     false,
			lastscan: 0,
		},
		{
			name:     "Add with negative expiry",
			key:      "test3",
			value:    "value3",
			expires:  -1,
			imdb:     true,
			lastscan: -1,
		},
		{
			name:     "Update existing key",
			key:      "test4",
			value:    "value4",
			expires:  time.Now().Add(time.Hour).Unix(),
			imdb:     false,
			lastscan: time.Now().Unix(),
		},
		{
			name:     "Empty key",
			key:      "",
			value:    "value5",
			expires:  time.Now().Add(time.Hour).Unix(),
			imdb:     true,
			lastscan: time.Now().Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SyncMap[string]{
				m:        make(map[string]string),
				expires:  make(map[string]int64),
				lastScan: make(map[string]int64),
				imdb:     make(map[string]bool),
			}

			sm.Add(tt.key, tt.value, tt.expires, tt.imdb, tt.lastscan)

			if val, exists := sm.m[tt.key]; !exists || val != tt.value {
				t.Errorf("value not set correctly, got %v, want %v", val, tt.value)
			}

			if exp, exists := sm.expires[tt.key]; !exists || exp != tt.expires {
				t.Errorf("expires not set correctly, got %v, want %v", exp, tt.expires)
			}

			if ls, exists := sm.lastScan[tt.key]; !exists || ls != tt.lastscan {
				t.Errorf("lastScan not set correctly, got %v, want %v", ls, tt.lastscan)
			}

			if imdb, exists := sm.imdb[tt.key]; !exists || imdb != tt.imdb {
				t.Errorf("imdb not set correctly, got %v, want %v", imdb, tt.imdb)
			}
		})
	}
}

func TestSyncMap_UpdateVal(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]int
		key      string
		value    int
		expected map[string]int
	}{
		{
			name:     "Update existing key",
			initial:  map[string]int{"test": 1},
			key:      "test",
			value:    2,
			expected: map[string]int{"test": 2},
		},
		{
			name:     "Add new key",
			initial:  map[string]int{},
			key:      "new",
			value:    1,
			expected: map[string]int{"new": 1},
		},
		{
			name:     "Update with zero value",
			initial:  map[string]int{"test": 1},
			key:      "test",
			value:    0,
			expected: map[string]int{"test": 0},
		},
		{
			name:     "Update with multiple existing entries",
			initial:  map[string]int{"a": 1, "b": 2, "c": 3},
			key:      "b",
			value:    5,
			expected: map[string]int{"a": 1, "b": 5, "c": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SyncMap[int]{
				mu: sync.Mutex{},
				m:  tt.initial,
			}

			sm.UpdateVal(tt.key, tt.value)

			if len(sm.m) != len(tt.expected) {
				t.Errorf("map length = %d, want %d", len(sm.m), len(tt.expected))
			}

			for k, v := range tt.expected {
				if got, ok := sm.m[k]; !ok || got != v {
					t.Errorf("map[%s] = %d, want %d", k, got, v)
				}
			}
		})
	}
}

func TestSyncMap_UpdateVal_Concurrent(t *testing.T) {
	sm := &SyncMap[int]{
		mu: sync.Mutex{},
		m:  make(map[string]int),
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(val int) {
			defer wg.Done()
			sm.UpdateVal("test", val)
		}(i)
	}

	wg.Wait()

	if _, ok := sm.m["test"]; !ok {
		t.Error("key 'test' not found in map")
	}
}

func TestSyncMap_UpdateExpire(t *testing.T) {
	tests := []struct {
		name      string
		initial   int64
		update    int64
		expectSet bool
	}{
		{
			name:      "Update non-zero expiry",
			initial:   time.Now().Add(time.Hour).UnixNano(),
			update:    time.Now().Add(2 * time.Hour).UnixNano(),
			expectSet: true,
		},
		{
			name:      "Update with zero expiry",
			initial:   time.Now().Add(time.Hour).UnixNano(),
			update:    0,
			expectSet: true,
		},
		{
			name:      "Update negative expiry",
			initial:   time.Now().Add(time.Hour).UnixNano(),
			update:    -1,
			expectSet: true,
		},
		{
			name:      "Update when initial is zero",
			initial:   0,
			update:    time.Now().Add(time.Hour).UnixNano(),
			expectSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSyncMap[string](10)
			key := "testKey"
			value := "testValue"

			sm.Add(key, value, tt.initial, false, 0)
			initialExpiry := sm.expires[key]

			sm.UpdateExpire(key, tt.update)

			if tt.expectSet {
				if sm.expires[key] != tt.update {
					t.Errorf(
						"UpdateExpire() did not set correct expiry value, got %v, want %v",
						sm.expires[key],
						tt.update,
					)
				}
				if sm.expires[key] == initialExpiry {
					t.Error("UpdateExpire() did not change expiry value when it should have")
				}
			} else {
				if sm.expires[key] != initialExpiry {
					t.Error("UpdateExpire() changed expiry value when it should not have")
				}
			}
		})
	}
}

func TestSyncMap_UpdateExpire_NonExistentKey(t *testing.T) {
	sm := NewSyncMap[string](10)
	newExpiry := time.Now().Add(time.Hour).UnixNano()

	sm.UpdateExpire("nonexistent", newExpiry)

	if _, exists := sm.expires["nonexistent"]; exists {
		t.Error("UpdateExpire() created an entry for non-existent key")
	}
}

func TestSyncMap_UpdateExpire_Concurrent(t *testing.T) {
	sm := NewSyncMap[string](10)
	key := "testKey"
	value := "testValue"
	initialExpiry := time.Now().Add(time.Hour).UnixNano()

	sm.Add(key, value, initialExpiry, false, 0)

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(val int64) {
			defer wg.Done()
			sm.UpdateExpire(key, val)
		}(time.Now().Add(time.Duration(i) * time.Second).UnixNano())
	}

	wg.Wait()

	if sm.expires[key] == initialExpiry {
		t.Error("UpdateExpire() failed to update value in concurrent scenario")
	}
}

func TestSyncMap_UpdateLastscan(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    int64
		initial  map[string]int64
		expected map[string]int64
	}{
		{
			name:     "Update existing key",
			key:      "test1",
			value:    123456789,
			initial:  map[string]int64{"test1": 100},
			expected: map[string]int64{"test1": 123456789},
		},
		{
			name:     "Add new key",
			key:      "test2",
			value:    987654321,
			initial:  map[string]int64{},
			expected: map[string]int64{"test2": 987654321},
		},
		{
			name:     "Update with zero value",
			key:      "test3",
			value:    0,
			initial:  map[string]int64{"test3": 999},
			expected: map[string]int64{"test3": 0},
		},
		{
			name:     "Update with negative value",
			key:      "test4",
			value:    -123456789,
			initial:  map[string]int64{"test4": 100},
			expected: map[string]int64{"test4": -123456789},
		},
		{
			name:     "Empty key",
			key:      "",
			value:    123456789,
			initial:  map[string]int64{},
			expected: map[string]int64{"": 123456789},
		},
		{
			name:     "Multiple existing keys",
			key:      "test5",
			value:    555,
			initial:  map[string]int64{"test5": 100, "other": 200},
			expected: map[string]int64{"test5": 555, "other": 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSyncMap[string](10)
			sm.lastScan = tt.initial

			sm.UpdateLastscan(tt.key, tt.value)

			if len(sm.lastScan) != len(tt.expected) {
				t.Errorf("lastScan map length = %d, want %d", len(sm.lastScan), len(tt.expected))
			}

			for k, v := range tt.expected {
				if got, ok := sm.lastScan[k]; !ok || got != v {
					t.Errorf("lastScan[%s] = %d, want %d", k, got, v)
				}
			}
		})
	}
}

func TestSyncMap_UpdateLastscan_Concurrent(t *testing.T) {
	sm := NewSyncMap[string](10)
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(val int64) {
			defer wg.Done()
			sm.UpdateLastscan("concurrent", val)
		}(int64(i))
	}

	wg.Wait()

	if _, ok := sm.lastScan["concurrent"]; !ok {
		t.Error("key 'concurrent' not found in lastScan map")
	}
}

func TestSyncMap_Check(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*SyncMap[string])
		key      string
		expected bool
	}{
		{
			name: "key exists in expires map only",
			setup: func(sm *SyncMap[string]) {
				sm.expires["test"] = time.Now().UnixNano()
			},
			key:      "test",
			expected: true,
		},
		{
			name: "key exists in main map only",
			setup: func(sm *SyncMap[string]) {
				sm.m["test"] = "value"
			},
			key:      "test",
			expected: true,
		},
		{
			name: "key exists in both maps",
			setup: func(sm *SyncMap[string]) {
				sm.m["test"] = "value"
				sm.expires["test"] = time.Now().UnixNano()
			},
			key:      "test",
			expected: true,
		},
		{
			name:     "key does not exist in either map",
			setup:    func(sm *SyncMap[string]) {},
			key:      "nonexistent",
			expected: false,
		},
		{
			name: "empty key",
			setup: func(sm *SyncMap[string]) {
				sm.m[""] = "value"
			},
			key:      "",
			expected: true,
		},
		{
			name: "special characters in key",
			setup: func(sm *SyncMap[string]) {
				sm.expires["!@#$%^&*"] = time.Now().UnixNano()
			},
			key:      "!@#$%^&*",
			expected: true,
		},
		{
			name: "unicode characters in key",
			setup: func(sm *SyncMap[string]) {
				sm.m["世界"] = "value"
			},
			key:      "世界",
			expected: true,
		},
		{
			name: "very long key",
			setup: func(sm *SyncMap[string]) {
				longKey := strings.Repeat("a", 1000)
				sm.expires[longKey] = time.Now().UnixNano()
			},
			key:      strings.Repeat("a", 1000),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SyncMap[string]{
				m:       make(map[string]string),
				expires: make(map[string]int64),
			}
			tt.setup(sm)

			result := sm.Check(tt.key)
			if result != tt.expected {
				t.Errorf("Check(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}
