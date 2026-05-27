package database

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

var (
	normKeyPool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 256)
			return &b
		},
	}
	compositeKeyPool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 64) // Most composite keys are under 64 bytes
			return &b
		},
	}
)

// normalizeKey converts a string to lowercase and trims whitespace for
// case-insensitive lookups. Uses a fast path for already-lowercase ASCII
// strings (e.g. slugs) that returns the input directly with zero allocations.
func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Fast path: scan for any uppercase ASCII or non-ASCII byte.
	// Slugs and already-normalised strings skip the pool entirely.
	needsConversion := false
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 0x80 {
			needsConversion = true
			break
		}
	}

	if !needsConversion {
		return s
	}

	// Slow path: byte-level loop avoids per-character rune decoding for ASCII.
	bufPtr := normKeyPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 {
			if c >= 'A' && c <= 'Z' {
				buf = append(buf, c+32)
			} else {
				buf = append(buf, c)
			}
		} else {
			// Non-ASCII: decode rune, lowercase, re-encode without allocating
			// an intermediate string.
			r, size := utf8.DecodeRuneInString(s[i:])

			var tmp [utf8.UTFMax]byte

			n := utf8.EncodeRune(tmp[:], unicode.ToLower(r))

			buf = append(buf, tmp[:n]...)

			i += size - 1
		}
	}

	result := string(buf)

	*bufPtr = buf
	normKeyPool.Put(bufPtr)

	return result
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

// ========== Index Append Functions ==========

// appendIndexThreeString adds a single entry to the ThreeString index maps so that
// indexed lookups remain accurate immediately after an insert, without a full rebuild.
// A new heap-allocated pointer is used so the index entry is independent of the slice.
func appendIndexThreeString(mapvar string, entry syncops.DbstaticThreeStringTwoInt) {
	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	entryPtr := &syncops.DbstaticThreeStringTwoInt{
		Str1: entry.Str1,
		Str2: entry.Str2,
		Str3: entry.Str3,
		Num1: entry.Num1,
		Num2: entry.Num2,
	}
	if entry.Str3 != "" {
		key := normalizeKey(entry.Str3)
		cache.indexThreeStringByStr3.ModifyInPlace(
			mapvar,
			func(m map[string]*syncops.DbstaticThreeStringTwoInt) {
				m[key] = entryPtr
			},
		)
	}

	cache.indexThreeStringByNum2.ModifyInPlace(
		mapvar,
		func(m map[uint]*syncops.DbstaticThreeStringTwoInt) {
			m[entry.Num2] = entryPtr
		},
	)
}

// appendIndexTwoString adds a single entry to the TwoString index map (title -> entries)
// so that title-based indexed lookups remain accurate immediately after an insert.
// Both Str1 and Str2 are indexed, mirroring buildIndexTwoStringByStr1.
func appendIndexTwoString(mapvar string, entry syncops.DbstaticTwoStringOneInt) {
	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	entryPtr := &syncops.DbstaticTwoStringOneInt{
		Str1: entry.Str1,
		Str2: entry.Str2,
		Num:  entry.Num,
	}

	var key1, key2 string
	if entry.Str1 != "" {
		key1 = normalizeKey(entry.Str1)
	}

	if entry.Str2 != "" {
		key2 = normalizeKey(entry.Str2)
	}

	cache.indexTwoStringByStr1.ModifyInPlace(
		mapvar,
		func(m map[string][]*syncops.DbstaticTwoStringOneInt) {
			if key1 != "" {
				m[key1] = append(m[key1], entryPtr)
			}

			if key2 != "" {
				m[key2] = append(m[key2], entryPtr)
			}
		},
	)
}

// appendIndexTwoInt adds a single entry to the TwoInt composite index map so that
// indexed lookups remain accurate immediately after an insert.
func appendIndexTwoInt(mapvar string, entry syncops.DbstaticOneStringTwoInt) {
	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	entryPtr := &syncops.DbstaticOneStringTwoInt{
		Str:  entry.Str,
		Num1: entry.Num1,
		Num2: entry.Num2,
	}
	key := makeCompositeKey(entry.Num1, entry.Str)
	cache.indexTwoIntComposite.ModifyInPlace(
		mapvar,
		func(m map[string]*syncops.DbstaticOneStringTwoInt) {
			m[key] = entryPtr
		},
	)
}

// ========== Index Building Functions ==========

// buildIndexThreeStringByStr3 creates an index mapping normalized Str3 (e.g., IMDB ID) to struct pointer.
func buildIndexThreeStringByStr3(
	arr []syncops.DbstaticThreeStringTwoInt,
) map[string]*syncops.DbstaticThreeStringTwoInt {
	index := make(map[string]*syncops.DbstaticThreeStringTwoInt, len(arr))

	for i := range arr {
		if arr[i].Str3 != "" {
			index[normalizeKey(arr[i].Str3)] = &arr[i]
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
	counts := make(map[string]int, len(arr)*2)
	for i := range arr {
		if arr[i].Str1 != "" {
			counts[normalizeKey(arr[i].Str1)]++
		}

		if arr[i].Str2 != "" {
			counts[normalizeKey(arr[i].Str2)]++
		}
	}

	total := 0
	for _, n := range counts {
		total += n
	}

	slab := make([]*syncops.DbstaticTwoStringOneInt, total)
	index := make(map[string][]*syncops.DbstaticTwoStringOneInt, len(counts))

	pos := 0
	for k, n := range counts {
		index[k] = slab[pos : pos : pos+n]

		pos += n
	}

	for i := range arr {
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

	for i := range arr {
		if arr[i] != "" {
			index[normalizeKey(arr[i])] = struct{}{}
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
		if entry, exists := indexMap[normalizeKey(*u)]; exists {
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
			return uint16(entry.Num1) //nolint:gosec // safe: value within target type range
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

	// Try alternate titles cache first (most common)
	altIndexMap := cache.indexTwoStringByStr1.GetVal(logger.CacheDBSeriesAlt)
	if altIndexMap != nil {
		if entries, exists := altIndexMap[normalizeKey(title)]; exists && len(entries) > 0 {
			return entries[0].Num
		}
	}

	// Fallback: For main series cache, we'd need to build a title->ID index
	// For now, fall back to linear search if not found in alternates
	return 0
}

// SlicesCacheContainsIFast - O(1) case-insensitive contains check for string arrays
// Returns true if the string exists in the cache (case-insensitive).
func SlicesCacheContainsIFast(isType uint, query string, w *string) bool {
	if !config.GetSettingsGeneral().UseIndexedCache || w == nil || *w == "" {
		return SlicesCacheContainsI(isType, query, w)
	}

	indexMap := cache.indexStringSet.GetVal(mtstrings.GetStringsMap(isType, query))
	if indexMap != nil {
		_, exists := indexMap[normalizeKey(*w)]
		return exists
	}

	return SlicesCacheContainsI(isType, query, w)
}

// ========== Index Helpers (called from RefreshCached via syncops.QueueFunc) ==========

// getLastScanForKind returns the last-scan timestamp from the SyncMap that corresponds
// to the given index kind. Used to detect whether a data refresh actually occurred.
func getLastScanForKind(kind uint8, mapvar string) int64 {
	switch kind {
	case indexThreeString:
		return cache.itemsthreestring.GetLastScan(mapvar)
	case indexTwoString:
		return cache.itemstwostring.GetLastScan(mapvar)
	case indexStringSet:
		return cache.itemsstring.GetLastScan(mapvar)
	}

	return 0
}

// buildAndStoreIndex builds and stores the index for the given kind.
// Must be called from the writer goroutine (via syncops.QueueFunc) to avoid
// races with concurrent slice appends.
func buildAndStoreIndex(kind uint8, mapvar string) {
	switch kind {
	case indexThreeString:
		arr := getCachedArrayDirect(cache.itemsthreestring, mapvar, false, false)
		if len(arr) == 0 {
			return
		}

		cache.indexThreeStringByStr3.Add(mapvar, buildIndexThreeStringByStr3(arr), 0, false, 0)
		cache.indexThreeStringByNum2.Add(mapvar, buildIndexThreeStringByNum2(arr), 0, false, 0)

	case indexTwoString:
		arr := getCachedArrayDirect(cache.itemstwostring, mapvar, false, false)
		if len(arr) > 0 {
			cache.indexTwoStringByStr1.Add(mapvar, buildIndexTwoStringByStr1(arr), 0, false, 0)
		}

	case indexStringSet:
		arr := getCachedArrayDirect(cache.itemsstring, mapvar, false, false)
		if len(arr) > 0 {
			cache.indexStringSet.Add(mapvar, buildIndexStringSet(arr), 0, false, 0)
		}
	}
}
