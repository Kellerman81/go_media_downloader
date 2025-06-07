package database

import (
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/jmoiron/sqlx"
)

// globalcache is a struct that stores configuration values for the cache
// including the default cache expiration time and a logger instance.
type globalcache struct {
	// defaultextension is the default cache expiration duration
	defaultextension time.Duration
}

type tcache struct {
	interval         time.Duration
	itemsstring      *logger.SyncMap[[]string]
	itemstwoint      *logger.SyncMap[[]DbstaticOneStringTwoInt]
	itemsthreestring *logger.SyncMap[[]DbstaticThreeStringTwoInt]
	itemstwostring   *logger.SyncMap[[]DbstaticTwoStringOneInt]
	itemsxstmt       *logger.SyncMap[sqlx.Stmt]      // sync.Map
	itemsxstmtP      *logger.SyncMap[*sqlx.Stmt]     // sync.Map
	itemsregex       *logger.SyncMap[regexp.Regexp]  // sync.Map
	itemsregexP      *logger.SyncMap[*regexp.Regexp] // sync.Map
	janitor          *time.Timer
}

var (
	strquery = "query"
	strcache = "cache"
)

// cache is a struct that stores the cache items map,
// a janitor timer, and an interval duration
// items is a sync.Map that stores cached items
// janitor is a timer that periodically cleans up expired cache items
// interval is the duration between janitor cleanups.
var (
	cache = tcache{
		itemsstring:      logger.NewSyncMap[[]string](20),
		itemstwoint:      logger.NewSyncMap[[]DbstaticOneStringTwoInt](20),
		itemsthreestring: logger.NewSyncMap[[]DbstaticThreeStringTwoInt](20),
		itemstwostring:   logger.NewSyncMap[[]DbstaticTwoStringOneInt](20),
		itemsxstmt:       logger.NewSyncMap[sqlx.Stmt](1000),
		itemsxstmtP:      logger.NewSyncMap[*sqlx.Stmt](1000),
		itemsregex:       logger.NewSyncMap[regexp.Regexp](1000),
		itemsregexP:      logger.NewSyncMap[*regexp.Regexp](1000),
		interval:         10 * time.Minute, // Set default interval to 5 minutes
	}
	globalCache *globalcache
	mu          = sync.Mutex{} // To make sure that the cache is only initialized once
)

// getexpiressql returns the expiry time in nanoseconds for cached statements.
// It checks the defaultextension field, and if greater than 0, sets the expiry to that duration from the current time.
// Otherwise it returns 0 indicating no expiry.
func (c *globalcache) getexpiressql() int64 {
	if c.defaultextension > 0 {
		return time.Now().Add(c.defaultextension).UnixNano()
	}
	return 0
}

// addStaticXStmt adds a prepared SQL statement to the cache with the given key and database connection.
// If the statement already exists in the cache, it returns without doing anything.
// Otherwise, it prepares the statement using the provided database connection and adds it to the cache.
// The function takes a key string and a boolean indicating whether to use the IMDB database connection.
// It returns nothing.
// Warning: Only use this function outside of goroutines - so only from the main program.
func (c *globalcache) addStaticXStmt(key string, imdb bool) {
	if cache.itemsxstmtP.Check(key) {
		return
	}
	sq, err := Getdb(imdb).Preparex(key)
	if err != nil || sq == nil {
		return
	}
	cache.itemsxstmtP.Add(key, sq, 0, imdb, 0)
}

// getXStmt retrieves a cached SQL statement with the given key and database connection.
// If the statement is not found in the cache, it prepares the statement using the provided
// database connection and adds it to the cache. If the cache auto-extend feature is enabled,
// it will extend the expiration time of the cached statement.
// The function takes a key string, a boolean indicating whether to use the IMDB database
// connection, and an optional slice of int64 values representing the expiration time in
// nanoseconds. It returns the cached SQL statement.
func (c *globalcache) getXStmt(key string, imdb bool) *sqlx.Stmt {
	if cache.itemsxstmtP.Check(
		key,
	) { // only check and get not add to static cache - function is called from goroutines
		return cache.itemsxstmtP.GetVal(key)
	}
	if !cache.itemsxstmt.Check(key) {
		cache.itemsxstmt.Add(key, preparestmt(imdb, key), c.getexpiressql(), imdb, 0)
		return cache.itemsxstmt.GetValP(key)
	}
	// val := cache.itemsxstmt.GetVal(key)
	// if val.Stmt == nil {
	// 	cache.itemsxstmt.UpdateVal(key, preparestmt(imdb, key))
	// 	cache.itemsxstmt.UpdateExpire(key, c.getexpiressql())
	// 	return cache.itemsxstmt.GetValP(key)
	// }
	expires := cache.itemsxstmt.GetExpire(key)
	if config.SettingsGeneral.CacheAutoExtend || expires != 0 && (time.Now().UnixNano() > expires) {
		cache.itemsxstmt.UpdateExpire(key, c.getexpiressql())
	}
	return cache.itemsxstmt.GetValP(key)
}

// preparestmt prepares a SQL statement using the provided database connection and SQL query key.
// If an error occurs or the prepared statement is nil, it returns an empty sqlx.Stmt.
// Otherwise, it returns the prepared statement.
func preparestmt(imdb bool, key string) sqlx.Stmt {
	sq, err := Getdb(imdb).Preparex(key)
	if err != nil || sq == nil {
		return sqlx.Stmt{}
	}
	return *sq
}

// setStaticRegexp sets a cached regular expression with the given key. If the cached regular expression
// does not exist, it creates a new one and adds it to the cache. This function is used to cache regular
// expressions that are used statically throughout the application.
// Warning: Only use this function outside of goroutines - so only from the main program.
func (c *globalcache) setStaticRegexp(key string) {
	if !cache.itemsregexP.Check(key) {
		cache.itemsregexP.Add(key, getregexP(key), 0, false, 0)
		return // cache.itemsregex.GetVal(key)
	}
}

// setRegexp sets a cached regular expression with the given key. If the cached regular expression
// does not exist, it creates a new one and adds it to the cache. If the cached regular expression
// already exists, it checks if the expiration time has passed and updates the expiration time if
// necessary.
// The function takes a key string and a duration time.Duration. If the duration is 0, it uses the
// default extension time. The function returns the cached regular expression.
func (c *globalcache) setRegexp(key string, duration time.Duration) *regexp.Regexp {
	if cache.itemsregexP.Check(
		key,
	) { // only check and get not add to static cache - function is called from goroutines
		return cache.itemsregexP.GetVal(key)
	}
	if !cache.itemsregex.Check(key) {
		if duration == 0 {
			duration = c.defaultextension
		}
		var expires int64
		if duration == 0 || key == "RegexSeriesIdentifier" || key == "RegexSeriesTitle" {
			expires = 0
		} else {
			expires = getexpireskey(duration)
		}
		cache.itemsregex.Add(key, getregex(key), expires, false, 0)
		return cache.itemsregex.GetValP(key)
	}
	expires := cache.itemsregex.GetExpire(key)
	if config.SettingsGeneral.CacheAutoExtend || expires != 0 && (time.Now().UnixNano() > expires) {
		if duration != 0 && key != "RegexSeriesIdentifier" && key != "RegexSeriesTitle" {
			cache.itemsregex.UpdateExpire(key, getexpireskey(c.defaultextension))
		}
	}
	return cache.itemsregex.GetValP(key)
}

// InitCache initializes the global cache by creating a new Cache instance
// with the provided expiration times and logger. It is called on startup
// to initialize the cache before it is used.
func InitCache() {
	NewCache(1*time.Hour, time.Hour*time.Duration(config.SettingsGeneral.CacheDuration))
}

// ClearCaches iterates over the cached string, three string two int, and two string int arrays, sets the Expire field on each cached array object to two hours in the past based on the config cache duration, and updates the cache with the expired array object. This effectively clears those cached arrays by expiring all entries.
func ClearCaches() {
	cache.itemsstring.DeleteFunc(func(item []string) bool {
		clear(item)
		item = nil
		return true
	})
	cache.itemsthreestring.DeleteFunc(func(item []DbstaticThreeStringTwoInt) bool {
		clear(item)
		item = nil
		return true
	})
	cache.itemstwoint.DeleteFunc(func(item []DbstaticOneStringTwoInt) bool {
		clear(item)
		item = nil
		return true
	})
	cache.itemstwostring.DeleteFunc(func(item []DbstaticTwoStringOneInt) bool {
		clear(item)
		item = nil
		return true
	})
	cache.itemsxstmt.DeleteFunc(func(item sqlx.Stmt) bool {
		item.Close()
		return true
	})
	cache.itemsregex.DeleteFunc(func(_ regexp.Regexp) bool {
		return true
	})
}

// AppendCache appends the given value v to the cached array for the given key a.
// If the cached array for the given key does not exist, this function does nothing.
// If the given value is already present in the cached array, this function does nothing.
func AppendCache(a string, v string) {
	if !cache.itemsstring.Check(a) {
		return
	}
	s := GetCachedStringArr(a, false, true)
	for idx := range s {
		if s[idx] == v {
			return
		}
	}
	cache.itemsstring.UpdateVal(a, append(s, v))
}

// AppendCacheThreeString appends the given DbstaticThreeStringTwoInt value v to the cached array
// for the given key a. If the cached array for the given key does not exist, this function
// does nothing. If the given value is already present in the cached array, this function
// does nothing.
func AppendCacheThreeString(a string, v DbstaticThreeStringTwoInt) {
	if !cache.itemsthreestring.Check(a) {
		return
	}
	s := GetCachedThreeStringArr(a, false, true)
	for idx := range s {
		if s[idx] == v {
			return
		}
	}
	cache.itemsthreestring.UpdateVal(a, append(s, v))
}

// AppendCacheTwoInt appends the given DbstaticOneStringTwoInt value v to the cached array
// for the given key a. If the cached array for the given key does not exist, this function
// does nothing. If the given value is already present in the cached array, this function
// does nothing.
func AppendCacheTwoInt(a string, v DbstaticOneStringTwoInt) {
	if !cache.itemstwoint.Check(a) {
		return
	}
	s := GetCachedTwoIntArr(a, false, true)
	for idx := range s {
		if s[idx] == v {
			return
		}
	}
	cache.itemstwoint.UpdateVal(a, append(s, v))
}

// AppendCacheTwoString appends the given DbstaticTwoStringOneInt value v to the cached array
// for the given key a. If the cached array for the given key does not exist, this function
// does nothing. If the given value is already present in the cached array, this function
// does nothing.
func AppendCacheTwoString(a string, v DbstaticTwoStringOneInt) {
	if !cache.itemstwostring.Check(a) {
		return
	}
	s := GetCachedTwoStringArr(a, false, true)
	for idx := range s {
		if s[idx] == v {
			return
		}
	}
	cache.itemstwostring.UpdateVal(a, append(s, v))
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
	for idx := range s {
		if s[idx] == v {
			return true
		}
	}
	return false
}

// ArrStructContainsString checks if the given slice s contains the string value v.
// It iterates over the slice and returns true if a match is found, false otherwise.
func ArrStructContainsString(s []string, v string) bool {
	for idx := range s {
		if s[idx] == v {
			return true
		}
	}
	return false
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
	for _, a := range GetCachedStringArr(logger.GetStringsMap(useseries, query), false, true) {
		if a == v {
			return true
		}
	}
	return false
}

// SlicesCacheContainsDelete removes an element matching v from the cached string
// array identified by s. It acquires a lock, defers unlocking, and iterates
// over the array to find a match. When found, it uses slices.Delete to remove
// the element while preserving order, updates the cache, and returns.
func SlicesCacheContainsDelete(s, v string) {
	if !cache.itemsstring.Check(s) {
		return
	}
	a := GetCachedStringArr(s, false, true)
	if !ArrStructContainsString(a, v) {
		return
	}
	cache.itemsstring.UpdateVal(s, slices.DeleteFunc(a, func(sl string) bool {
		return sl == v
	}))
}

// CacheOneStringTwoIntIndexFunc looks up the cached one string two int array
// identified by s and calls the passed in function f on each element.
// Returns true if f returns true for any element.
func CacheOneStringTwoIntIndexFunc(s string, f func(*DbstaticOneStringTwoInt) bool) bool {
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

// RefreshMediaCache refreshes the media caches for movies and series.
// It will refresh the caches based on the useseries parameter:
// - if useseries is false, it will refresh the movie caches
// - if useseries is true, it will refresh the series caches
// It locks access to the caches while refreshing to prevent concurrent access issues.
func RefreshMediaCacheDB(useseries bool, force bool) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheDBMedia)
	item := GetCachedThreeStringArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountDBMedia))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMapThreeString(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemsthreestring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany1String("debug", "refresh cache", strcache, mapvar)

	data := Getrows0[DbstaticThreeStringTwoInt](
		false,
		count+100,
		logger.GetStringsMap(useseries, logger.DBCacheDBMedia),
	)
	if len(data) > 0 {
		cache.storeMapThreeString(
			mapvar,
			data,
			time.Now().
				Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
				UnixNano(),
			time.Now().UnixNano(),
		)
	} else {
		logger.LogDynamicany1String("debug", "refresh cache empty", strcache, mapvar)
	}
}

// RefreshMediaCacheList refreshes the media caches for movies and series.
// It will refresh the caches based on the useseries parameter:
// - if useseries is false, it will refresh the movie caches
// - if useseries is true, it will refresh the series caches
// It locks access to the caches while refreshing to prevent concurrent access issues.
func RefreshMediaCacheList(useseries bool, force bool) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheMedia)
	item := GetCachedTwoIntArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountMedia))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMapTwoInt(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemstwoint.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany1String("debug", "refresh cache", strcache, mapvar)

	cache.storeMapTwoInt(
		mapvar,
		Getrows0[DbstaticOneStringTwoInt](
			false,
			count+100,
			logger.GetStringsMap(useseries, logger.DBCacheMedia),
		),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// Refreshhistorycachetitle refreshes the cached history title arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
func Refreshhistorycachetitle(useseries bool, force bool) {
	if !config.SettingsGeneral.UseHistoryCache {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheHistoryTitle)
	item := GetCachedStringArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountHistoriesTitle))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMap(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemsstring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany1String("debug", "refresh cache", strcache, mapvar)
	cache.storeMap(
		mapvar,
		Getrows0[string](
			false,
			count+100,
			logger.GetStringsMap(useseries, logger.DBHistoriesTitle),
		),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// Refreshhistorycacheurl refreshes the cached history URL arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
// The force parameter determines if the cache should be refreshed regardless of the last scan time.
func Refreshhistorycacheurl(useseries bool, force bool) {
	if !config.SettingsGeneral.UseHistoryCache {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheHistoryURL)
	item := GetCachedStringArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountHistoriesURL))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMap(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemsstring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany1String("debug", "refresh cache", strcache, mapvar)
	cache.storeMap(
		mapvar,
		Getrows0[string](false, count+100, logger.GetStringsMap(useseries, logger.DBHistoriesURL)),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// RefreshMediaCacheTitles refreshes the cached media title arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func RefreshMediaCacheTitles(useseries bool, force bool) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheMediaTitles)
	item := GetCachedTwoStringArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountDBTitles))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMapTwoString(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemstwostring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany3StrIntInt(
		"debug",
		"refresh media titles cache",
		strcache,
		mapvar,
		"oldcount",
		len(item),
		"newcount",
		int(count),
	)
	expires := cache.itemstwostring.GetExpire(mapvar)
	logger.Logtype("debug", 0).
		Time("Expires", time.Unix(0, expires)).
		Msg("refresh media titles cache expires")
	cache.storeMapTwoString(
		mapvar,
		Getrows0[DbstaticTwoStringOneInt](
			false,
			count+100,
			logger.GetStringsMap(useseries, logger.DBCacheDBTitles),
		),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// Refreshfilescached refreshes the cached file location arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func Refreshfilescached(useseries bool, force bool) {
	if !config.SettingsGeneral.UseFileCache {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	mapvar := logger.GetStringsMap(useseries, logger.CacheFiles)
	item := GetCachedStringArr(mapvar, true, false)
	count := Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountFiles))
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMap(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}

	lastscan := cache.itemsstring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany0("debug", "refresh files cache")
	cache.storeMap(
		mapvar,
		Getrows0[string](false, count+300, logger.GetStringsMap(useseries, logger.DBCacheFiles)),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// Refreshunmatchedcached refreshes the cached string array of unmatched files for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func Refreshunmatchedcached(useseries bool, force bool) {
	if !config.SettingsGeneral.UseFileCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// hours12 := logger.JoinStrings("-", strconv.Itoa(config.SettingsGeneral.CacheDuration), " hours")
	// hours24 := logger.JoinStrings("-", strconv.Itoa(2*config.SettingsGeneral.CacheDuration), " hours")
	hours24 := 2 * config.SettingsGeneral.CacheDuration
	ExecNMap(useseries, "DBRemoveUnmatched", &hours24)
	mapvar := logger.GetStringsMap(useseries, logger.CacheUnmatched)
	item := GetCachedStringArr(mapvar, true, false)
	count := Getdatarow1[uint](
		false,
		logger.GetStringsMap(useseries, logger.DBCountUnmatched),
		&config.SettingsGeneral.CacheDuration,
	)
	if !force && len(item) == int(count) {
		return
	}
	if count == 0 {
		if len(item) > 0 {
			cache.storeMap(
				mapvar,
				nil,
				time.Now().
					Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).
					UnixNano(),
				time.Now().UnixNano(),
			)
		}
		return
	}
	lastscan := cache.itemsstring.GetLastscan(mapvar)
	if !force && lastscan != 0 && lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
		return
	}
	logger.LogDynamicany0("debug", "refresh unmatched cache")
	cache.storeMap(
		mapvar,
		Getrows1[string](
			false,
			count+300,
			logger.GetStringsMap(useseries, logger.DBCacheUnmatched),
			&config.SettingsGeneral.CacheDuration,
		),
		time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration)*time.Hour).UnixNano(),
		time.Now().UnixNano(),
	)
}

// RefreshCached refreshes the cached data for the specified key. It calls the appropriate
// refresh function based on the key, such as RefreshMediaCache, RefreshMediaCacheTitles,
// Refreshunmatchedcached, Refreshfilescached, or Refreshhistorycache.
func RefreshCached(key string, force bool) {
	switch key {
	case logger.CacheMovie:
		RefreshMediaCacheList(false, force)
	case logger.CacheSeries:
		RefreshMediaCacheList(true, force)
	case logger.CacheDBMovie:
		RefreshMediaCacheDB(false, force)
	case logger.CacheDBSeriesAlt:
		RefreshMediaCacheTitles(true, force)
	case logger.CacheDBSeries:
		RefreshMediaCacheDB(true, force)
	case logger.CacheTitlesMovie:
		RefreshMediaCacheTitles(false, force)
	case logger.CacheUnmatchedMovie:
		Refreshunmatchedcached(false, force)
	case logger.CacheUnmatchedSeries:
		Refreshunmatchedcached(true, force)
	case logger.CacheFilesMovie:
		Refreshfilescached(false, force)
	case logger.CacheFilesSeries:
		Refreshfilescached(true, force)
	case logger.CacheHistoryURLMovie:
		Refreshhistorycacheurl(false, force)
	case logger.CacheHistoryTitleMovie:
		Refreshhistorycachetitle(false, force)
	case logger.CacheHistoryURLSeries:
		Refreshhistorycacheurl(true, force)
	case logger.CacheHistoryTitleSeries:
		Refreshhistorycachetitle(true, force)
	}
}

// GetCachedStringArr retrieves the cached array of strings associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedStringArr(key string, checkexpire bool, retry bool) []string {
	if cache.itemsstring.Check(key) {
		if checkexpire {
			if cache.itemsstring.CheckExpires(
				key,
				config.SettingsGeneral.CacheAutoExtend,
				config.SettingsGeneral.CacheDuration,
			) {
				return nil
			}
		}
	}
	return getrefresh(cache.itemsstring, key, retry)
}

// GetCachedTwoStringArr retrieves the cached array of DbstaticTwoStringOneInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedTwoStringArr(key string, checkexpire bool, retry bool) []DbstaticTwoStringOneInt {
	if cache.itemstwostring.Check(key) {
		if checkexpire {
			if cache.itemstwostring.CheckExpires(
				key,
				config.SettingsGeneral.CacheAutoExtend,
				config.SettingsGeneral.CacheDuration,
			) {
				return nil
			}
		}
	}
	return getrefresh(cache.itemstwostring, key, retry)
}

// GetCachedTwoIntArr retrieves the cached array of DbstaticOneStringTwoInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedTwoIntArr(key string, checkexpire bool, retry bool) []DbstaticOneStringTwoInt {
	if cache.itemstwoint.Check(key) {
		if checkexpire {
			if cache.itemstwoint.CheckExpires(
				key,
				config.SettingsGeneral.CacheAutoExtend,
				config.SettingsGeneral.CacheDuration,
			) {
				return nil
			}
		}
	}
	return getrefresh(cache.itemstwoint, key, retry)
}

// GetCachedThreeStringArr retrieves the cached array of DbstaticThreeStringTwoInt objects associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
// The checkexpire parameter determines whether to check if the cached object has expired.
// The retry parameter determines whether to refresh the cached object if it is empty or the zero value.
func GetCachedThreeStringArr(key string, checkexpire bool, retry bool) []DbstaticThreeStringTwoInt {
	if cache.itemsthreestring.Check(key) {
		if checkexpire {
			if cache.itemsthreestring.CheckExpires(
				key,
				config.SettingsGeneral.CacheAutoExtend,
				config.SettingsGeneral.CacheDuration,
			) {
				return nil
			}
		}
	}
	return getrefresh(cache.itemsthreestring, key, retry)
}

// getrefresh retrieves the value associated with the given key from the SyncMap, and refreshes the
// cached value if the retry flag is set and the cached value is empty or the zero value of the
// generic type T.
func getrefresh[T comparable](s *logger.SyncMap[[]T], key string, retry bool) []T {
	t := s.GetVal(key)
	var x T
	if retry && (len(t) == 0 || t[0] == x) {
		RefreshCached(key, true)
		return s.GetVal(key)
	}
	return t
}

// storeMap stores a map with a string key and a slice of strings as the value in the cache.
// It first checks if the key already exists in the cache, and if so, updates the expiration,
// value, and last scan time. Otherwise, it adds the new key-value pair to the cache.
// The logger.LogDynamicany1String function is called to log a debug message.
func (c *tcache) storeMap(s string, val []string, expires, lastscan int64) {
	logger.LogDynamicany1String("debug", "refresh cache store", strquery, s)

	if cache.itemsstring.Check(s) {
		cache.itemsstring.UpdateExpire(s, expires)
		cache.itemsstring.UpdateVal(s, val)
		cache.itemsstring.UpdateLastscan(s, lastscan)
		return
	}
	c.itemsstring.Add(s, val, expires, false, lastscan)
}

// storeMapTwoInt stores a map with one string key and two integer values in the cache.
// It first checks if the key already exists in the cache, and if so, updates the expiration,
// value, and last scan time. Otherwise, it adds the new key-value pair to the cache.
// The logger.LogDynamicany1String function is called to log a debug message.
func (c *tcache) storeMapTwoInt(s string, val []DbstaticOneStringTwoInt, expires, lastscan int64) {
	logger.LogDynamicany1String("debug", "refresh cache store", strquery, s)

	if cache.itemstwoint.Check(s) {
		cache.itemstwoint.UpdateExpire(s, expires)
		cache.itemstwoint.UpdateVal(s, val)
		cache.itemstwoint.UpdateLastscan(s, lastscan)
		return
	}
	c.itemstwoint.Add(s, val, expires, false, lastscan)
}

// storeMapThreeString stores a map with three string keys and two integer values in the cache.
// It first checks if the key already exists in the cache, and if so, updates the expiration,
// value, and last scan time. Otherwise, it adds the new key-value pair to the cache.
// The logger.LogDynamicany1String function is called to log a debug message.
func (c *tcache) storeMapThreeString(
	s string,
	val []DbstaticThreeStringTwoInt,
	expires, lastscan int64,
) {
	logger.LogDynamicany1String("debug", "refresh cache store", strquery, s)

	if cache.itemsthreestring.Check(s) {
		cache.itemsthreestring.UpdateExpire(s, expires)
		cache.itemsthreestring.UpdateVal(s, val)
		cache.itemsthreestring.UpdateLastscan(s, lastscan)
		return
	}
	c.itemsthreestring.Add(s, val, expires, false, lastscan)
}

// storeMapTwoString stores a map with two string keys and one integer value in the cache.
// It first checks if the key already exists in the cache, and if so, updates the expiration,
// value, and last scan time. Otherwise, it adds the new key-value pair to the cache.
// The logger.LogDynamicany1String function is called to log a debug message.
func (c *tcache) storeMapTwoString(
	s string,
	val []DbstaticTwoStringOneInt,
	expires, lastscan int64,
) {
	logger.LogDynamicany1String("debug", "refresh cache store", strquery, s)

	if cache.itemstwostring.Check(s) {
		cache.itemstwostring.UpdateExpire(s, expires)
		cache.itemstwostring.UpdateVal(s, val)
		cache.itemstwostring.UpdateLastscan(s, lastscan)
		return
	}
	c.itemstwostring.Add(s, val, expires, false, lastscan)
}

// CheckcachedMovieTitleHistory checks if the given movie title exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the title exists, false
// otherwise.
func CheckcachedTitleHistory(useseries bool, file *string) bool {
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryTitle, file)
	}
	return Getdatarow1[uint](
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
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryURL, file)
	}
	return Getdatarow1[uint](
		false,
		logger.GetStringsMap(useseries, logger.DBCountHistoriesByURL),
		file,
	) >= 1
}

// InvalidateImdbStmt iterates over the cache.items sync.Map and deletes any
// cached items that have the imdb field set to true. This is likely used to
// invalidate any cached IMDB-related data when needed.
func InvalidateImdbStmt() {
	cache.itemsxstmt.DeleteFuncImdbVal(func(b bool) bool {
		return b
	}, func(s sqlx.Stmt) {
		s.Close()
	})
	cache.itemsxstmtP.DeleteFuncImdbVal(func(b bool) bool {
		return b
	}, func(s *sqlx.Stmt) {
		s.Close()
	})
}

// getregex returns a compiled regular expression based on the provided key.
// It supports the following keys:
//
// "RegexSeriesTitle": Matches a series title with optional season and episode information.
// "RegexSeriesTitleDate": Matches a series title with a date.
// "RegexSeriesIdentifier": Matches a series identifier (season and episode or date).
// For any other key, it returns a regular expression compiled from the key string.
func getregex(key string) regexp.Regexp {
	return *getregexP(key)
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

// Getfirstsubmatchindex returns the indexes of the first submatch found
// in matchfor using the compiled regular expression stored in cache
// under key.
// func Getfirstsubmatchindex(key, matchfor string) []int {
// 	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
// 	return rgx.FindStringSubmatchIndex(matchfor)
// }

// Getallsubmatchindex returns the indexes of the last submatch found in matchfor
// using the compiled regular expression stored in cache under key. If no matches
// are found, it returns nil.
// func Getallsubmatchindex(key, matchfor string) []int {
// 	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
// 	matches := rgx.FindAllStringSubmatchIndex(matchfor, 10)
// 	if !(len(matches) >= 1 && len(matches[len(matches)-1]) >= 1) {
// 		return nil
// 	}
// 	return matches[len(matches)-1]
// }

// RunRetRegex returns the indexes of the last submatch found in matchfor
// using the compiled regular expression stored in the global cache under key.
// If useall is true, it returns the indexes of the last submatch from all
// matches found, otherwise it returns the indexes of the first submatch.
// If no matches are found, it returns nil.
func RunRetRegex(key string, matchfor string, useall bool) []int {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	if useall {
		matches := rgx.FindAllStringSubmatchIndex(matchfor, 10)
		if len(matches) < 1 || len(matches[len(matches)-1]) < 1 {
			return nil
		}
		return matches[len(matches)-1]
	}
	return rgx.FindStringSubmatchIndex(matchfor)
}

// RegexGetMatchesFind checks if the regular expression matches the input
// string at least mincount times. It returns true if there are at least
// mincount matches, false otherwise. The regular expression is retrieved
// from the global cache.
func RegexGetMatchesFind(key, matchfor string, mincount int) bool {
	rgx := globalCache.setRegexp(key, globalCache.defaultextension)
	if mincount == 1 {
		return len(rgx.FindStringIndex(matchfor)) >= 1
	}
	return len(rgx.FindAllStringIndex(matchfor, mincount)) >= mincount
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

// startJanitor starts the background cache janitor goroutine.
func startJanitor() {
	cache.janitor = time.NewTimer(cache.interval)
	go func() {
		for {
			<-cache.janitor.C
			now := time.Now().UnixNano()
			cache.itemsstring.DeleteFuncExpires(func(d int64) bool {
				return d != 0 && now >= d
			})
			cache.itemsthreestring.DeleteFuncExpires(func(d int64) bool {
				return d != 0 && now >= d
			})
			cache.itemstwoint.DeleteFuncExpires(func(d int64) bool {
				return d != 0 && now >= d
			})
			cache.itemstwostring.DeleteFuncExpires(func(d int64) bool {
				return d != 0 && now >= d
			})

			cache.itemsxstmt.DeleteFuncExpiresVal(func(d int64) bool {
				return d != 0 && now >= d
			}, func(s sqlx.Stmt) {
				s.Close()
			})
			cache.itemsregex.DeleteFuncExpires(func(d int64) bool {
				return d != 0 && now >= d
			})
			cache.janitor.Reset(cache.interval)
		}
	}()
}

// getexpireskey returns the expiration time in nanoseconds for the given cache key
// and duration. Special cases like 0 duration or predefined keys will not expire.
func getexpireskey(duration time.Duration) int64 {
	return time.Now().Add(duration).UnixNano()
}

// NewRegex creates a new GlobalcacheRegex instance.
// cleaningInterval specifies the interval to clean up expired regex entries.
// extension specifies the default expiration duration to use for cached regexes.
// log is the logger used for logging.
// It initializes the cache and starts a goroutine to clean up expired entries
// based on the cleaningInterval.
func NewCache(cleaningInterval, extension time.Duration) {
	if cleaningInterval >= 1 {
		startJanitor() // for cache items
	}

	globalCache = &globalcache{
		defaultextension: extension,
	}
}

// SetStaticRegexp sets a static regular expression in the global cache.
// The regular expression can be used for fast matching without compiling it every time.
func SetStaticRegexp(key string) {
	globalCache.setStaticRegexp(key)
}
