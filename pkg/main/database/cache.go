package database

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/dgraph-io/ristretto"
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
	itemsxstmt       *syncops.SyncMap[*sqlx.Stmt]     // DEPRECATED: Use ristrettoStmt
	itemsregex       *syncops.SyncMap[*regexp.Regexp] // DEPRECATED: Use ristrettoRegex

	// NEW: Ristretto caches for high-performance lookups
	ristrettoStmt  *ristretto.Cache
	ristrettoRegex *ristretto.Cache

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
)

var (
	cache = tcache{
		itemsstring:      syncops.NewSyncMap[[]string](20),
		itemstwoint:      syncops.NewSyncMap[[]syncops.DbstaticOneStringTwoInt](20),
		itemsthreestring: syncops.NewSyncMap[[]syncops.DbstaticThreeStringTwoInt](20),
		itemstwostring:   syncops.NewSyncMap[[]syncops.DbstaticTwoStringOneInt](20),
		itemsxstmt:       syncops.NewSyncMap[*sqlx.Stmt](1000),
		itemsregex:       syncops.NewSyncMap[*regexp.Regexp](1000),
		interval:         10 * time.Minute, // Set default interval to 10 minutes
	}
	globalCache   *globalcache
	initOnce      sync.Once // To make sure that the cache is only initialized once
	janitorActive bool
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
func (c *globalcache) addStaticXStmt(key string, imdb bool) {
	// if cache.ristrettoStmt == nil {
	// 	// Fallback to old implementation
	// 	if !cache.itemsxstmt.Check(key) {
	// 		stmt := preparestmt(imdb, key)
	// 		syncops.QueueSyncMapAdd(syncops.MapTypeXStmt, key, stmt, c.getexpiressql(true), imdb, time.Now().UnixNano())
	// 	}
	// 	return
	// }

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
// Falls back to old implementation if Ristretto not initialized.
func (c *globalcache) getXStmt(key string, imdb bool) *sqlx.Stmt {
	// if cache.ristrettoStmt == nil {
	// 	// Fallback to old implementation
	// 	return c.getXStmtOld(key, imdb)
	// }

	// Try Ristretto cache first
	if value, found := cache.ristrettoStmt.Get(key); found {
		if stmt, ok := value.(*sqlx.Stmt); ok {
			return stmt
		}
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

// getXStmtOld is the old implementation using syncops - kept as fallback
// func (c *globalcache) getXStmtOld(key string, imdb bool) *sqlx.Stmt {
// 	if cache.itemsxstmt.Check(key) {
// 		expires := cache.itemsxstmt.GetExpire(key)
// 		if config.GetSettingsGeneral().CacheAutoExtend ||
// 			expires != 0 && (time.Now().UnixNano() > expires) {
// 			syncops.QueueSyncMapUpdateExpire(syncops.MapTypeXStmt, key, c.getexpiressql(false))
// 		}
// 		return cache.itemsxstmt.GetVal(key)
// 	}
// 	stmt := preparestmt(imdb, key)
// 	syncops.QueueSyncMapAdd(syncops.MapTypeXStmt, key, stmt, c.getexpiressql(false), imdb, time.Now().UnixNano())
// 	return stmt
// }

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
func (c *globalcache) setStaticRegexp(key string) {
	// if cache.ristrettoRegex == nil {
	// 	// Fallback to old implementation if Ristretto not initialized
	// 	if !cache.itemsregex.Check(key) {
	// 		syncops.QueueSyncMapAdd(syncops.MapTypeRegex, key, getregexP(key), 0, false, time.Now().UnixNano())
	// 	}
	// 	return
	// }

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
func (c *globalcache) setRegexp(key string, duration time.Duration) *regexp.Regexp {
	// if cache.ristrettoRegex == nil {
	// 	// Fallback to old implementation if Ristretto not initialized
	// 	return c.setRegexpOld(key, duration)
	// }

	// Try Ristretto cache first
	if value, found := cache.ristrettoRegex.Get(key); found {
		if rgx, ok := value.(*regexp.Regexp); ok {
			return rgx
		}
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

// setRegexpOld is the old implementation using syncops - kept as fallback
// func (c *globalcache) setRegexpOld(key string, duration time.Duration) *regexp.Regexp {
// 	if !cache.itemsregex.Check(key) {
// 		rgx := getregexP(key)
// 		expires := getexpireskey(duration)
// 		if duration == 0 {
// 			c.mu.RLock()
// 			defaultExt := c.defaultextension
// 			c.mu.RUnlock()
// 			expires = getexpireskey(defaultExt)
// 		}
// 		syncops.QueueSyncMapAdd(syncops.MapTypeRegex, key, rgx, expires, false, time.Now().UnixNano())
// 		return rgx
// 	}
// 	expires := cache.itemsregex.GetExpire(key)
// 	if config.GetSettingsGeneral().CacheAutoExtend || expires != 0 && (time.Now().UnixNano() > expires) {
// 		if duration != 0 && key != "RegexSeriesIdentifier" && key != "RegexSeriesTitle" {
// 			c.mu.RLock()
// 			defaultExt := c.defaultextension
// 			c.mu.RUnlock()
// 			syncops.QueueSyncMapUpdateExpire(syncops.MapTypeRegex, key, getexpireskey(defaultExt))
// 		}
// 	}
// 	return cache.itemsregex.GetVal(key)
// }

// InitCache initializes the global cache by creating a new Cache instance
// with the provided expiration times and logger. It is called on startup
// to initialize the cache before it is used.
func InitCache() {
	NewCache(1*time.Hour, time.Hour*time.Duration(config.GetSettingsGeneral().CacheDuration))
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
	syncops.QueueSyncMapDeleteFunc(syncops.MapTypeXStmt, func(item *sqlx.Stmt) bool {
		item.Close()
		return true
	})
	syncops.QueueSyncMapDeleteFunc(syncops.MapTypeRegex, func(_ *regexp.Regexp) bool {
		return true
	})
}

// AppendCache appends the given value v to the cached array for the given key a.
// This function is now thread-safe using atomic slice operations.
func AppendCache(a string, v string) {
	syncops.QueueAtomicAppendString(syncops.MapTypeString, a, v)
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
	}
}

// AppendCacheMap appends a value to the cache map for the given query string.
// If useseries is true, it appends to the logger.Mapstringsseries map, otherwise it appends to the logger.Mapstringsmovies map.
// The value is appended using the AppendCache function.
func AppendCacheMap(useseries bool, query string, v string) {
	AppendCache(logger.GetStringsMap(useseries, query), v)
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
func SlicesCacheContainsI(useseries bool, query string, w *string) bool {
	v := *w
	for _, a := range GetCachedStringArr(logger.GetStringsMap(useseries, query), false, true) {
		if v == a || strings.EqualFold(v, a) {
			return true
		}
	}

	return false
}

// SlicesCacheContains checks if the cached string array identified by s contains
// the value v. It iterates over the array and returns true if a match is found.
func SlicesCacheContains(useseries bool, query, v string) bool {
	return slices.Contains(
		GetCachedStringArr(logger.GetStringsMap(useseries, query), false, true),
		v,
	)
}

// SlicesCacheContainsDelete removes an element matching v from the cached string
// array identified by s. This function is now thread-safe using atomic slice operations.
func SlicesCacheContainsDelete(s, v string) {
	syncops.QueueAtomicRemoveString(syncops.MapTypeString, s, v)
}

// CacheOneStringTwoIntIndexFunc looks up the cached one string two int array
// identified by s and calls the passed in function f on each element.
// Returns true if f returns true for any element.
func CacheOneStringTwoIntIndexFunc(s string, f func(*syncops.DbstaticOneStringTwoInt) bool) bool {
	a := GetCachedTwoIntArr(s, false, true)
	for idx := range a {
		if f(&a[idx]) {
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
	for _, a := range GetCachedTwoIntArr(s, false, true) {
		if a.Num1 == id && strings.EqualFold(a.Str, listname) {
			return a.Num2
		}
	}

	return 0
}

// CacheOneStringTwoIntIndexFuncStr looks up the cached one string, two int array
// identified by s and returns the string value for the entry where the
// second int matches the passed in uint i. It stores the returned string in
// listname. If no match is found, it sets listname to an empty string.
func CacheOneStringTwoIntIndexFuncStr(useseries bool, query string, i uint) string {
	for _, a := range GetCachedTwoIntArr(logger.GetStringsMap(useseries, query), false, true) {
		if a.Num2 == i {
			return a.Str
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
	for _, a := range GetCachedThreeStringArr(s, false, true) {
		if a.Str3 == t || strings.EqualFold(a.Str3, t) {
			return a.Num2
		}
	}

	return 0
}

// CacheThreeStringIntIndexFuncGetYear looks up the cached three string, two int array
// identified by s and returns the first int value for the entry where the second int
// matches the int str. Returns 0 if no match found.
func CacheThreeStringIntIndexFuncGetYear(s string, i uint) uint16 {
	for _, a := range GetCachedThreeStringArr(s, false, true) {
		if a.Num2 == i {
			return uint16(a.Num1)
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
//   - useseries: Determines whether to use series-specific or movie-specific data
//   - force: If true, forces a cache refresh regardless of existing cache state
//   - maptypestr: String identifier for the map type
//   - mapcountsql: SQL query to get the count of items in the cache
//   - mapsql: SQL query to retrieve the cache items
//   - cachevar: Synchronized map to store the cache items
func refreshCacheDBInternal[t comparable](
	useseries bool,
	force bool,
	maptypestr string,
	mapcountsql string,
	mapsql string,
	cachevar *syncops.SyncMap[[]t],
) {
	mapvar := logger.GetStringsMap(useseries, maptypestr)
	item := getCachedArrayDirect(cachevar, mapvar, true, false)

	count := Getdatarow[uint](
		false,
		logger.GetStringsMap(useseries, mapcountsql),
		&config.GetSettingsGeneral().CacheDuration,
	)
	if !force && len(item) == int(count) {
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
		logger.GetStringsMap(useseries, mapsql),
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
// It will refresh the caches based on the useseries parameter:
// - if useseries is false, it will refresh the movie caches
// - if useseries is true, it will refresh the series caches
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheDB(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheDBMedia,
		logger.DBCountDBMedia,
		logger.DBCacheDBMedia,
		cache.itemsthreestring,
	)
}

// RefreshMediaCacheList refreshes the media caches for movies and series.
// It will refresh the caches based on the useseries parameter:
// - if useseries is false, it will refresh the movie caches
// - if useseries is true, it will refresh the series caches
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheList(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheMedia,
		logger.DBCountMedia,
		logger.DBCacheMedia,
		cache.itemstwoint,
	)
}

// Refreshhistorycachetitle refreshes the cached history title arrays for movies or series.
// The useseries parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshhistorycachetitle(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheHistoryTitle,
		logger.DBCountHistoriesTitle,
		logger.DBHistoriesTitle,
		cache.itemsstring,
	)
}

// Refreshhistorycacheurl refreshes the cached history URL arrays for movies or series.
// The useseries parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshhistorycacheurl(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseHistoryCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheHistoryURL,
		logger.DBCountHistoriesURL,
		logger.DBHistoriesURL,
		cache.itemsstring,
	)
}

// RefreshMediaCacheTitles refreshes the cached media title arrays for movies or series.
// The useseries parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func RefreshMediaCacheTitles(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheMediaTitles,
		logger.DBCountDBTitles,
		logger.DBCacheDBTitles,
		cache.itemstwostring,
	)
}

// Refreshfilescached refreshes the cached file location arrays for movies or series.
// The useseries parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshfilescached(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseFileCache {
		return
	}

	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheFiles,
		logger.DBCountFiles,
		logger.DBCacheFiles,
		cache.itemsstring,
	)
}

// Refreshunmatchedcached refreshes the cached string array of unmatched files for movies or series.
// The useseries parameter determines if it refreshes the cache for series or movies.
// Operations are queued to the single cache writer to prevent concurrent access issues.
func Refreshunmatchedcached(useseries bool, force bool) {
	if !config.GetSettingsGeneral().UseFileCache {
		return
	}

	ExecNMap(useseries, "DBRemoveUnmatched", &config.GetSettingsGeneral().CacheDuration2)
	refreshCacheDBInternal(
		useseries,
		force,
		logger.CacheUnmatched,
		logger.DBCountUnmatched,
		logger.DBCacheUnmatched,
		cache.itemsstring,
	)
}

// RefreshCached refreshes the cached data for the specified key. It calls the appropriate
// refresh function based on the key. Uses indexed versions when UseIndexedCache is enabled.
func RefreshCached(key string, force bool) {
	// Use indexed refresh functions if enabled for better performance
	useIndexed := config.GetSettingsGeneral().UseIndexedCache

	switch key {
	case logger.CacheMovie:
		RefreshMediaCacheList(false, force)
	case logger.CacheSeries:
		RefreshMediaCacheList(true, force)
	case logger.CacheDBMovie:
		if useIndexed {
			RefreshMediaCacheDBIndexed(false, force)
		} else {
			RefreshMediaCacheDB(false, force)
		}

	case logger.CacheDBSeriesAlt:
		if useIndexed {
			RefreshMediaCacheTitlesIndexed(true, force)
		} else {
			RefreshMediaCacheTitles(true, force)
		}

	case logger.CacheDBSeries:
		if useIndexed {
			RefreshMediaCacheDBIndexed(true, force)
		} else {
			RefreshMediaCacheDB(true, force)
		}

	case logger.CacheTitlesMovie:
		if useIndexed {
			RefreshMediaCacheTitlesIndexed(false, force)
		} else {
			RefreshMediaCacheTitles(false, force)
		}

	case logger.CacheUnmatchedMovie:
		Refreshunmatchedcached(false, force)
	case logger.CacheUnmatchedSeries:
		Refreshunmatchedcached(true, force)
	case logger.CacheFilesMovie:
		Refreshfilescached(false, force)
	case logger.CacheFilesSeries:
		Refreshfilescached(true, force)
	case logger.CacheHistoryURLMovie:
		if useIndexed {
			RefreshhistorycacheurlIndexed(false, force)
		} else {
			Refreshhistorycacheurl(false, force)
		}

	case logger.CacheHistoryTitleMovie:
		if useIndexed {
			RefreshhistorycachetitleIndexed(false, force)
		} else {
			Refreshhistorycachetitle(false, force)
		}

	case logger.CacheHistoryURLSeries:
		if useIndexed {
			RefreshhistorycacheurlIndexed(true, force)
		} else {
			Refreshhistorycacheurl(true, force)
		}

	case logger.CacheHistoryTitleSeries:
		if useIndexed {
			RefreshhistorycachetitleIndexed(true, force)
		} else {
			Refreshhistorycachetitle(true, force)
		}
	}
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
			logger.Logtype("error", 1).Str("key", s).Msg("Unable to determine MapType for SyncMap in storeMapType")
		}

		return
	}
	// Use syncops for Add operation as well
	mapType := getMapTypeForSyncMap(c)
	if mapType != "" {
		syncops.QueueSyncMapAdd(mapType, s, val, expires, false, lastscan)
	} else {
		// Log error if map type cannot be determined
		logger.Logtype("error", 1).Str("key", s).Msg("Unable to determine MapType for SyncMap in storeMapType Add")
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
	case cache.itemsxstmt:
		return syncops.MapTypeXStmt
	case cache.itemsregex:
		return syncops.MapTypeRegex
	default:
		return ""
	}
}

// CheckcachedMovieTitleHistory checks if the given movie title exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the title exists, false
// otherwise.
func CheckcachedTitleHistory(useseries bool, file *string) bool {
	if config.GetSettingsGeneral().UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryTitle, file)
	}

	return Getdatarow[uint](
		false,
		logger.GetStringsMap(useseries, logger.DBCountHistoriesByTitle),
		file,
	) >= 1
}

// CheckcachedMovieUrlHistory checks if the given movie URL exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the URL exists, false
// otherwise.
func CheckcachedURLHistory(useseries bool, file *string) bool {
	if config.GetSettingsGeneral().UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryURL, file)
	}

	return Getdatarow[uint](
		false,
		logger.GetStringsMap(useseries, logger.DBCountHistoriesByURL),
		file,
	) >= 1
}

// InvalidateImdbStmt iterates over the cache.items sync.Map and deletes any
// cached items that have the imdb field set to true. This is likely used to
// invalidate any cached IMDB-related data when needed.
func InvalidateImdbStmt() {
	syncops.QueueSyncMapDeleteFuncImdbVal(syncops.MapTypeXStmt, func(b bool) bool {
		return b
	}, func(s *sqlx.Stmt) {
		s.Close()
	})
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

	cache.janitorCtx, cache.janitorCancel = context.WithCancel(context.Background())
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

				syncops.QueueSyncMapDeleteFuncExpiresVal(syncops.MapTypeXStmt, func(d int64) bool {
					return d != 0 && now >= d
				}, func(s *sqlx.Stmt) {
					s.Close()
				})
				syncops.QueueSyncMapDeleteFuncExpires(syncops.MapTypeRegex, func(d int64) bool {
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

		// Regex cache: ~1000 patterns, 10MB max
		cache.ristrettoRegex, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: 100_000,  // 10x expected items
			MaxCost:     10 << 20, // 10MB
			BufferItems: 64,
			Metrics:     false,
			OnEvict: func(item *ristretto.Item) {
				// Regex patterns don't need cleanup
			},
		})
		if err != nil {
			logger.Logtype("error", 1).Err(err).Msg("Failed to initialize Ristretto regex cache")
		}

		// SQL statement cache: ~1000 statements, 10MB max
		cache.ristrettoStmt, err = ristretto.NewCache(&ristretto.Config{
			NumCounters: 100_000,
			MaxCost:     10 << 20,
			BufferItems: 64,
			Metrics:     false,
			OnEvict: func(item *ristretto.Item) {
				// Close SQL statement on eviction
				if stmt, ok := item.Value.(*sqlx.Stmt); ok && stmt != nil {
					stmt.Close()
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
		syncops.RegisterSyncMap(syncops.MapTypeXStmt, cache.itemsxstmt)
		syncops.RegisterSyncMap(syncops.MapTypeRegex, cache.itemsregex)

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
	syncops.QueueSyncMapDelete(syncops.MapTypeXStmt, key)
	syncops.QueueSyncMapDelete(syncops.MapTypeRegex, key)
}

// ClearCacheType clears all entries from a specific cache type.
// This function is thread-safe and can be called concurrently from multiple goroutines.
func ClearCacheType(cacheType string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	switch cacheType {
	case logger.CacheMovie, logger.CacheSeries:
		// Clear media list caches
		syncops.QueueSyncMapDeleteFunc(syncops.MapTypeString, func(value []string) bool {
			return true // Clear all entries
		})

	case logger.CacheDBMovie, logger.CacheDBSeries, logger.CacheDBSeriesAlt:
		// Clear database caches
		syncops.QueueSyncMapDeleteFunc(
			syncops.MapTypeThreeString,
			func(value []syncops.DbstaticThreeStringTwoInt) bool {
				return true // Clear all entries
			},
		)

	case logger.CacheTitlesMovie:
		// Clear title caches
		syncops.QueueSyncMapDeleteFunc(
			syncops.MapTypeTwoString,
			func(value []syncops.DbstaticTwoStringOneInt) bool {
				return true // Clear all entries
			},
		)

	case logger.CacheUnmatchedMovie, logger.CacheUnmatchedSeries,
		logger.CacheFilesMovie, logger.CacheFilesSeries,
		logger.CacheHistoryURLMovie, logger.CacheHistoryTitleMovie,
		logger.CacheHistoryURLSeries, logger.CacheHistoryTitleSeries:
		// Clear string-based caches
		syncops.QueueSyncMapDeleteFunc(syncops.MapTypeString, func(value []string) bool {
			return true // Clear all entries
		})
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
