package syncops

import (
	"regexp"

	"github.com/jmoiron/sqlx"
)

// processSyncMapCheckExpires checks and manages expiration for sync map entries.
// It validates if an entry exists for the given key, optionally extends the expiration time,
// and handles different map types (String, TwoString, ThreeString, TwoInt).
// Returns true if the operation was successfully processed, false otherwise.
func (m *SyncOpsManager) processSyncMapCheckExpires(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			syncMap.CheckExpires(op.Key, op.Extend, op.Duration)
			return true
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			syncMap.CheckExpires(op.Key, op.Extend, op.Duration)
			return true
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			syncMap.CheckExpires(op.Key, op.Extend, op.Duration)
			return true
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			syncMap.CheckExpires(op.Key, op.Extend, op.Duration)
			return true
		}
	}

	return false
}

// processSyncMapDeleteFunc removes entries from sync maps based on custom filter functions.
// The filter function is applied to each value in the map, and matching entries are deleted.
// Supports String, TwoString, ThreeString, and TwoInt map types with type-safe filter functions.
// Returns true if the operation was successfully processed, false otherwise.
func (m *SyncOpsManager) processSyncMapDeleteFunc(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if filterFunc, ok := op.FilterFunc.(func([]string) bool); ok {
				syncMap.DeleteFunc(filterFunc)
				return true
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func([]DbstaticTwoStringOneInt) bool); ok {
				syncMap.DeleteFunc(filterFunc)
				return true
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func([]DbstaticThreeStringTwoInt) bool); ok {
				syncMap.DeleteFunc(filterFunc)
				return true
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func([]DbstaticOneStringTwoInt) bool); ok {
				syncMap.DeleteFunc(filterFunc)
				return true
			}
		}
	}

	return false
}

// processSyncMapDeleteFuncExpires removes entries from sync maps based on expiration time filters.
// The filter function receives the expiration timestamp (int64) and returns true for entries to delete.
// Supports all map types including XStmt and Regex in addition to the standard data types.
// Returns true if the operation was successfully processed, false otherwise.
func (m *SyncOpsManager) processSyncMapDeleteFuncExpires(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}

	case MapTypeRegex:
		if syncMap, ok := syncMapInterface.(*SyncMap[*regexp.Regexp]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				syncMap.DeleteFuncExpires(filterFunc)
				return true
			}
		}
	}

	return false
}

// processSyncMapDeleteFuncExpiresVal removes expired entries and executes a callback on their values.
// Combines expiration-based filtering with value processing before deletion.
// The filter function checks expiration times, and the value function processes each deleted entry.
// Supports String, TwoString, ThreeString, TwoInt, and XStmt map types.
// Returns true if the operation was successfully processed, false otherwise.
func (m *SyncOpsManager) processSyncMapDeleteFuncExpiresVal(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]string)); ok {
					syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticTwoStringOneInt)); ok {
					syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticThreeStringTwoInt)); ok {
					syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticOneStringTwoInt)); ok {
					syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(int64) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func(*sqlx.Stmt)); ok {
					syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)
					return true
				}
			}
		}
	}

	return false
}

// processSyncMapDeleteFuncImdbVal removes entries based on IMDB-related boolean filters and processes values.
// Designed specifically for IMDB data management with boolean condition filtering.
// The filter function receives a boolean flag and the value function processes deleted entries.
// Supports String, TwoString, ThreeString, TwoInt, and XStmt map types.
// Returns true if the operation was successfully processed, false otherwise.
func (m *SyncOpsManager) processSyncMapDeleteFuncImdbVal(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	switch op.MapType {
	case MapTypeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if filterFunc, ok := op.FilterFunc.(func(bool) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]string)); ok {
					syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeTwoString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(bool) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticTwoStringOneInt)); ok {
					syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeThreeString:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(bool) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticThreeStringTwoInt)); ok {
					syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeTwoInt:
		if syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(bool) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func([]DbstaticOneStringTwoInt)); ok {
					syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)
					return true
				}
			}
		}

	case MapTypeXStmt:
		if syncMap, ok := syncMapInterface.(*SyncMap[*sqlx.Stmt]); ok {
			if filterFunc, ok := op.FilterFunc.(func(bool) bool); ok {
				if valueFunc, ok := op.ValueFunc.(func(*sqlx.Stmt)); ok {
					syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)
					return true
				}
			}
		}
	}

	return false
}

// processAtomicAppendString atomically appends a string value to a string slice in the sync map.
// This operation is thread-safe and ensures consistent state during concurrent access.
// Only works with MapTypeString sync maps containing string slice values.
// Returns true if the string was successfully appended, false otherwise.
func (m *SyncOpsManager) processAtomicAppendString(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	if op.MapType == MapTypeString {
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if value, ok := op.Value.(string); ok {
				// Call the AtomicAppendToStringSlice function
				AtomicAppendToStringSlice(syncMap, op.Key, value)
				return true
			}
		}
	}

	return false
}

// processAtomicRemoveString atomically removes a string value from a string slice in the sync map.
// This operation is thread-safe and ensures consistent state during concurrent access.
// Only works with MapTypeString sync maps containing string slice values.
// Returns true if the string was successfully removed, false otherwise.
func (m *SyncOpsManager) processAtomicRemoveString(op SyncOperation) bool {
	syncMapInterface, exists := m.syncMaps[op.MapType]
	if !exists {
		return false
	}

	if op.MapType == MapTypeString {
		if syncMap, ok := syncMapInterface.(*SyncMap[[]string]); ok {
			if value, ok := op.Value.(string); ok {
				// Call the AtomicRemoveFromStringSlice function
				AtomicRemoveFromStringSlice(syncMap, op.Key, value)
				return true
			}
		}
	}

	return false
}
