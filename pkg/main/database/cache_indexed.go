package database

import (
	"strings"
	"sync"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// normKeyPool provides reusable byte buffers for normalizeKey to reduce allocations.
var normKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256)
		return &b
	},
}

// normalizeKey converts a string to lowercase and trims whitespace for case-insensitive lookups
// Uses sync.Pool to reuse buffers and reduce allocations.
func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Get buffer from pool
	bufPtr := normKeyPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	// Convert to lowercase manually to avoid strings.ToLower allocation
	for _, r := range s {
		if r <= unicode.MaxASCII {
			if r >= 'A' && r <= 'Z' {
				buf = append(buf, byte(r+32))
			} else {
				buf = append(buf, byte(r))
			}
		} else {
			// For non-ASCII characters, use unicode.ToLower
			for _, b := range string(unicode.ToLower(r)) {
				buf = append(buf, byte(b))
			}
		}
	}

	result := string(buf)

	// Return buffer to pool
	*bufPtr = buf
	normKeyPool.Put(bufPtr)

	return result
}

// compositeKeyPool provides reusable byte buffers for makeCompositeKey.
var compositeKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 64) // Most composite keys are under 64 bytes
		return &b
	},
}

// makeCompositeKey creates a composite key from mediaID and listname for O(1) lookups
// Uses sync.Pool to avoid fmt.Sprintf allocations.
func makeCompositeKey(num1 uint, str string) string {
	normalizedStr := normalizeKey(str)

	// Get buffer from pool
	bufPtr := compositeKeyPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	// Append number as string
	buf = appendUint(buf, num1)

	// Append separator
	buf = append(buf, ':')

	// Append normalized string
	buf = append(buf, normalizedStr...)

	result := string(buf)

	// Return buffer to pool
	*bufPtr = buf
	compositeKeyPool.Put(bufPtr)

	return result
}

// appendUint appends a uint to a byte slice without allocating.
func appendUint(buf []byte, n uint) []byte {
	if n == 0 {
		return append(buf, '0')
	}

	// Count digits
	temp := n

	digits := 0
	for temp > 0 {
		digits++

		temp /= 10
	}

	// Grow buffer
	start := len(buf)

	buf = append(buf, make([]byte, digits)...)

	// Write digits from right to left
	for i := digits - 1; i >= 0; i-- {
		buf[start+i] = byte('0' + n%10)

		n /= 10
	}

	return buf
}

// ========== Index Building Functions ==========

// buildIndexThreeStringByStr3 creates an index mapping normalized Str3 (e.g., IMDB ID) to struct pointer.
func buildIndexThreeStringByStr3(
	arr []syncops.DbstaticThreeStringTwoInt,
) map[string]*syncops.DbstaticThreeStringTwoInt {
	index := make(map[string]*syncops.DbstaticThreeStringTwoInt, len(arr))

	for i := range arr {
		if arr[i].Str3 != "" {
			key := normalizeKey(arr[i].Str3)

			index[key] = &arr[i]
		}
	}

	return index
}

// buildIndexThreeStringByNum2 creates an index mapping Num2 (database ID) to struct pointer.
func buildIndexThreeStringByNum2(
	arr []syncops.DbstaticThreeStringTwoInt,
) map[uint]*syncops.DbstaticThreeStringTwoInt {
	index := make(map[uint]*syncops.DbstaticThreeStringTwoInt, len(arr))

	for i := range arr {
		index[arr[i].Num2] = &arr[i]
	}

	return index
}

// buildIndexTwoStringByStr1 creates a one-to-many index for title lookups.
func buildIndexTwoStringByStr1(
	arr []syncops.DbstaticTwoStringOneInt,
) map[string][]*syncops.DbstaticTwoStringOneInt {
	index := make(map[string][]*syncops.DbstaticTwoStringOneInt, len(arr)*2)

	for i := range arr {
		// Index by both Str1 and Str2 for title and slug lookups
		if arr[i].Str1 != "" {
			key := normalizeKey(arr[i].Str1)

			index[key] = append(index[key], &arr[i])
		}

		if arr[i].Str2 != "" {
			key := normalizeKey(arr[i].Str2)

			index[key] = append(index[key], &arr[i])
		}
	}

	return index
}

// buildIndexStringSet creates a set-based index for fast case-insensitive contains checks.
func buildIndexStringSet(arr []string) map[string]struct{} {
	index := make(map[string]struct{}, len(arr))

	for _, s := range arr {
		if s != "" {
			index[normalizeKey(s)] = struct{}{}
		}
	}

	return index
}

// ========== Fast Lookup Functions with Fallback ==========

// CacheThreeStringIntIndexFuncFast - O(1) lookup by Str3 (e.g., IMDB ID)
// Returns Num2 (database ID) for the given Str3 value.
// Falls back to linear scan if index not available.
func CacheThreeStringIntIndexFuncFast(s string, u *string) uint {
	if !config.GetSettingsGeneral().UseIndexedCache || u == nil || *u == "" {
		// Fallback to old O(n) implementation
		return CacheThreeStringIntIndexFunc(s, u)
	}

	// Try indexed lookup first
	indexMap := cache.indexThreeStringByStr3.GetVal(s)
	if indexMap != nil {
		key := normalizeKey(*u)
		if entry, exists := indexMap[key]; exists {
			return entry.Num2
		}
	}

	// Fallback if index not built or not found
	return CacheThreeStringIntIndexFunc(s, u)
}

// CacheThreeStringIntIndexFuncGetYearFast - O(1) lookup by Num2 (database ID)
// Returns Num1 (year) for the given database ID.
func CacheThreeStringIntIndexFuncGetYearFast(s string, id uint) uint16 {
	if !config.GetSettingsGeneral().UseIndexedCache {
		return CacheThreeStringIntIndexFuncGetYear(s, id)
	}

	indexMap := cache.indexThreeStringByNum2.GetVal(s)
	if indexMap != nil {
		if entry, exists := indexMap[id]; exists {
			return uint16(entry.Num1)
		}
	}

	return CacheThreeStringIntIndexFuncGetYear(s, id)
}

// CacheOneStringTwoIntIndexFuncRetFast - O(1) lookup for (mediaID, listname) composite key
// Returns Num2 (list ID) for the given media ID and list name combination.
func CacheOneStringTwoIntIndexFuncRetFast(s string, id uint, listname string) uint {
	if !config.GetSettingsGeneral().UseIndexedCache {
		return CacheOneStringTwoIntIndexFuncRet(s, id, listname)
	}

	indexMap := cache.indexTwoIntComposite.GetVal(s)
	if indexMap != nil {
		key := makeCompositeKey(id, listname)
		if entry, exists := indexMap[key]; exists {
			return entry.Num2
		}
	}

	return CacheOneStringTwoIntIndexFuncRet(s, id, listname)
}

// FindSeriesIDByTitleFast - O(1) lookup for series ID by title
// Returns database ID for the given title (case-insensitive).
// Searches both main series cache and alternate titles cache.
func FindSeriesIDByTitleFast(title string) uint {
	if !config.GetSettingsGeneral().UseIndexedCache || title == "" {
		return 0
	}

	normalizedTitle := normalizeKey(title)

	// Try alternate titles cache first (most common)
	altIndexMap := cache.indexTwoStringByStr1.GetVal(logger.CacheDBSeriesAlt)
	if altIndexMap != nil {
		if entries, exists := altIndexMap[normalizedTitle]; exists && len(entries) > 0 {
			return entries[0].Num
		}
	}

	// Fallback: For main series cache, we'd need to build a title->ID index
	// For now, fall back to linear search if not found in alternates
	return 0
}

// SlicesCacheContainsIFast - O(1) case-insensitive contains check for string arrays
// Returns true if the string exists in the cache (case-insensitive).
func SlicesCacheContainsIFast(useseries bool, query string, w *string) bool {
	if !config.GetSettingsGeneral().UseIndexedCache || w == nil || *w == "" {
		return SlicesCacheContainsI(useseries, query, w)
	}

	indexMap := cache.indexStringSet.GetVal(logger.GetStringsMap(useseries, query))
	if indexMap != nil {
		_, exists := indexMap[normalizeKey(*w)]
		return exists
	}

	return SlicesCacheContainsI(useseries, query, w)
}

// ========== Index Refresh Functions ==========

// RefreshMediaCacheDBIndexed refreshes the media cache and builds indexes.
func RefreshMediaCacheDBIndexed(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	// Call standard refresh first
	RefreshMediaCacheDB(useseries, force)

	// Build indexes if enabled
	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	mapvar := logger.GetStringsMap(useseries, logger.CacheDBMedia)

	// Get the refreshed array
	arr := GetCachedThreeStringArr(mapvar, false, false)
	if len(arr) == 0 {
		return
	}

	// logger.Logtype("debug", 1).
	// 	Str("cache", mapvar).
	// 	Int("count", len(arr)).
	// 	Msg("building cache indexes")

	// Build and store indexes
	indexStr3 := buildIndexThreeStringByStr3(arr)
	indexNum2 := buildIndexThreeStringByNum2(arr)

	// Store indexes using SyncMap's Add method (no expiration for indexes)
	cache.indexThreeStringByStr3.Add(mapvar, indexStr3, 0, false, 0)
	cache.indexThreeStringByNum2.Add(mapvar, indexNum2, 0, false, 0)

	// logger.Logtype("debug", 1).
	// 	Str("cache", mapvar).
	// 	Int("str3_keys", len(indexStr3)).
	// 	Int("num2_keys", len(indexNum2)).
	// 	Msg("cache indexes built")
}

// RefreshMediaCacheTitlesIndexed refreshes title cache and builds indexes.
func RefreshMediaCacheTitlesIndexed(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	RefreshMediaCacheTitles(useseries, force)

	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	mapvar := logger.GetStringsMap(useseries, logger.CacheMediaTitles)
	arr := GetCachedTwoStringArr(mapvar, false, false)

	if len(arr) > 0 {
		// logger.Logtype("debug", 1).
		// 	Str("cache", mapvar).
		// 	Int("count", len(arr)).
		// 	Msg("building title cache indexes")
		indexByStr1 := buildIndexTwoStringByStr1(arr)
		cache.indexTwoStringByStr1.Add(mapvar, indexByStr1, 0, false, 0)

		// logger.Logtype("debug", 1).
		// 	Str("cache", mapvar).
		// 	Int("keys", len(indexByStr1)).
		// 	Msg("title cache indexes built")
	}
}

// RefreshhistorycacheurlIndexed refreshes history URL cache and builds set index.
func RefreshhistorycacheurlIndexed(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	Refreshhistorycacheurl(useseries, force)

	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	mapvar := logger.GetStringsMap(useseries, logger.CacheHistoryURL)
	arr := GetCachedStringArr(mapvar, false, false)

	if len(arr) > 0 {
		// logger.Logtype("debug", 2).
		// 	Str("cache", mapvar).
		// 	Int("count", len(arr)).
		// 	Msg("building history URL cache index")
		indexSet := buildIndexStringSet(arr)
		cache.indexStringSet.Add(mapvar, indexSet, 0, false, 0)
	}
}

// RefreshhistorycachetitleIndexed refreshes history title cache and builds set index.
func RefreshhistorycachetitleIndexed(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	Refreshhistorycachetitle(useseries, force)

	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	mapvar := logger.GetStringsMap(useseries, logger.CacheHistoryTitle)
	arr := GetCachedStringArr(mapvar, false, false)

	if len(arr) > 0 {
		// logger.Logtype("debug", 2).
		// 	Str("cache", mapvar).
		// 	Int("count", len(arr)).
		// 	Msg("building history title cache index")
		indexSet := buildIndexStringSet(arr)
		cache.indexStringSet.Add(mapvar, indexSet, 0, false, 0)
	}
}
