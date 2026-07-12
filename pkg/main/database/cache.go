package database

import (
	"context"
	"iter"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/jmoiron/sqlx"
)

// globalcache is a struct that stores configuration values for the cache
// including the default cache expiration time and a logger instance.
type globalcache struct {
	// defaultextension is the default cache expiration duration
	defaultextension time.Duration
	// mu protects concurrent access to defaultextension
	mu sync.RWMutex
}

type tcache struct {
	interval         time.Duration
	itemsstring      *syncops.SyncMap[[]string]
	itemstwoint      *syncops.SyncMap[[]syncops.DbstaticOneStringTwoInt]
	itemsthreestring *syncops.SyncMap[[]syncops.DbstaticThreeStringTwoInt]
	itemstwostring   *syncops.SyncMap[[]syncops.DbstaticTwoStringOneInt]

	// NEW: Ristretto caches for high-performance lookups
	ristrettoStmt  *ristretto.Cache[string, *sqlx.Stmt]
	ristrettoRegex *ristretto.Cache[string, *regexp.Regexp]

	// NEW: Hybrid indexes for O(1) lookups on struct arrays
	indexThreeStringByStr3 *syncops.SyncMap[map[string]*syncops.DbstaticThreeStringTwoInt] // IMDB ID -> entry
	indexThreeStringByNum2 *syncops.SyncMap[map[uint]*syncops.DbstaticThreeStringTwoInt]   // DB ID -> entry
	indexTwoIntComposite   *syncops.SyncMap[map[string]*syncops.DbstaticOneStringTwoInt]   // "mediaID:listname" -> entry
	indexTwoStringByStr1   *syncops.SyncMap[map[string][]*syncops.DbstaticTwoStringOneInt] // title -> entries (one-to-many)
	indexStringSet         *syncops.SyncMap[map[string]struct{}]                           // normalized string -> exists

	janitor       *time.Timer
	janitorCtx    context.Context
	janitorCancel context.CancelFunc
	mu            sync.RWMutex
}

var (
	strquery = "query"
	strcache = "cache"
	cache    = tcache{
		itemsstring:      syncops.NewSyncMap[[]string](20),
		itemstwoint:      syncops.NewSyncMap[[]syncops.DbstaticOneStringTwoInt](20),
		itemsthreestring: syncops.NewSyncMap[[]syncops.DbstaticThreeStringTwoInt](20),
		itemstwostring:   syncops.NewSyncMap[[]syncops.DbstaticTwoStringOneInt](20),
		interval:         10 * time.Minute, // Set default interval to 10 minutes
	}
	globalCache      *globalcache
	initOnce         sync.Once // To make sure that the cache is only initialized once
	janitorActive    bool
	cacheDispatchMap = map[string]cacheDispatchEntry{
		// Movies
		logger.CacheMovie: {
			config.MediaTypeMovie,
			logger.CacheMedia,
			logger.DBCountMedia,
			logger.DBCacheMedia,
			varKindOneStringTwoInt,
			guardMedia,
			"",
			indexNone,
		},
		logger.CacheDBMovie: {
			config.MediaTypeMovie,
			logger.CacheDBMedia,
			logger.DBCountDBMedia,
			logger.DBCacheDBMedia,
			varKindThreeString,
			guardMedia,
			"",
			indexThreeString,
		},
		logger.CacheTitlesMovie: {
			config.MediaTypeMovie,
			logger.CacheMediaTitles,
			logger.DBCountDBTitles,
			logger.DBCacheDBTitles,
			varKindTwoString,
			guardMedia,
			"",
			indexTwoString,
		},
		logger.CacheUnmatchedMovie: {
			config.MediaTypeMovie,
			logger.CacheUnmatched,
			logger.DBCountUnmatched,
			logger.DBCacheUnmatched,
			varKindString,
			guardFile,
			"DBRemoveUnmatched",
			indexNone,
		},
		logger.CacheFilesMovie: {
			config.MediaTypeMovie,
			logger.CacheFiles,
			logger.DBCountFiles,
			logger.DBCacheFiles,
			varKindString,
			guardFile,
			"",
			indexNone,
		},
		logger.CacheHistoryURLMovie: {
			config.MediaTypeMovie,
			logger.CacheHistoryURL,
			logger.DBCountHistoriesURL,
			logger.DBHistoriesURL,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheHistoryTitleMovie: {
			config.MediaTypeMovie,
			logger.CacheHistoryTitle,
			logger.DBCountHistoriesTitle,
			logger.DBHistoriesTitle,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		// Series (CacheDBSeriesAlt resolves from CacheMediaTitles via series StringsMap)
		logger.CacheSeries: {
			config.MediaTypeSeries,
			logger.CacheMedia,
			logger.DBCountMedia,
			logger.DBCacheMedia,
			varKindOneStringTwoInt,
			guardMedia,
			"",
			indexNone,
		},
		logger.CacheDBSeries: {
			config.MediaTypeSeries,
			logger.CacheDBMedia,
			logger.DBCountDBMedia,
			logger.DBCacheDBMedia,
			varKindThreeString,
			guardMedia,
			"",
			indexThreeString,
		},
		logger.CacheDBSeriesAlt: {
			config.MediaTypeSeries,
			logger.CacheMediaTitles,
			logger.DBCountDBTitles,
			logger.DBCacheDBTitles,
			varKindTwoString,
			guardMedia,
			"",
			indexTwoString,
		},
		logger.CacheUnmatchedSeries: {
			config.MediaTypeSeries,
			logger.CacheUnmatched,
			logger.DBCountUnmatched,
			logger.DBCacheUnmatched,
			varKindString,
			guardFile,
			"DBRemoveUnmatched",
			indexNone,
		},
		logger.CacheFilesSeries: {
			config.MediaTypeSeries,
			logger.CacheFiles,
			logger.DBCountFiles,
			logger.DBCacheFiles,
			varKindString,
			guardFile,
			"",
			indexNone,
		},
		logger.CacheHistoryURLSeries: {
			config.MediaTypeSeries,
			logger.CacheHistoryURL,
			logger.DBCountHistoriesURL,
			logger.DBHistoriesURL,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheHistoryTitleSeries: {
			config.MediaTypeSeries,
			logger.CacheHistoryTitle,
			logger.DBCountHistoriesTitle,
			logger.DBHistoriesTitle,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		// Books
		logger.CacheBook: {
			config.MediaTypeBook,
			logger.CacheMedia,
			logger.DBCountMedia,
			logger.DBCacheMedia,
			varKindOneStringTwoInt,
			guardMedia,
			"",
			indexNone,
		},
		logger.CacheDBBook: {
			config.MediaTypeBook,
			logger.CacheDBMedia,
			logger.DBCountDBMedia,
			logger.DBCacheDBMedia,
			varKindThreeString,
			guardMedia,
			"",
			indexThreeString,
		},
		logger.CacheTitlesBook: {
			config.MediaTypeBook,
			logger.CacheMediaTitles,
			logger.DBCountDBTitles,
			logger.DBCacheDBTitles,
			varKindTwoString,
			guardMedia,
			"",
			indexTwoString,
		},
		logger.CacheUnmatchedBook: {
			config.MediaTypeBook,
			logger.CacheUnmatched,
			logger.DBCountUnmatched,
			logger.DBCacheUnmatched,
			varKindString,
			guardFile,
			"DBRemoveUnmatched",
			indexNone,
		},
		logger.CacheFilesBook: {
			config.MediaTypeBook,
			logger.CacheFiles,
			logger.DBCountFiles,
			logger.DBCacheFiles,
			varKindString,
			guardFile,
			"",
			indexNone,
		},
		logger.CacheHistoryURLBook: {
			config.MediaTypeBook,
			logger.CacheHistoryURL,
			logger.DBCountHistoriesURL,
			logger.DBHistoriesURL,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheHistoryTitleBook: {
			config.MediaTypeBook,
			logger.CacheHistoryTitle,
			logger.DBCountHistoriesTitle,
			logger.DBHistoriesTitle,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		// Audiobooks
		logger.CacheAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheMedia,
			logger.DBCountMedia,
			logger.DBCacheMedia,
			varKindOneStringTwoInt,
			guardMedia,
			"",
			indexNone,
		},
		logger.CacheDBAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheDBMedia,
			logger.DBCountDBMedia,
			logger.DBCacheDBMedia,
			varKindThreeString,
			guardMedia,
			"",
			indexThreeString,
		},
		logger.CacheTitlesAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheMediaTitles,
			logger.DBCountDBTitles,
			logger.DBCacheDBTitles,
			varKindTwoString,
			guardMedia,
			"",
			indexTwoString,
		},
		logger.CacheUnmatchedAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheUnmatched,
			logger.DBCountUnmatched,
			logger.DBCacheUnmatched,
			varKindString,
			guardFile,
			"DBRemoveUnmatched",
			indexNone,
		},
		logger.CacheFilesAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheFiles,
			logger.DBCountFiles,
			logger.DBCacheFiles,
			varKindString,
			guardFile,
			"",
			indexStringSet,
		},
		logger.CacheHistoryURLAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheHistoryURL,
			logger.DBCountHistoriesURL,
			logger.DBHistoriesURL,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheHistoryTitleAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheHistoryTitle,
			logger.DBCountHistoriesTitle,
			logger.DBHistoriesTitle,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		// Music / Albums
		logger.CacheAlbum: {
			config.MediaTypeMusic,
			logger.CacheMedia,
			logger.DBCountMedia,
			logger.DBCacheMedia,
			varKindOneStringTwoInt,
			guardMedia,
			"",
			indexNone,
		},
		logger.CacheDBAlbum: {
			config.MediaTypeMusic,
			logger.CacheDBMedia,
			logger.DBCountDBMedia,
			logger.DBCacheDBMedia,
			varKindThreeString,
			guardMedia,
			"",
			indexThreeString,
		},
		logger.CacheTitlesAlbum: {
			config.MediaTypeMusic,
			logger.CacheMediaTitles,
			logger.DBCountDBTitles,
			logger.DBCacheDBTitles,
			varKindTwoString,
			guardMedia,
			"",
			indexTwoString,
		},
		logger.CacheUnmatchedAlbum: {
			config.MediaTypeMusic,
			logger.CacheUnmatched,
			logger.DBCountUnmatched,
			logger.DBCacheUnmatched,
			varKindString,
			guardFile,
			"DBRemoveUnmatched",
			indexNone,
		},
		logger.CacheFilesAlbum: {
			config.MediaTypeMusic,
			logger.CacheFiles,
			logger.DBCountFiles,
			logger.DBCacheFiles,
			varKindString,
			guardFile,
			"",
			indexStringSet,
		},
		logger.CacheHistoryURLAlbum: {
			config.MediaTypeMusic,
			logger.CacheHistoryURL,
			logger.DBCountHistoriesURL,
			logger.DBHistoriesURL,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheHistoryTitleAlbum: {
			config.MediaTypeMusic,
			logger.CacheHistoryTitle,
			logger.DBCountHistoriesTitle,
			logger.DBHistoriesTitle,
			varKindString,
			guardHistory,
			"",
			indexStringSet,
		},
		logger.CacheRootpathAlbum: {
			config.MediaTypeMusic,
			logger.CacheRootpath,
			logger.DBCountRootpath,
			logger.DBCacheRootpath,
			varKindString,
			guardFile,
			"",
			indexStringSet,
		},
		logger.CacheRootpathAudiobook: {
			config.MediaTypeAudiobook,
			logger.CacheRootpath,
			logger.DBCountRootpath,
			logger.DBCacheRootpath,
			varKindString,
			guardFile,
			"",
			indexStringSet,
		},
	}
)

// getexpiressql returns the expiry time in nanoseconds for cached statements.
// It checks the defaultextension field, and if greater than 0, sets the expiry to that duration from the current time.
// Otherwise it returns 0 indicating no expiry.
func (c *globalcache) getexpiressql(static bool) int64 {
	if static {
		return 0
	}

	c.mu.RLock()

	defaultExt := c.defaultextension
	c.mu.RUnlock()

	if defaultExt > 0 {
		return time.Now().Add(defaultExt).UnixNano()
	}

	return 0
}

// addStaticXStmt adds a prepared SQL statement to the Ristretto cache with the given key.
// If the statement already exists in the cache, it returns without doing anything.
// Static statements are cached permanently. Falls back to old implementation if Ristretto not initialized.
func (*globalcache) addStaticXStmt(key string, imdb bool) {
	// Check if already in Ristretto cache
	if _, found := cache.ristrettoStmt.Get(key); found {
		return
	}

	stmt := preparestmt(imdb, key)
	if stmt != nil {
		cache.ristrettoStmt.Set(key, stmt, 1) // Static statements never expire
		cache.ristrettoStmt.Wait()
	}
}

// getXStmt retrieves a cached SQL statement using Ristretto cache.
// If the statement is not found, it prepares the statement and caches it.
func (*globalcache) getXStmt(key string, imdb bool) *sqlx.Stmt {
	// Try Ristretto cache first
	if stmt, found := cache.ristrettoStmt.Get(key); found {
		return stmt
	}

	// Cache miss - prepare statement and cache it
	stmt := preparestmt(imdb, key)
	if stmt != nil {
		cache.ristrettoStmt.Set(key, stmt, 1)
		cache.ristrettoStmt.Wait()
	} else {
		_, err := Getdb(imdb).Preparex(key)
		logger.Logtype("error", 1).
			Str(strQuery, key).
			Err(err).
			Msg("stmt failed")
	}

	return stmt
}

// preparestmt prepares a SQL statement using the provided database connection and SQL query key.
// If an error occurs or the prepared statement is nil, it returns an empty sqlx.Stmt.
// Otherwise, it returns the prepared statement.
func preparestmt(imdb bool, key string) *sqlx.Stmt {
	sq, err := Getdb(imdb).Preparex(key)
	if err != nil || sq == nil {
		return nil
	}

	return sq
}

// setStaticRegexp sets a cached regular expression with the given key. If the cached regular expression
// does not exist, it compiles and caches it using Ristretto. This function is used to cache regular
// expressions that are used statically throughout the application.
func (*globalcache) setStaticRegexp(key string) {
	// Check if already cached in Ristretto
	if _, found := cache.ristrettoRegex.Get(key); found {
		return
	}

	rgx := getregexP(key)
	cache.ristrettoRegex.Set(key, rgx, 1) // Static patterns never expire (cost=1)
	cache.ristrettoRegex.Wait()
}

// setRegexp sets a cached regular expression with the given key using Ristretto cache.
// If the cached regular expression does not exist, it creates a new one and adds it to the cache.
// The function takes a key string and a duration time.Duration for TTL.
// If duration is 0, patterns are cached permanently. Returns the compiled regexp.
func (*globalcache) setRegexp(key string, duration time.Duration) *regexp.Regexp {
	// Try Ristretto cache first
	if rgx, found := cache.ristrettoRegex.Get(key); found {
		return rgx
	}

	// Cache miss - compile and cache
	rgx := getregexP(key)

	if duration > 0 {
		// Cache with TTL
		cache.ristrettoRegex.SetWithTTL(key, rgx, 1, duration)
	} else {
		// Cache permanently
		cache.ristrettoRegex.Set(key, rgx, 1)
	}

	cache.ristrettoRegex.Wait() // Ensure write completes

	return rgx
}

// InitCache initializes the global cache by creating a new Cache instance
// with the provided expiration times and logger. It is called on startup
// to initialize the cache before it is used.
func InitCache() {
	hours := 24
	if s := config.GetSettingsGeneral(); s != nil {
		hours = s.CacheDuration
	}

	NewCache(1*time.Hour, time.Duration(hours)*time.Hour)
}

// ClearCaches iterates over the cached string, three string two int, and two string int arrays, sets the Expire field on each cached array object to two hours in the past based on the config cache duration, and updates the cache with the expired array object. This effectively clears those cached arrays by expiring all entries.
func ClearCaches() {
	syncops.QueueSyncMapDeleteFunc(syncops.MapTypeString, func(item []string) bool {
		clear(item)
		return true
	})
	syncops.QueueSyncMapDeleteFunc(
		syncops.MapTypeThreeString,
		func(item []syncops.DbstaticThreeStringTwoInt) bool {
			clear(item)
			return true
		},
	)
	syncops.QueueSyncMapDeleteFunc(
		syncops.MapTypeTwoInt,
		func(item []syncops.DbstaticOneStringTwoInt) bool {
			clear(item)
			return true
		},
	)
	syncops.QueueSyncMapDeleteFunc(
		syncops.MapTypeTwoString,
		func(item []syncops.DbstaticTwoStringOneInt) bool {
			clear(item)
			return true
		},
	)

	// Clear all index maps
	cache.indexThreeStringByStr3.DeleteFunc(
		func(_ map[string]*syncops.DbstaticThreeStringTwoInt) bool { return true },
	)
	cache.indexThreeStringByNum2.DeleteFunc(
		func(_ map[uint]*syncops.DbstaticThreeStringTwoInt) bool { return true },
	)
	cache.indexTwoStringByStr1.DeleteFunc(
		func(_ map[string][]*syncops.DbstaticTwoStringOneInt) bool { return true },
	)
	cache.indexStringSet.DeleteFunc(func(_ map[string]struct{}) bool { return true })
	cache.indexTwoIntComposite.DeleteFunc(
		func(_ map[string]*syncops.DbstaticOneStringTwoInt) bool { return true },
	)

	if cache.ristrettoStmt != nil {
		cache.ristrettoStmt.Clear()
	}
}

// AppendCache appends the given value v to the cached array for the given key a.
// If the cache has an associated indexStringSet, the normalized key is also added
// to that index so lookups remain consistent without a full rebuild.
func AppendCache(a string, v string) {
	syncops.QueueAtomicAppendString(syncops.MapTypeString, a, v)

	if entry, ok := cacheDispatchMap[a]; ok && entry.indexKind == indexStringSet {
		normalizedV := normalizeKey(v)
		cache.indexStringSet.ModifyInPlace(a, func(m map[string]struct{}) {
			m[normalizedV] = struct{}{}
		})
	}
}

// AppendCacheThreeString appends the given syncops.DbstaticThreeStringTwoInt value v to the cached array
// for the given key a. This function uses sequential SyncMap operations which may create race conditions
// if called concurrently. It should only be called from single-threaded contexts or with external synchronization.
func AppendCacheThreeString(a string, v syncops.DbstaticThreeStringTwoInt) {
	if cache.itemsthreestring.Check(a) {
		syncopsValue := syncops.DbstaticThreeStringTwoInt{
			Str1: v.Str1,
			Str2: v.Str2,
			Str3: v.Str3,
			Num1: v.Num1,
			Num2: v.Num2,
		}
		syncops.QueueSliceAppend(syncops.MapTypeThreeString, a, syncopsValue)
		appendIndexThreeString(a, syncopsValue)
	}
}

// AppendCacheTwoInt appends the given syncops.DbstaticOneStringTwoInt value v to the cached array
// for the given key a. This function uses sequential SyncMap operations which may create race conditions
// if called concurrently. It should only be called from single-threaded contexts or with external synchronization.
func AppendCacheTwoInt(a string, v syncops.DbstaticOneStringTwoInt) {
	if cache.itemstwoint.Check(a) {
		syncopsValue := syncops.DbstaticOneStringTwoInt{
			Str:  v.Str,
			Num1: v.Num1,
			Num2: v.Num2,
		}
		syncops.QueueSliceAppend(syncops.MapTypeTwoInt, a, syncopsValue)
		appendIndexTwoInt(a, syncopsValue)
	}
}

// AppendCacheTwoString appends the given syncops.DbstaticTwoStringOneInt value v to the cached array
// for the given key a. This function uses sequential SyncMap operations which may create race conditions
// if called concurrently. It should only be called from single-threaded contexts or with external synchronization.
func AppendCacheTwoString(a string, v syncops.DbstaticTwoStringOneInt) {
	if cache.itemstwostring.Check(a) {
		syncopsValue := syncops.DbstaticTwoStringOneInt{
			Str1: v.Str1,
			Str2: v.Str2,
			Num:  v.Num,
		}
		syncops.QueueSliceAppend(syncops.MapTypeTwoString, a, syncopsValue)
		appendIndexTwoString(a, syncopsValue)
	}
}

// AppendCacheMap appends a value to the cache map for the given query string.
// If isType is true, it appends to the logger.Mapstringsseries map, otherwise it appends to the logger.Mapstringsmovies map.
// The value is appended using the AppendCache function.
func AppendCacheMap(isType uint, query string, v string) {
	AppendCache(mtstrings.GetStringsMap(isType, query), v)
}

// ArrStructContains checks if the given slice s contains the value v.
// The comparison is performed using the comparable constraint, which allows
// comparing values of the same type. If the values are of a custom struct type,
// the comparison is performed by checking if all fields of the struct are equal.
func ArrStructContains(s []DbstaticTwoUint, v DbstaticTwoUint) bool {
	return slices.Contains(s, v)
}

// ArrStructContainsString checks if the given slice s contains the string value v.
// It iterates over the slice and returns true if a match is found, false otherwise.
func ArrStructContainsString(s []string, v string) bool {
	return slices.Contains(s, v)
}

// SlicesCacheContainsI checks if string v exists in the cached string array
// identified by s using a case-insensitive comparison. It gets the cached
// array, iterates over it, compares each element to v with EqualFold,
// and returns true if a match is found.
func SlicesCacheContainsI(isType uint, query string, w *string) bool {
	if w == nil || *w == "" {
		return false
	}

	v := *w

	arr := GetCachedStringArr(mtstrings.GetStringsMap(isType, query), false, true)
	for i := range arr {
		if v == arr[i] || strings.EqualFold(v, arr[i]) {
			return true
		}
	}

	return false
}

// SlicesCacheContains checks if the cached string array identified by s contains
// the value v. It iterates over the array and returns true if a match is found.
func SlicesCacheContains(isType uint, query, v string) bool {
	return slices.Contains(
		GetCachedStringArr(mtstrings.GetStringsMap(isType, query), false, true),
		v,
	)
}

// SlicesCacheContainsDelete removes an element matching v from the cached string
// array identified by s. If the cache has an associated indexStringSet, the
// normalized key is also removed from that index so lookups remain consistent.
func SlicesCacheContainsDelete(s, v string) {
	syncops.QueueAtomicRemoveString(syncops.MapTypeString, s, v)

	if entry, ok := cacheDispatchMap[s]; ok && entry.indexKind == indexStringSet {
		normalizedV := normalizeKey(v)
		cache.indexStringSet.ModifyInPlace(s, func(m map[string]struct{}) {
			delete(m, normalizedV)
		})
	}
}

// CacheOneStringTwoIntIndexFunc looks up the cached one string two int array
// identified by s and calls the passed in function f on each element.
// Returns true if f returns true for any element.
func CacheOneStringTwoIntIndexFunc(s string, f func(*syncops.DbstaticOneStringTwoInt) bool) bool {
	arr := GetCachedTwoIntArr(s, false, true)
	for i := range arr {
		if f(&arr[i]) {
			return true
		}
	}

	return false
}

// CacheOneStringTwoIntIndexFuncRet searches the cached one string two int array
// identified by s by applying the passed in function f to each element. If f returns
// true for any element, the Num2 field of that element is returned. If no match is
// found, 0 is returned.
func CacheOneStringTwoIntIndexFuncRet(s string, id uint, listname string) uint {
	arr := GetCachedTwoIntArr(s, false, true)
	for i := range arr {
		if arr[i].Num1 == id && strings.EqualFold(arr[i].Str, listname) {
			return arr[i].Num2
		}
	}

	return 0
}

// CacheOneStringTwoIntIndexFuncStr looks up the cached one string, two int array
// identified by s and returns the string value for the entry where the
// second int matches the passed in uint i. It stores the returned string in
// listname. If no match is found, it sets listname to an empty string.
func CacheOneStringTwoIntIndexFuncStr(isType uint, query string, i uint) string {
	arr := GetCachedTwoIntArr(mtstrings.GetStringsMap(isType, query), false, true)
	for j := range arr {
		if arr[j].Num2 == i {
			return arr[j].Str
		}
	}

	return ""
}

// CacheThreeStringIntIndexFunc looks up the cached three string, two int array
// identified by s and returns the second int value for the entry where the
// third string matches the string str. Returns 0 if no match found.
func CacheThreeStringIntIndexFunc(s string, u *string) uint {
	if u == nil || *u == "" {
		return 0
	}

	t := *u

	arr := GetCachedThreeStringArr(s, false, true)
	for i := range arr {
		if arr[i].Str3 == t || strings.EqualFold(arr[i].Str3, t) {
			return arr[i].Num2
		}
	}

	return 0
}

// CacheThreeStringIntIndexFuncGetYear looks up the cached three string, two int array
// identified by s and returns the first int value for the entry where the second int
// matches the int str. Returns 0 if no match found.
func CacheThreeStringIntIndexFuncGetYear(s string, i uint) uint16 {
	arr := GetCachedThreeStringArr(s, false, true)
	for j := range arr {
		if arr[j].Num2 == i {
			return uint16(arr[j].Num1) //nolint:gosec // safe: value within target type range
		}
	}

	return 0
}

// refreshCacheDBInternal is a generic function that refreshes a cache for a specific type of data.
// It handles locking the cache, checking if a refresh is needed, and updating the cache
// with new data from the database. The function supports different cache types based on
// the generic type parameter and allows forcing a refresh.
// This function should only be called from the single cache writer goroutine.
//
// Parameters:
//   - isType: Determines whether to use series-specific or movie-specific data
//   - force: If true, forces a cache refresh regardless of existing cache state
//   - maptypestr: String identifier for the map type
//   - mapcountsql: SQL query to get the count of items in the cache
//   - mapsql: SQL query to retrieve the cache items
//   - cachevar: Synchronized map to store the cache items
func refreshCacheDBInternal[t comparable](
	isType uint,
	force bool,
	maptypestr string,
	mapcountsql string,
	mapsql string,
	cachevar *syncops.SyncMap[[]t],
) {
	mapvar := mtstrings.GetStringsMap(isType, maptypestr)
	item := getCachedArrayDirect(cachevar, mapvar, true, false)

	count := Getdatarow[uint](
		false,
		mtstrings.GetStringsMap(isType, mapcountsql),
		&config.GetSettingsGeneral().CacheDuration,
	)
	if !force && len(item) == int(count) { //nolint:gosec // safe: value within target type range
		return
	}

	if count == 0 {
		if len(item) > 0 || !cachevar.Check(mapvar) {
			storeMapType(
				cachevar,
				mapvar,
				make([]t, 0, 100),
				time.Now().
					Add(time.Duration(config.GetSettingsGeneral().CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}

		return
	}

	lastscan := cachevar.GetLastScan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}

	logger.Logtype("debug", 1).
		Str(strcache, mapvar).
		Msg("refresh cache")

	data := GetrowsN[t](
		false,
		count+100,
		mtstrings.GetStringsMap(isType, mapsql),
		&config.GetSettingsGeneral().CacheDuration,
	)
	if len(data) > 0 {
		storeMapType(
			cachevar,
			mapvar,
			data,
			time.Now().
				Add(time.Duration(config.GetSettingsGeneral().CacheDuration)*time.Hour).
				UnixNano(),
			time.Now().UnixNano(),
		)
	} else {
		logger.Logtype("debug", 1).
			Str(strcache, mapvar).
			Msg("refresh cache empty")
	}
}

// RefreshMediaCacheDB refreshes the media caches for movies and series.
// It will refresh the caches based on the isType parameter:
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheDB(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheDBMedia,
		logger.DBCountDBMedia,
		logger.DBCacheDBMedia,
		cache.itemsthreestring,
	)
}

// RefreshMediaCacheList refreshes the media caches for movies and series.
// It will refresh the caches based on the isType parameter:
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheList(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheMedia,
		logger.DBCountMedia,
		logger.DBCacheMedia,
		cache.itemstwoint,
	)
}

// Refreshhistorycachetitle refreshes the cached history title arrays for movies or series.
// The isType parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshhistorycachetitle(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheHistoryTitle,
		logger.DBCountHistoriesTitle,
		logger.DBHistoriesTitle,
		cache.itemsstring,
	)
}

// Refreshhistorycacheurl refreshes the cached history URL arrays for movies or series.
// The isType parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshhistorycacheurl(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheHistoryURL,
		logger.DBCountHistoriesURL,
		logger.DBHistoriesURL,
		cache.itemsstring,
	)
}

// RefreshMediaCacheTitles refreshes the cached media title arrays for movies or series.
// The isType parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheTitles(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheMediaTitles,
		logger.DBCountDBTitles,
		logger.DBCacheDBTitles,
		cache.itemstwostring,
	)
}

// Refreshfilescached refreshes the cached file location arrays for movies or series.
// The isType parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshfilescached(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseFileCache {
		return
	}

	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheFiles,
		logger.DBCountFiles,
		logger.DBCacheFiles,
		cache.itemsstring,
	)
}

// Refreshunmatchedcached refreshes the cached string array of unmatched files for movies or series.
// The isType parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshunmatchedcached(isType uint, force bool) {
	if !config.GetSettingsGeneral().UseFileCache {
		return
	}

	ExecNMap(isType, "DBRemoveUnmatched", &config.GetSettingsGeneral().CacheDuration2)
	refreshCacheDBInternal(
		isType,
		force,
		logger.CacheUnmatched,
		logger.DBCountUnmatched,
		logger.DBCacheUnmatched,
		cache.itemsstring,
	)
}

// varKind selects which SyncMap is used for a cache entry.
const (
	varKindThreeString     uint8 = iota // cache.itemsthreestring
	varKindTwoString                    // cache.itemstwostring
	varKindOneStringTwoInt              // cache.itemstwoint
	varKindString                       // cache.itemsstring
)

// guardXxx selects which enabled-setting guards a cache refresh.
const (
	guardMedia uint8 = iota
	guardHistory
	guardFile
)

// indexXxx selects which index (if any) to build after a data refresh.
const (
	indexNone        uint8 = iota
	indexThreeString       // indexThreeStringByStr3 + indexThreeStringByNum2
	indexTwoString         // indexTwoStringByStr1
	indexStringSet         // indexStringSet
)

// cacheDispatchEntry holds everything RefreshCached needs to refresh one cache key.
// cacheKey/countKey/dataKey are GENERIC keys resolved via mtstrings.GetStringsMap(isType, …)
// inside refreshCacheDBInternal, so they are identical across all media types.
type cacheDispatchEntry struct {
	isType    uint
	cacheKey  string // generic cache key (also used for lastScan lookup)
	countKey  string // generic count-SQL key
	dataKey   string // generic data-SQL key
	varKind   uint8
	guard     uint8
	removeKey string // if non-empty, ExecNMap is called before refresh (unmatched only)
	indexKind uint8
}

// doRefreshInternal calls refreshCacheDBInternal with the SyncMap selected by e.varKind.
func doRefreshInternal(e cacheDispatchEntry, force bool) {
	switch e.varKind {
	case varKindThreeString:
		refreshCacheDBInternal(
			e.isType,
			force,
			e.cacheKey,
			e.countKey,
			e.dataKey,
			cache.itemsthreestring,
		)

	case varKindTwoString:
		refreshCacheDBInternal(
			e.isType,
			force,
			e.cacheKey,
			e.countKey,
			e.dataKey,
			cache.itemstwostring,
		)

	case varKindOneStringTwoInt:
		refreshCacheDBInternal(
			e.isType,
			force,
			e.cacheKey,
			e.countKey,
			e.dataKey,
			cache.itemstwoint,
		)

	case varKindString:
		refreshCacheDBInternal(
			e.isType,
			force,
			e.cacheKey,
			e.countKey,
			e.dataKey,
			cache.itemsstring,
		)
	}
}

// RefreshCached refreshes the cached data for the specified key.
// Guard check, optional pre-step, data refresh, and index build are all handled here.
func RefreshCached(key string, force bool) {
	entry, ok := cacheDispatchMap[key]
	if !ok {
		return
	}

	switch entry.guard {
	case guardMedia:
		if !config.GetSettingsGeneral().UseMediaCache {
			return
		}

	case guardHistory:
		if !config.GetSettingsGeneral().UseHistoryCache {
			return
		}

	case guardFile:
		if !config.GetSettingsGeneral().UseFileCache {
			return
		}
	}

	if entry.removeKey != "" {
		ExecNMap(entry.isType, entry.removeKey, &config.GetSettingsGeneral().CacheDuration2)
	}

	if entry.indexKind == indexNone {
		doRefreshInternal(entry, force)
		return
	}

	mapvar := mtstrings.GetStringsMap(entry.isType, entry.cacheKey)
	lastScanBefore := getLastScanForKind(entry.indexKind, mapvar)
	doRefreshInternal(entry, force)

	if !config.GetSettingsGeneral().UseIndexedCache {
		return
	}

	kind := entry.indexKind
	syncops.QueueFunc(func() {
		if !force && lastScanBefore != 0 && getLastScanForKind(kind, mapvar) == lastScanBefore {
			return
		}

		buildAndStoreIndex(kind, mapvar)
	})
}

// GetCachedTwoStringArr retrieves the cached array of syncops.DbstaticTwoStringOneInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedTwoStringArr(
	key string,
	checkexpire bool,
	retry bool,
) []syncops.DbstaticTwoStringOneInt {
	if cache.itemstwostring.Check(key) {
		if checkexpire {
			syncops.QueueSyncMapCheckExpires(
				syncops.MapTypeTwoString,
				key,
				config.GetSettingsGeneral().CacheAutoExtend,
				config.GetSettingsGeneral().CacheDuration,
			)
		}

		return cache.itemstwostring.GetVal(key)
	}

	if retry {
		return getrefresh(cache.itemstwostring, key, retry)
	}

	return nil
}

// GetCachedTwoIntArr retrieves the cached array of syncops.DbstaticOneStringTwoInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedTwoIntArr(
	key string,
	checkexpire bool,
	retry bool,
) []syncops.DbstaticOneStringTwoInt {
	if cache.itemstwoint.Check(key) {
		if checkexpire {
			syncops.QueueSyncMapCheckExpires(
				syncops.MapTypeTwoInt,
				key,
				config.GetSettingsGeneral().CacheAutoExtend,
				config.GetSettingsGeneral().CacheDuration,
			)
		}

		return cache.itemstwoint.GetVal(key)
	}

	if retry {
		return getrefresh(cache.itemstwoint, key, retry)
	}

	return nil
}

// GetCachedThreeStringArr retrieves the cached array of syncops.DbstaticThreeStringTwoInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedThreeStringArr(
	key string,
	checkexpire bool,
	retry bool,
) []syncops.DbstaticThreeStringTwoInt {
	if cache.itemsthreestring.Check(key) {
		if checkexpire {
			syncops.QueueSyncMapCheckExpires(
				syncops.MapTypeThreeString,
				key,
				config.GetSettingsGeneral().CacheAutoExtend,
				config.GetSettingsGeneral().CacheDuration,
			)
		}

		return cache.itemsthreestring.GetVal(key)
	}

	if retry {
		return getrefresh(cache.itemsthreestring, key, retry)
	}

	return nil
}

// GetCachedThreeStringSeq returns an iterator over the cached ThreeString slice.
// Yields pointers into the underlying slice to avoid per-element struct copies.
// Safe for early exit: the caller may return or break without draining the sequence.
func GetCachedThreeStringSeq(
	key string,
	checkexpire bool,
	retry bool,
) iter.Seq[*syncops.DbstaticThreeStringTwoInt] {
	arr := GetCachedThreeStringArr(key, checkexpire, retry)

	return func(yield func(*syncops.DbstaticThreeStringTwoInt) bool) {
		for i := range arr {
			if !yield(&arr[i]) {
				return
			}
		}
	}
}

// GetCachedTwoStringSeq returns an iterator over the cached TwoString slice.
// Yields pointers into the underlying slice to avoid per-element struct copies.
func GetCachedTwoStringSeq(
	key string,
	checkexpire bool,
	retry bool,
) iter.Seq[*syncops.DbstaticTwoStringOneInt] {
	arr := GetCachedTwoStringArr(key, checkexpire, retry)

	return func(yield func(*syncops.DbstaticTwoStringOneInt) bool) {
		for i := range arr {
			if !yield(&arr[i]) {
				return
			}
		}
	}
}

// GetCachedTwoIntSeq returns an iterator over the cached TwoInt slice.
// Yields pointers into the underlying slice to avoid per-element struct copies.
func GetCachedTwoIntSeq(
	key string,
	checkexpire bool,
	retry bool,
) iter.Seq[*syncops.DbstaticOneStringTwoInt] {
	arr := GetCachedTwoIntArr(key, checkexpire, retry)

	return func(yield func(*syncops.DbstaticOneStringTwoInt) bool) {
		for i := range arr {
			if !yield(&arr[i]) {
				return
			}
		}
	}
}

// GetCachedStringSeq returns an iterator over the cached string slice.
func GetCachedStringSeq(key string, checkexpire bool, retry bool) iter.Seq[string] {
	arr := GetCachedStringArr(key, checkexpire, retry)

	return func(yield func(string) bool) {
		for i := range arr {
			if !yield(arr[i]) {
				return
			}
		}
	}
}

// GetCachedStringArr retrieves a cached array of strings.
// If the key exists and checkexpire is true, it checks for cache expiration.
// If the cache is expired or not found, it attempts to refresh the cache based on the retry flag.
// Returns the cached array or nil if not found or expired.
func GetCachedStringArr(key string, checkexpire bool, retry bool) []string {
	if cache.itemsstring.Check(key) {
		if checkexpire {
			syncops.QueueSyncMapCheckExpires(
				syncops.MapTypeString,
				key,
				config.GetSettingsGeneral().CacheAutoExtend,
				config.GetSettingsGeneral().CacheDuration,
			)
		}

		return cache.itemsstring.GetVal(key)
	}

	if retry {
		return getrefresh(cache.itemsstring, key, retry)
	}

	return nil
}

// getCachedArrayDirect retrieves a cached array directly from a SyncMap without
// going through the syncops queue system. This is used internally by the cache
// refresh system and should not be used by external callers.
func getCachedArrayDirect[t comparable](
	c *syncops.SyncMap[[]t],
	key string,
	checkexpire bool,
	retry bool,
) []t {
	if c.Check(key) {
		if checkexpire {
			// For internal cache operations, we don't need to queue the operation
			// since this is likely being called from within the cache refresh system
			expired := c.CheckExpires(
				key,
				config.GetSettingsGeneral().CacheAutoExtend,
				config.GetSettingsGeneral().CacheDuration,
			)
			if expired {
				return nil
			}
		}

		return c.GetVal(key)
	}

	if retry {
		return getrefresh(c, key, retry)
	}

	return nil
}

// getrefresh retrieves the value associated with the given key from the SyncMap, and refreshes the
// cached value if the retry flag is set and the cached value is empty or the zero value of the
// generic type T.
func getrefresh[T comparable](s *syncops.SyncMap[[]T], key string, retry bool) []T {
	t := s.GetVal(key)
	if retry {
		if len(t) == 0 {
			return forcerefresh(s, key)
		} else {
			var x T
			if t[0] == x {
				return forcerefresh(s, key)
			}
		}
	}

	return t
}

// forcerefresh retrieves and refreshes the cached value for a given key in a SyncMap.
// It triggers a cache refresh for the specified key and returns the updated value.
// The function is generic and works with any slice type that is comparable.
func forcerefresh[T comparable](s *syncops.SyncMap[[]T], key string) []T {
	RefreshCached(key, true)
	return s.GetVal(key)
}

// storeMapType updates or adds a value to a SyncMap with the given key, updating expiration time, value, and last scan timestamp.
// If the key already exists in the map, it updates the existing entry; otherwise, it adds a new entry.
// The function is generic and works with any slice type, logging a debug message during the process.
// This function should only be called from the single cache writer goroutine.
func storeMapType[t any](
	c *syncops.SyncMap[[]t],
	s string,
	val []t,
	expires, lastscan int64,
) {
	logger.Logtype("debug", 1).
		Str(strquery, s).
		Msg("refresh cache store")

	// WARNING: This function uses sequential SyncMap operations which may create race conditions
	// if called concurrently. It should only be called from single-threaded contexts.
	if c.Check(s) {
		// Use syncops for atomic operations to prevent race conditions
		mapType := getMapTypeForSyncMap(c)
		if mapType != "" {
			syncops.QueueSyncMapUpdateExpire(mapType, s, expires)
			syncops.QueueSyncMapUpdateVal(mapType, s, val)
			syncops.QueueSyncMapUpdateLastscan(mapType, s, lastscan)
		} else {
			// Log error if map type cannot be determined
			logger.Logtype("error", 1).
				Str("key", s).
				Msg("Unable to determine MapType for SyncMap in storeMapType")
		}

		return
	}

	// Use syncops for Add operation as well
	mapType := getMapTypeForSyncMap(c)
	if mapType != "" {
		syncops.QueueSyncMapAdd(mapType, s, val, expires, false, lastscan)
	} else {
		// Log error if map type cannot be determined
		logger.Logtype("error", 1).
			Str("key", s).
			Msg("Unable to determine MapType for SyncMap in storeMapType Add")
	}
}

// getMapTypeForSyncMap determines the MapType for a given SyncMap pointer.
func getMapTypeForSyncMap(c any) syncops.MapType {
	switch c {
	case cache.itemsstring:
		return syncops.MapTypeString
	case cache.itemstwostring:
		return syncops.MapTypeTwoString
	case cache.itemsthreestring:
		return syncops.MapTypeThreeString
	case cache.itemstwoint:
		return syncops.MapTypeTwoInt
	default:
		return ""
	}
}

// CheckcachedTitleHistory checks if the given title exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the title exists, false
// otherwise.
func CheckcachedTitleHistory(isType uint, file *string) bool {
	if config.GetSettingsGeneral().UseFileCache {
		return SlicesCacheContainsI(isType, logger.CacheHistoryTitle, file)
	}

	return Getdatarow[uint](
		false,
		mtstrings.GetStringsMap(isType, logger.DBCountHistoriesByTitle),
		file,
	) >= 1
}

// CheckcachedURLHistory checks if the given URL exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the URL exists, false
// otherwise.
func CheckcachedURLHistory(isType uint, file *string) bool {
	if config.GetSettingsGeneral().UseFileCache {
		return SlicesCacheContainsI(isType, logger.CacheHistoryURL, file)
	}

	return Getdatarow[uint](
		false,
		mtstrings.GetStringsMap(isType, logger.DBCountHistoriesByURL),
		file,
	) >= 1
}

// InvalidateImdbStmt clears all cached prepared statements.
// Since we use Ristretto which doesn't track IMDB vs non-IMDB statements,
// we clear all statements and let them be re-prepared on demand.
// This is called when the IMDB database is exchanged.
func InvalidateImdbStmt() {
	if cache.ristrettoStmt != nil {
		cache.ristrettoStmt.Clear()
	}
}

// getregexP returns a compiled regular expression based on the provided key.
// It supports the following keys:
//
// "RegexSeriesTitle": Matches a series title with optional season and episode information.
// "RegexSeriesTitleDate": Matches a series title with a date.
// "RegexSeriesIdentifier": Matches a series identifier (season and episode or date).
// For any other key, it returns a regular expression compiled from the key string.
func getregexP(key string) *regexp.Regexp {
	switch key {
	case "RegexSeriesTitle":
		return regexp.MustCompile(
			`^(.*)(?i)(?:(?:\.| - |-)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$))`,
		)

	case "RegexSeriesTitleDate":
		return regexp.MustCompile(
			`^(.*)(?i)(?:\.|-| |_)(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$)`,
		)

	case "RegexSeriesIdentifier":
		return regexp.MustCompile(
			`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`,
		)

	default:
		return regexp.MustCompile(key)
	}
}

// RunRetRegex returns the indexes of the last submatch found in matchfor
// using the compiled regular expression stored in the global cache under key.
// If useall is true, it returns the indexes of the last submatch from all
// matches found, otherwise it returns the indexes of the first submatch.
// If no matches are found, it returns nil.
func RunRetRegex(key string, matchfor string, useall bool) []int {
	if useall {
		matches := findAllStringSubmatchIndex(key, matchfor)
		if len(matches) < 1 || len(matches[len(matches)-1]) < 1 {
			return nil
		}

		return matches[len(matches)-1]
	}

	return findStringSubmatchIndex(key, matchfor)
}

// findAllStringSubmatchIndex returns all string submatch indexes for a given key and matchfor string
// with a maximum of 10 matches. It first attempts to use a cached precompiled
// regular expression, falling back to compiling a new regular expression if needed.
// Returns a slice of string submatch indexes or nil if no matches are found.
func findAllStringSubmatchIndex(key string, matchfor string) [][]int {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	return rgx.FindAllStringSubmatchIndex(matchfor, 10)
}

// findStringSubmatchIndex returns the first string submatch index for a given key and matchfor string.
// It first attempts to use a cached precompiled regular expression, falling back to
// compiling a new regular expression if needed. Returns a slice of string submatch indexes
// or nil if no matches are found.
func findStringSubmatchIndex(key string, matchfor string) []int {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	return rgx.FindStringSubmatchIndex(matchfor)
}

// RegexGetMatchesFind checks if the regular expression matches the input
// string at least mincount times. It returns true if there are at least
// mincount matches, false otherwise. The regular expression is retrieved
// from the global cache.
func RegexGetMatchesFind(key, matchfor string, mincount int) bool {
	if mincount == 1 {
		return len(findStringIndex(key, matchfor)) >= 1
	}

	return len(findAllStringIndex(key, matchfor, mincount)) >= mincount
}

// findStringIndex returns the first string index match for a given key and matchfor string.
// It first attempts to use a cached precompiled regular expression, falling back to
// compiling a new regular expression if needed. Returns a slice of string index matches
// or nil if no matches are found.
func findStringIndex(key string, matchfor string) []int {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	return rgx.FindStringIndex(matchfor)
}

// findAllStringIndex returns all string index matches for a given key and matchfor string
// with a minimum count of matches specified. It first attempts to use a cached precompiled
// regular expression, falling back to compiling a new regular expression if needed.
// Returns a slice of string index matches or nil if no matches are found.
func findAllStringIndex(key string, matchfor string, mincount int) [][]int {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	return rgx.FindAllStringIndex(matchfor, mincount)
}

// getmatches returns the indexes of the first submatch found in matchfor
// using the compiled regular expression stored in cache under key. If cached
// is false, it will compile the regex and find matches without caching.
func getmatches(cached bool, key, matchfor string) []int {
	if !cached {
		return regexp.MustCompile(key).FindStringSubmatchIndex(matchfor)
	}

	return RunRetRegex(key, matchfor, false)
}

// RegexGetMatchesStr1Str2 returns the first and second submatches found in the
// input string 'matchfor' using the regular expression stored in the cache
// under the key 'key'. If 'cached' is true, the cached regular expression is
// used, otherwise a new one is compiled. The function returns the first and
// second submatches as strings, or empty strings if no matches or less than
// two submatches are found.
func RegexGetMatchesStr1Str2(cached bool, key, matchfor string) (string, string) {
	matches := getmatches(cached, key, matchfor)
	switch {
	case len(matches) == 0:
		return "", ""
	case len(matches) >= 6:
		if matches[3] != -1 && matches[5] != -1 {
			return matchfor[matches[2]:matches[3]], matchfor[matches[4]:matches[5]]
		}

		if matches[3] == -1 && matches[5] != -1 {
			return "", matchfor[matches[4]:matches[5]]
		}

		if matches[3] != -1 {
			return matchfor[matches[2]:matches[3]], ""
		}

	case len(matches) >= 4:
		if matches[3] != -1 {
			return matchfor[matches[2]:matches[3]], ""
		}

	default:
		return "", ""
	}

	return "", ""
}

// startJanitor starts the background cache janitor goroutine with proper cleanup.
func startJanitor() {
	if janitorActive {
		return
	}

	cache.janitorCtx, cache.janitorCancel = context.WithCancel(
		context.Background(),
	)
	cache.janitor = time.NewTimer(cache.interval)
	janitorActive = true

	go func() {
		defer func() {
			janitorActive = false

			if cache.janitor != nil {
				cache.janitor.Stop()
			}
		}()

		for {
			select {
			case <-cache.janitorCtx.Done():
				return
			case <-cache.janitor.C:
				now := time.Now().UnixNano()
				syncops.QueueSyncMapDeleteFuncExpires(syncops.MapTypeString, func(d int64) bool {
					return d != 0 && now >= d
				})
				syncops.QueueSyncMapDeleteFuncExpires(
					syncops.MapTypeThreeString,
					func(d int64) bool {
						return d != 0 && now >= d
					},
				)
				syncops.QueueSyncMapDeleteFuncExpires(syncops.MapTypeTwoInt, func(d int64) bool {
					return d != 0 && now >= d
				})
				syncops.QueueSyncMapDeleteFuncExpires(syncops.MapTypeTwoString, func(d int64) bool {
					return d != 0 && now >= d
				})

				select {
				case <-cache.janitorCtx.Done():
					return
				default:
					cache.janitor.Reset(cache.interval)
				}
			}
		}
	}()
}

// getexpireskey returns the expiration time in nanoseconds for the given cache key
// and duration. Special cases like 0 duration or predefined keys will not expire.
// func getexpireskey(duration time.Duration) int64 {
// 	return time.Now().Add(duration).UnixNano()
// }

// startCacheWriter starts the single cache writer goroutine to serialize cache operations
// Old cache writer functions removed - functionality moved to syncops package

// NewCache creates a new cache instance with proper initialization.
// cleaningInterval specifies the interval to clean up expired cache entries.
// extension specifies the default expiration duration to use for cached items.
// It initializes the cache and starts a goroutine to clean up expired entries
// based on the cleaningInterval. Uses sync.Once to ensure single initialization.
func NewCache(cleaningInterval, extension time.Duration) {
	initOnce.Do(func() {
		if cleaningInterval >= 1 {
			cache.interval = cleaningInterval

			startJanitor() // for cache items
		}

		// Initialize Ristretto caches for regex and SQL statements
		var err error

		// Regex cache. Entries are stored with cost 1, so MaxCost is an entry
		// count (~10M) - effectively unbounded; eviction never triggers in practice.
		cache.ristrettoRegex, err = ristretto.NewCache(&ristretto.Config[string, *regexp.Regexp]{
			NumCounters: 100_000,  // 10x expected items
			MaxCost:     10 << 20, // 10MB
			BufferItems: 64,
			Metrics:     false,
			OnEvict: func(_ *ristretto.Item[*regexp.Regexp]) {
				// Regex patterns don't need cleanup
			},
		})
		if err != nil {
			logger.Logtype("error", 1).Err(err).Msg("Failed to initialize Ristretto regex cache")
		}

		// SQL statement cache. Entries are stored with cost 1, so MaxCost is an
		// entry count (~10M) - effectively unbounded; eviction never triggers in
		// practice (which also avoids closing a statement while it is in use).
		cache.ristrettoStmt, err = ristretto.NewCache(&ristretto.Config[string, *sqlx.Stmt]{
			NumCounters: 100_000,
			MaxCost:     10 << 20,
			BufferItems: 64,
			Metrics:     false,
			OnEvict: func(item *ristretto.Item[*sqlx.Stmt]) {
				// Close SQL statement on eviction
				if item.Value != nil {
					item.Value.Close()
				}
			},
		})
		if err != nil {
			logger.Logtype("error", 1).Err(err).Msg("Failed to initialize Ristretto stmt cache")
		}

		// Initialize hybrid index SyncMaps
		cache.indexThreeStringByStr3 = syncops.NewSyncMap[map[string]*syncops.DbstaticThreeStringTwoInt](
			20,
		)
		cache.indexThreeStringByNum2 = syncops.NewSyncMap[map[uint]*syncops.DbstaticThreeStringTwoInt](
			20,
		)
		cache.indexTwoIntComposite = syncops.NewSyncMap[map[string]*syncops.DbstaticOneStringTwoInt](
			20,
		)
		cache.indexTwoStringByStr1 = syncops.NewSyncMap[map[string][]*syncops.DbstaticTwoStringOneInt](
			20,
		)
		cache.indexStringSet = syncops.NewSyncMap[map[string]struct{}](20)

		// Initialize syncops and register all SyncMaps
		syncops.InitSyncOps()
		syncops.RegisterSyncMap(syncops.MapTypeString, cache.itemsstring)
		syncops.RegisterSyncMap(syncops.MapTypeTwoInt, cache.itemstwoint)
		syncops.RegisterSyncMap(syncops.MapTypeThreeString, cache.itemsthreestring)
		syncops.RegisterSyncMap(syncops.MapTypeTwoString, cache.itemstwostring)

		globalCache = &globalcache{
			defaultextension: extension,
		}
	})
}

// SetStaticRegexp sets a static regular expression in the global cache.
// The regular expression can be used for fast matching without compiling it every time.
// This function pre-compiles regex patterns and stores them in the global cache to avoid
// the overhead of compilation during media processing. Used extensively for parsing
// movie/series titles, quality detection, and file pattern matching.
func SetStaticRegexp(key string) {
	globalCache.setStaticRegexp(key)
}

// GetCachedRegexp returns the compiled regexp for the given pattern,
// compiling and caching it permanently on first call.
func GetCachedRegexp(pattern string) *regexp.Regexp {
	return globalCache.setRegexp(pattern, 0)
}

// StopCache gracefully shuts down the cache janitor goroutine and cleans up resources.
// This function should be called during application shutdown to prevent goroutine leaks.
func StopCache() {
	if cache.janitorCancel != nil {
		cache.janitorCancel()
	}

	if cache.janitor != nil {
		cache.janitor.Stop()
	}

	// Close Ristretto caches
	if cache.ristrettoRegex != nil {
		cache.ristrettoRegex.Close()
	}

	if cache.ristrettoStmt != nil {
		cache.ristrettoStmt.Close()
	}

	// Shutdown syncops manager
	syncops.Shutdown()
}

// DeleteCacheEntry removes a specific key from all cache types.
// This function is thread-safe and uses the syncops system.
func DeleteCacheEntry(key string) {
	// Delete from all SyncMaps
	syncops.QueueSyncMapDelete(syncops.MapTypeString, key)
	syncops.QueueSyncMapDelete(syncops.MapTypeTwoInt, key)
	syncops.QueueSyncMapDelete(syncops.MapTypeThreeString, key)
	syncops.QueueSyncMapDelete(syncops.MapTypeTwoString, key)

	// Delete from all index maps
	cache.indexThreeStringByStr3.Delete(key)
	cache.indexThreeStringByNum2.Delete(key)
	cache.indexTwoStringByStr1.Delete(key)
	cache.indexStringSet.Delete(key)
	cache.indexTwoIntComposite.Delete(key)

	if cache.ristrettoStmt != nil {
		cache.ristrettoStmt.Del(key)
	}
}

// ClearCacheType clears all entries from a specific cache type.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func ClearCacheType(cacheType string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	switch cacheType {
	case logger.CacheMovie, logger.CacheSeries:
		// Clear media list caches
		syncops.QueueSyncMapDeleteFunc(syncops.MapTypeString, func(_ []string) bool {
			return true // Clear all entries
		})
		cache.indexStringSet.DeleteFunc(func(_ map[string]struct{}) bool { return true })

	case logger.CacheDBMovie, logger.CacheDBSeries, logger.CacheDBSeriesAlt:
		// Clear database caches
		syncops.QueueSyncMapDeleteFunc(
			syncops.MapTypeThreeString,
			func(_ []syncops.DbstaticThreeStringTwoInt) bool {
				return true // Clear all entries
			},
		)
		cache.indexThreeStringByStr3.DeleteFunc(
			func(_ map[string]*syncops.DbstaticThreeStringTwoInt) bool { return true },
		)
		cache.indexThreeStringByNum2.DeleteFunc(
			func(_ map[uint]*syncops.DbstaticThreeStringTwoInt) bool { return true },
		)

	case logger.CacheTitlesMovie:
		// Clear title caches
		syncops.QueueSyncMapDeleteFunc(
			syncops.MapTypeTwoString,
			func(_ []syncops.DbstaticTwoStringOneInt) bool {
				return true // Clear all entries
			},
		)
		cache.indexTwoStringByStr1.DeleteFunc(
			func(_ map[string][]*syncops.DbstaticTwoStringOneInt) bool { return true },
		)

	case logger.CacheUnmatchedMovie, logger.CacheUnmatchedSeries,
		logger.CacheFilesMovie, logger.CacheFilesSeries,
		logger.CacheHistoryURLMovie, logger.CacheHistoryTitleMovie,
		logger.CacheHistoryURLSeries, logger.CacheHistoryTitleSeries:
		// Clear string-based caches
		syncops.QueueSyncMapDeleteFunc(syncops.MapTypeString, func(_ []string) bool {
			return true // Clear all entries
		})
		cache.indexStringSet.DeleteFunc(func(_ map[string]struct{}) bool { return true })
	}
}

// SafeGetCacheString safely retrieves a string array from cache with proper locking.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeGetCacheString(key string) []string {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.itemsstring.GetVal(key)
}

// SafeGetCacheTwoString safely retrieves a syncops.DbstaticTwoStringOneInt array from cache with proper locking.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeGetCacheTwoString(key string) []syncops.DbstaticTwoStringOneInt {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.itemstwostring.GetVal(key)
}

// SafeGetCacheThreeString safely retrieves a syncops.DbstaticThreeStringTwoInt array from cache with proper locking.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeGetCacheThreeString(key string) []syncops.DbstaticThreeStringTwoInt {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.itemsthreestring.GetVal(key)
}

// SafeGetCacheTwoInt safely retrieves a syncops.DbstaticOneStringTwoInt array from cache with proper locking.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeGetCacheTwoInt(key string) []syncops.DbstaticOneStringTwoInt {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.itemstwoint.GetVal(key)
}

// SafeCheckCache safely checks if a key exists in any cache type.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeCheckCache(key string) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return cache.itemsstring.Check(key) ||
		cache.itemstwostring.Check(key) ||
		cache.itemsthreestring.Check(key) ||
		cache.itemstwoint.Check(key)
}

// SafeRemoveFromCacheString safely removes a specific value from a string cache array.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func SafeRemoveFromCacheString(key, value string) {
	// This function is now thread-safe using atomic slice operations.
	syncops.QueueAtomicRemoveString(syncops.MapTypeString, key, value)
}
