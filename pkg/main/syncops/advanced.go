package syncops

// processSyncMapCheckExpires checks and manages expiration for sync map entries.
// It validates if an entry exists for the given key, optionally extends the expiration time,
// and handles the slice-valued map types. Returns true if the operation matched a supported type.
func processSyncMapCheckExpires(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]string]) {
			s.CheckExpires(op.Key, op.Extend, op.Duration)
		})

	case MapTypeTwoString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticTwoStringOneInt]) {
			s.CheckExpires(op.Key, op.Extend, op.Duration)
		})

	case MapTypeThreeString:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticThreeStringTwoInt]) {
			s.CheckExpires(op.Key, op.Extend, op.Duration)
		})

	case MapTypeTwoInt:
		return syncMapMetaOp(syncMapInterface, func(s *SyncMap[[]DbstaticOneStringTwoInt]) {
			s.CheckExpires(op.Key, op.Extend, op.Duration)
		})

	default:
		// Other map types do not support CheckExpires
		return false
	}
}

// deleteFunc asserts the map to SyncMap[[]T] and the filter to func([]T) bool,
// then deletes every entry for which the filter returns true.
func deleteFunc[T any](syncMapInterface any, op SyncOperation) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[[]T])
	if !ok {
		return false
	}

	filterFunc, ok := op.FilterFunc.(func([]T) bool)
	if !ok {
		return false
	}

	syncMap.DeleteFunc(filterFunc)

	return true
}

// processSyncMapDeleteFunc removes entries from sync maps based on custom filter functions.
// The filter function is applied to each value in the map, and matching entries are deleted.
func processSyncMapDeleteFunc(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return deleteFunc[string](syncMapInterface, op)
	case MapTypeTwoString:
		return deleteFunc[DbstaticTwoStringOneInt](syncMapInterface, op)
	case MapTypeThreeString:
		return deleteFunc[DbstaticThreeStringTwoInt](syncMapInterface, op)
	case MapTypeTwoInt:
		return deleteFunc[DbstaticOneStringTwoInt](syncMapInterface, op)
	default:
		// Other map types do not support DeleteFunc
		return false
	}
}

// deleteFuncExpires asserts the map to SyncMap[[]T] and the filter to func(int64) bool,
// then deletes every entry whose expiration timestamp satisfies the filter.
func deleteFuncExpires[T any](syncMapInterface any, op SyncOperation) bool {
	syncMap, ok := syncMapInterface.(*SyncMap[[]T])
	if !ok {
		return false
	}

	filterFunc, ok := op.FilterFunc.(func(int64) bool)
	if !ok {
		return false
	}

	syncMap.DeleteFuncExpires(filterFunc)

	return true
}

// processSyncMapDeleteFuncExpires removes entries from sync maps based on expiration time filters.
// The filter function receives the expiration timestamp (int64) and returns true for entries to delete.
func processSyncMapDeleteFuncExpires(syncMapInterface any, op SyncOperation) bool {
	switch op.MapType {
	case MapTypeString:
		return deleteFuncExpires[string](syncMapInterface, op)
	case MapTypeTwoString:
		return deleteFuncExpires[DbstaticTwoStringOneInt](syncMapInterface, op)
	case MapTypeThreeString:
		return deleteFuncExpires[DbstaticThreeStringTwoInt](syncMapInterface, op)
	case MapTypeTwoInt:
		return deleteFuncExpires[DbstaticOneStringTwoInt](syncMapInterface, op)
	default:
		// Other map types do not support DeleteFuncExpires
		return false
	}
}

// processAtomicAppendString atomically appends a string value to a string slice in the sync map.
// Only works with MapTypeString sync maps containing string slice values.
func processAtomicAppendString(syncMapInterface any, op SyncOperation) bool {
	if op.MapType != MapTypeString {
		return false
	}

	syncMap, ok := syncMapInterface.(*SyncMap[[]string])
	if !ok {
		return false
	}

	value, ok := op.Value.(string)
	if !ok {
		return false
	}

	AtomicAppendToStringSlice(syncMap, op.Key, value)

	return true
}

// processAtomicRemoveString atomically removes a string value from a string slice in the sync map.
// Only works with MapTypeString sync maps containing string slice values.
func processAtomicRemoveString(syncMapInterface any, op SyncOperation) bool {
	if op.MapType != MapTypeString {
		return false
	}

	syncMap, ok := syncMapInterface.(*SyncMap[[]string])
	if !ok {
		return false
	}

	value, ok := op.Value.(string)
	if !ok {
		return false
	}

	AtomicRemoveFromStringSlice(syncMap, op.Key, value)

	return true
}
