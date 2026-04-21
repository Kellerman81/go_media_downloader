package syncops

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

	default:
		// Other map types do not support CheckExpires
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
		syncMap, ok := syncMapInterface.(*SyncMap[[]string])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func([]string) bool)
		if !ok {
			break
		}

		syncMap.DeleteFunc(filterFunc)

		return true

	case MapTypeTwoString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func([]DbstaticTwoStringOneInt) bool)
		if !ok {
			break
		}

		syncMap.DeleteFunc(filterFunc)

		return true

	case MapTypeThreeString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func([]DbstaticThreeStringTwoInt) bool)
		if !ok {
			break
		}

		syncMap.DeleteFunc(filterFunc)

		return true

	case MapTypeTwoInt:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func([]DbstaticOneStringTwoInt) bool)
		if !ok {
			break
		}

		syncMap.DeleteFunc(filterFunc)

		return true

	default:
		// Other map types do not support DeleteFunc
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
		syncMap, ok := syncMapInterface.(*SyncMap[[]string])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		syncMap.DeleteFuncExpires(filterFunc)

		return true

	case MapTypeTwoString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		syncMap.DeleteFuncExpires(filterFunc)

		return true

	case MapTypeThreeString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		syncMap.DeleteFuncExpires(filterFunc)

		return true

	case MapTypeTwoInt:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		syncMap.DeleteFuncExpires(filterFunc)

		return true

	default:
		// Other map types do not support DeleteFuncExpires
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
		syncMap, ok := syncMapInterface.(*SyncMap[[]string])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]string))
		if !ok {
			break
		}

		syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)

		return true

	case MapTypeTwoString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticTwoStringOneInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)

		return true

	case MapTypeThreeString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticThreeStringTwoInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)

		return true

	case MapTypeTwoInt:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(int64) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticOneStringTwoInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncExpiresVal(filterFunc, valueFunc)

		return true
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
		syncMap, ok := syncMapInterface.(*SyncMap[[]string])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(bool) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]string))
		if !ok {
			break
		}

		syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)

		return true

	case MapTypeTwoString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticTwoStringOneInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(bool) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticTwoStringOneInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)

		return true

	case MapTypeThreeString:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticThreeStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(bool) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticThreeStringTwoInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)

		return true

	case MapTypeTwoInt:
		syncMap, ok := syncMapInterface.(*SyncMap[[]DbstaticOneStringTwoInt])
		if !ok {
			break
		}

		filterFunc, ok := op.FilterFunc.(func(bool) bool)
		if !ok {
			break
		}

		valueFunc, ok := op.ValueFunc.(func([]DbstaticOneStringTwoInt))
		if !ok {
			break
		}

		syncMap.DeleteFuncImdbVal(filterFunc, valueFunc)

		return true
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
