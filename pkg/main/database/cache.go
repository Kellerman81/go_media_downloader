package database

import (
	"regexp"
	"slices"
	"strconv"
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
	items      []tcacheitem[any]           //sync.Map
	itemsstmt  []tcacheitem[sqlx.Stmt]     //sync.Map
	itemsregex []tcacheitem[regexp.Regexp] //sync.Map
	janitor    *time.Timer
	interval   time.Duration
}

type tcacheitem[t any] struct {
	name string
	// expires is the expiration time for the cached statement
	expires int64
	// imdb indicates if the statement is for the imdb database
	imdb bool
	// value is the prepared statement
	value    t
	lastscan int64
}

var strquery = "query"
var strcache = "cache"

func GetItemType[t any](z *tcacheitem[t]) *t {
	return &z.value
}

// Load retrieves the value stored in the cache for the given key.
// If the key is not found in the cache, the second return value will be false.
func (c *tcache) loadStmt(s string) (*tcacheitem[sqlx.Stmt], bool) {
	for idx := range c.itemsstmt {
		if c.itemsstmt[idx].name == s {
			return &c.itemsstmt[idx], true
		}
	}
	return nil, false
	//return c.items.Load(any(s))
}
func (c *tcache) loadRegex(s string) (*tcacheitem[regexp.Regexp], bool) {
	for idx := range c.itemsregex {
		if c.itemsregex[idx].name == s {
			return &c.itemsregex[idx], true
		}
	}
	return nil, false
	//return c.items.Load(any(s))
}

// Store stores the given value under the specified string key in the cache.
func (c *tcache) store(s string, val any, expires int64, imdb bool, lastscan int64) {
	logger.LogDynamicany("debug", "store val", &strquery, s)
	i := -1
	for idx := range c.items {
		if c.items[idx].name == s {
			i = idx
			break
		}
	}
	if i != -1 {
		c.items[i].value = nil
		c.items[i].value = val
		c.items[i].expires = expires
		c.items[i].lastscan = lastscan
	} else {
		c.items = append(c.items, tcacheitem[any]{name: s, value: val, expires: expires, imdb: imdb, lastscan: lastscan})
	}
}
func (c *tcache) storeStmt(s string, val *sqlx.Stmt, expires int64, imdb bool, lastscan int64) {
	i := -1
	for idx := range c.itemsstmt {
		if c.itemsstmt[idx].name == s {
			i = idx
			break
		}
	}
	if i != -1 {
		c.itemsstmt[i].value = *val
		c.itemsstmt[i].expires = expires
		c.itemsstmt[i].lastscan = lastscan
	} else {
		c.itemsstmt = append(c.itemsstmt, tcacheitem[sqlx.Stmt]{name: s, value: *val, expires: expires, imdb: imdb, lastscan: lastscan})
	}
}
func (c *tcache) storeRegex(s string, val *regexp.Regexp, expires int64, imdb bool, lastscan int64) {
	i := -1
	for idx := range c.itemsregex {
		if c.itemsregex[idx].name == s {
			i = idx
			break
		}
	}
	if i != -1 {
		c.itemsregex[i].value = *val
		c.itemsregex[i].expires = expires
		c.itemsregex[i].lastscan = lastscan
	} else {
		c.itemsregex = append(c.itemsregex, tcacheitem[regexp.Regexp]{name: s, value: *val, expires: expires, imdb: imdb, lastscan: lastscan})
	}
}
func (c *tcache) deleteStmt(s string) {
	i := slices.IndexFunc(c.itemsstmt, func(d tcacheitem[sqlx.Stmt]) bool {
		return (d.name == s)
	})
	if i != -1 {
		c.itemsstmt = slices.Delete(c.itemsstmt, i, i+1)
	}
}

// cache is a struct that stores the cache items map,
// a janitor timer, and an interval duration
// items is a sync.Map that stores cached items
// janitor is a timer that periodically cleans up expired cache items
// interval is the duration between janitor cleanups
var (
	cache = tcache{
		//items:    sync.Map{},      // Initialize empty sync.Map
		items:      make([]tcacheitem[any], 0, 1000),
		itemsstmt:  make([]tcacheitem[sqlx.Stmt], 0, 1000),
		itemsregex: make([]tcacheitem[regexp.Regexp], 0, 1000),
		interval:   5 * time.Minute, // Set default interval to 5 minutes
	}
	globalCache *globalcache
	mu          = sync.Mutex{} //To make sure that the cache is only initialized once
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

// GetStmt retrieves a prepared statement from the cache, or prepares it if not cached.
// It locks access during retrieval and update.
// Key is the statement text.
// Db is the database connection.
// It returns the prepared statement.
func (c *globalcache) getStmt(key string, imdb bool) *sqlx.Stmt {
	item, ok := cache.loadStmt(key)
	if !ok {
		sq, err := Getdb(imdb).Preparex(key)
		if err != nil {
			return nil
		}
		cache.storeStmt(key, sq, c.getexpiressql(), imdb, 0)
		return sq
		//return c.setsql(key, imdb)
	}
	if item.expires != 0 && time.Now().UnixNano() > item.expires {
		item.expires = c.getexpiressql()
	} else if item.expires != 0 {
		if config.SettingsGeneral.CacheAutoExtend {
			item.expires = c.getexpiressql()
		}
	}
	return &item.value
}

// setRegexp sets a cached regular expression with the given key and duration.
// If the cached regular expression does not exist or is expired, it creates a new one.
// If the cache auto-extend feature is enabled, it will extend the expiration time of the cached regular expression.
// The function returns the cached regular expression.
func (c *globalcache) setRegexp(key string, duration time.Duration) *regexp.Regexp {
	item, ok := cache.loadRegex(key)
	if !ok {
		regex := getregex(key)
		if regex == nil {
			return nil
		}

		if duration == 0 {
			duration = c.defaultextension
		}
		var expires int64
		if duration == 0 || key == "RegexSeriesIdentifier" || key == "RegexSeriesTitle" {
			expires = 0
		} else {
			expires = getexpireskey(duration)
		}
		cache.storeRegex(key, regex, expires, false, 0)
		return regex
		//return c.set(key, duration)
	}

	if item.expires != 0 && time.Now().UnixNano() > item.expires {
		if duration == 0 || key == "RegexSeriesIdentifier" || key == "RegexSeriesTitle" {
			item.expires = 0
		} else {
			item.expires = getexpireskey(c.defaultextension)
		}
	} else if item.expires != 0 {
		if config.SettingsGeneral.CacheAutoExtend {
			if duration == 0 || key == "RegexSeriesIdentifier" || key == "RegexSeriesTitle" {
				item.expires = 0
			} else {
				item.expires = getexpireskey(c.defaultextension)
			}
		}
	}
	return &item.value
}

// InitCache initializes the global cache by creating a new Cache instance
// with the provided expiration times and logger. It is called on startup
// to initialize the cache before it is used.
func InitCache() {
	NewCache(1*time.Hour, time.Hour*time.Duration(config.SettingsGeneral.CacheDuration))
}

// ClearCaches iterates over the cached string, three string two int, and two string int arrays, sets the Expire field on each cached array object to two hours in the past based on the config cache duration, and updates the cache with the expired array object. This effectively clears those cached arrays by expiring all entries.
func ClearCaches() {
	oldi := time.Now().Add(time.Hour * -time.Duration(2+config.SettingsGeneral.CacheDuration)).UnixNano()
	for idx := range cache.items {
		cache.items[idx].expires = oldi
	}
	for idx := range cache.itemsstmt {
		cache.itemsstmt[idx].expires = oldi
	}
	for idx := range cache.itemsregex {
		cache.itemsregex[idx].expires = oldi
	}
}

// AppendCache appends the given value v to the cached array for the given key a.
// If the cached array for the given key does not exist, this function does nothing.
// If the given value is already present in the cached array, this function does nothing.
func AppendCache[T comparable](a string, v T) {
	s := getCachedTypeObj(a, false)
	if s == nil {
		return
	}
	if ArrStructContains(s.value.([]T), v) {
		return
	}
	s.value = append(s.value.([]T), v)
}

// AppendCacheMap appends a value to the cache map for the given query string.
// If useseries is true, it appends to the logger.Mapstringsseries map, otherwise it appends to the logger.Mapstringsmovies map.
// The value is appended using the AppendCache function.
func AppendCacheMap[T comparable](useseries bool, query string, v T) {
	AppendCache(logger.GetStringsMap(useseries, query), v)
}

// ArrStructContains checks if the given slice s contains the value v.
// The comparison is performed using the comparable constraint, which allows
// comparing values of the same type. If the values are of a custom struct type,
// the comparison is performed by checking if all fields of the struct are equal.
func ArrStructContains[T comparable](s []T, v T) bool {
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
func SlicesCacheContainsI(useseries bool, query string, v string) bool {
	a := getCachedTypeObjArrMap[string](useseries, query, false)
	if a == nil {
		return false
	}
	for idx := range a {
		if v == a[idx] || strings.EqualFold(v, a[idx]) {
			return true
		}
	}
	return false
}

// SlicesCacheContains checks if the cached string array identified by s contains
// the value v. It iterates over the array and returns true if a match is found.
func SlicesCacheContains(useseries bool, query string, v string) bool {
	a := getCachedTypeObjArrMap[string](useseries, query, false)
	if a == nil {
		return false
	}
	return ArrStructContains(a, v)
}

// SlicesCacheContainsDelete removes an element matching v from the cached string
// array identified by s. It acquires a lock, defers unlocking, and iterates
// over the array to find a match. When found, it uses slices.Delete to remove
// the element while preserving order, updates the cache, and returns.
func SlicesCacheContainsDelete(s string, v string) {
	a := getCachedTypeObj(s, false)
	if a == nil {
		return
	}
	for idx := range a.value.([]string) {
		if v == a.value.([]string)[idx] {
			a.value = slices.Delete(a.value.([]string), idx, idx+1)
			return
		}
	}
}

// CacheOneStringTwoIntIndexFunc looks up the cached one string two int array
// identified by s and calls the passed in function f on each element.
// Returns true if f returns true for any element.
func CacheOneStringTwoIntIndexFunc(s string, f func(DbstaticOneStringTwoInt) bool) bool {
	a := GetCachedTypeObjArr[DbstaticOneStringTwoInt](s, false)
	if a == nil {
		return false
	}
	for idx := range a {
		if f(a[idx]) {
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
	a := GetCachedTypeObjArr[DbstaticOneStringTwoInt](s, false)
	if a == nil {
		return 0
	}
	for idx := range a {
		if a[idx].Num1 == id && strings.EqualFold(a[idx].Str, listname) {
			return a[idx].Num2
		}
	}
	return 0
}

// CacheOneStringTwoIntIndexFuncStr looks up the cached one string, two int array
// identified by s and returns the string value for the entry where the
// second int matches the passed in uint i. It stores the returned string in
// listname. If no match is found, it sets listname to an empty string.
func CacheOneStringTwoIntIndexFuncStr(useseries bool, query string, i uint) string {
	a := getCachedTypeObjArrMap[DbstaticOneStringTwoInt](useseries, query, false)
	if a == nil {
		return ""
	}
	for idx := range a {
		if a[idx].Num2 == i {
			return a[idx].Str
		}
	}
	return ""
}

// CacheThreeStringIntIndexFunc looks up the cached three string, two int array
// identified by s and returns the second int value for the entry where the
// third string matches the string str. Returns 0 if no match found.
func CacheThreeStringIntIndexFunc(s string, t string) uint {
	if t == "" {
		return 0
	}
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s, false)
	if a == nil {
		return 0
	}
	for idx := range a {
		if a[idx].Str3 == t || strings.EqualFold(a[idx].Str3, t) {
			return a[idx].Num2
		}
	}
	return 0
}

// CacheThreeStringIntIndexFuncGetYear looks up the cached three string, two int array
// identified by s and returns the first int value for the entry where the second int
// matches the int str. Returns 0 if no match found.
func CacheThreeStringIntIndexFuncGetYear(s string, i uint) uint16 {
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s, false)
	if a == nil {
		return 0
	}
	for idx := range a {
		if a[idx].Num2 == i {
			return uint16(a[idx].Num1)
		}
	}
	return 0
}

// RefreshMediaCache refreshes the media caches for movies and series.
// It will refresh the caches based on the useseries parameter:
// - if useseries is false, it will refresh the movie caches
// - if useseries is true, it will refresh the series caches
// It locks access to the caches while refreshing to prevent concurrent access issues.
func RefreshMediaCache(useseries bool) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour
	doupdate := true

	if useseries {
		item := getCachedTypeObjArrMap[DbstaticTwoStringOneInt](useseries, logger.CacheDBMedia, true)
		if item != nil {
			if len(item) == GetdatarowN[int](false, "select count() from dbseries") {
				doupdate = false
			}
		}
		if item != nil && doupdate && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheDBMedia)) {
			doupdate = false
		}
		if doupdate {
			cache.store(logger.CacheDBSeries, GetrowsN[DbstaticTwoStringOneInt](false, GetdatarowN[uint](false, "select count() from dbseries")+100,
				"select seriename, slug, id from dbseries"),
				time.Now().Add(dur).UnixNano(), false,
				time.Now().UnixNano(),
			)
		}
	} else {
		item := getCachedTypeObjArrMap[DbstaticThreeStringTwoInt](useseries, logger.CacheDBMedia, true)
		if item != nil {
			if len(item) == GetdatarowN[int](false, "select count() from dbmovies") {
				doupdate = false
			}
		}
		if item != nil && doupdate && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheDBMedia)) {
			doupdate = false
		}
		if doupdate {
			logger.LogDynamicany("debug", "refresh cache", &strcache, logger.GetStringsMap(useseries, logger.CacheDBMedia))
			cache.store(logger.CacheDBMovie, GetrowsN[DbstaticThreeStringTwoInt](false, GetdatarowN[uint](false, "select count() from dbmovies")+100,
				"select title, slug, imdb_id, year, id from dbmovies"),
				time.Now().Add(dur).UnixNano(), false,
				time.Now().UnixNano())
		}
	}
	doupdate = true
	item := getCachedTypeObjArrMap[DbstaticOneStringTwoInt](useseries, logger.CacheMedia, true)
	if item != nil {
		if len(item) == getdatarowNMap[int](false, useseries, logger.DBCountMedia) {
			doupdate = false
		}
	}
	if item != nil && doupdate && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheMedia)) {
		doupdate = false
	}
	if doupdate {
		logger.LogDynamicany("debug", "refresh cache", &strcache, logger.GetStringsMap(useseries, logger.CacheMedia))
		cache.storeMap(useseries, logger.CacheMedia, GetrowsN[DbstaticOneStringTwoInt](false, getdatarowNMap[uint](false, useseries, logger.DBCountMedia)+100,
			logger.GetStringsMap(useseries, logger.DBCacheMedia)),
			time.Now().Add(dur).UnixNano(), false,
			time.Now().UnixNano(),
		)
	}
}

// Refreshhistorycache refreshes the cached history title and URL arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func Refreshhistorycache(useseries bool) {
	if !config.SettingsGeneral.UseHistoryCache {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	doupdate := true
	item := getCachedTypeObjArrMap[string](useseries, logger.CacheHistoryTitle, true)
	if item != nil {
		if len(item) == getdatarowNMap[int](false, useseries, logger.DBCountHistoriesTitle) {
			doupdate = false
		}
	}
	if item != nil && doupdate && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheHistoryTitle)) {
		doupdate = false
	}
	if doupdate {
		logger.LogDynamicany("debug", "refresh cache", &strcache, logger.GetStringsMap(useseries, logger.CacheHistoryTitle))
		cache.storeMap(useseries, logger.CacheHistoryTitle, GetrowsN[string](false, getdatarowNMap[uint](false, useseries, logger.DBCountHistoriesTitle)+100,
			logger.GetStringsMap(useseries, logger.DBHistoriesTitle)),
			time.Now().Add(dur).UnixNano(), false,
			time.Now().UnixNano(),
		)
	}

	doupdate = true
	item2 := getCachedTypeObjArrMap[string](useseries, logger.CacheHistoryUrl, true)
	if item2 != nil {
		if len(item2) == getdatarowNMap[int](false, useseries, logger.DBCountHistoriesUrl) {
			doupdate = false
		}
	}
	if item2 != nil && doupdate && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheHistoryUrl)) {
		doupdate = false
	}
	if doupdate {
		logger.LogDynamicany("debug", "refresh cache", &strcache, logger.GetStringsMap(useseries, logger.CacheHistoryUrl))
		cache.storeMap(useseries, logger.CacheHistoryUrl, GetrowsN[string](false, getdatarowNMap[uint](false, useseries, logger.DBCountHistoriesUrl)+100,
			logger.GetStringsMap(useseries, logger.DBHistoriesUrl)),
			time.Now().Add(dur).UnixNano(), false,
			time.Now().UnixNano(),
		)
	}
}

// RefreshMediaCacheTitles refreshes the cached media title arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func RefreshMediaCacheTitles(useseries bool) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	item := getCachedTypeObjArrMap[DbstaticTwoStringOneInt](useseries, logger.CacheMediaTitles, true)
	if item != nil {
		if len(item) == getdatarowNMap[int](false, useseries, logger.DBCountDBTitles) {
			return
		}
	}
	if item != nil && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheMediaTitles)) {
		return
	}
	logger.LogDynamicany("debug", "refresh media titles cache")
	cache.storeMap(useseries, logger.CacheMediaTitles, GetrowsN[DbstaticTwoStringOneInt](false, getdatarowNMap[uint](false, useseries, logger.DBCountDBTitles)+100,
		logger.GetStringsMap(useseries, logger.DBCacheDBTitles)),
		time.Now().Add(dur).UnixNano(), false,
		time.Now().UnixNano(),
	)
}

// Refreshfilescached refreshes the cached file location arrays for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func Refreshfilescached(useseries bool) {
	if !config.SettingsGeneral.UseFileCache {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	item := getCachedTypeObjArrMap[string](useseries, logger.CacheFiles, true)
	if item != nil {
		if len(item) == getdatarowNMap[int](false, useseries, logger.DBCountFiles) {
			return
		}
	}

	if item != nil && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheFiles)) {
		return
	}
	logger.LogDynamicany("debug", "refresh files cache")
	cache.storeMap(useseries, logger.CacheFiles, GetrowsN[string](false, getdatarowNMap[uint](false, useseries, logger.DBCountFiles)+300,
		logger.GetStringsMap(useseries, logger.DBCacheFiles)),
		time.Now().Add(dur).UnixNano(), false,
		time.Now().UnixNano(),
	)
}

// Refreshunmatchedcached refreshes the cached string array of unmatched files for movies or series.
// It handles locking and unlocking the cache mutex.
// The useseries parameter determines if it refreshes the cache for series or movies.
func Refreshunmatchedcached(useseries bool) {
	if !config.SettingsGeneral.UseFileCache {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	hours12 := logger.JoinStrings("-", strconv.Itoa(config.SettingsGeneral.CacheDuration), " hours")
	hours24 := logger.JoinStrings("-", strconv.Itoa(2*config.SettingsGeneral.CacheDuration), " hours")
	ExecNMap(useseries, "DBRemoveUnmatched", hours24)
	item := getCachedTypeObjArrMap[string](useseries, logger.CacheUnmatched, true)
	if item != nil {
		if len(item) == Getdatarow1Map[int](false, useseries, logger.DBCountUnmatched, hours12) {
			return
		}
	}
	if item != nil && !checkcachelastscan(logger.GetStringsMap(useseries, logger.CacheUnmatched)) {
		return
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour
	logger.LogDynamicany("debug", "refresh unmatched cache")
	cache.storeMap(useseries, logger.CacheUnmatched, GetrowsN[string](false, Getdatarow1Map[uint](false, useseries, logger.DBCountUnmatched, hours12)+300,
		logger.GetStringsMap(useseries, logger.DBCacheUnmatched), hours12),
		time.Now().Add(dur).UnixNano(), false,
		time.Now().UnixNano(),
	)
}

// RefreshCached refreshes the cached data for the specified key. It calls the appropriate
// refresh function based on the key, such as RefreshMediaCache, RefreshMediaCacheTitles,
// Refreshunmatchedcached, Refreshfilescached, or Refreshhistorycache.
func RefreshCached(key string) {
	switch key {
	case logger.CacheMovie:
		{
			RefreshMediaCache(false)
		}
	case logger.CacheSeries:
		{
			RefreshMediaCache(true)
		}
	case logger.CacheDBMovie:
		{
			RefreshMediaCache(false)
		}
	case logger.CacheDBSeriesAlt:
		{
			RefreshMediaCacheTitles(true)
		}
	case logger.CacheDBSeries:
		{
			RefreshMediaCache(true)
		}
	case logger.CacheTitlesMovie:
		{
			RefreshMediaCacheTitles(false)
		}
	case logger.CacheUnmatchedMovie:
		{
			Refreshunmatchedcached(false)
		}
	case logger.CacheUnmatchedSeries:
		{
			Refreshunmatchedcached(true)
		}
	case logger.CacheFilesMovie:
		{
			Refreshfilescached(false)
		}
	case logger.CacheFilesSeries:
		{
			Refreshfilescached(true)
		}
	case logger.CacheHistoryUrlMovie, logger.CacheHistoryTitleMovie:
		{
			Refreshhistorycache(false)
		}
	case logger.CacheHistoryUrlSeries, logger.CacheHistoryTitleSeries:
		{
			Refreshhistorycache(true)
		}
	}
}

// GetCachedTypeObj retrieves the cached CacheTypeExpire[T] object associated with the given key.
// If no cached object is found for the key, or the cached object has expired, it returns nil.
func getCachedTypeObj(key string, checkexpire bool) *tcacheitem[any] {
	if checkexpire && !checkcacheexpireDataType(key) {
		return nil
	}
	for idx := range cache.items {
		if cache.items[idx].name == key {
			return &cache.items[idx]
		}
	}
	return nil
}

// GetCachedTypeObjArr retrieves the cached array of type T associated with the given key.
// If no cached object is found for the key, it returns nil.
func GetCachedTypeObjArr[t any](key string, checkexpire bool) []t {
	a := getCachedTypeObj(key, checkexpire)
	if a == nil {
		return nil
	}
	return a.value.([]t)
}
func getCachedTypeObjArrMap[t any](useseries bool, query string, checkexpire bool) []t {
	return GetCachedTypeObjArr[t](logger.GetStringsMap(useseries, query), checkexpire)
}

// CheckcacheexpireData checks if the given cached data has expired based
// on its internal timestamp. Returns false if the cache entry has expired.
func checkcacheexpireDataType(s string) bool {
	for idx := range cache.items {
		if cache.items[idx].name != s {
			continue
		}
		if cache.items[idx].expires != 0 && cache.items[idx].expires < time.Now().UnixNano() {
			if config.SettingsGeneral.CacheAutoExtend {
				cache.items[idx].expires = time.Now().Add(time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour).UnixNano()
			} else {
				return false
			}
		}
		return true
	}
	return true
}

func checkcachelastscan(s string) bool {
	for idx := range cache.items {
		if cache.items[idx].name != s {
			continue
		}
		if cache.items[idx].lastscan != 0 && cache.items[idx].lastscan > time.Now().Add(-1*time.Minute).UnixNano() {
			return false
		}
		return true
	}
	return true
}

func (c *tcache) storeMap(useseries bool, s string, val any, expires int64, imdb bool, lastscan int64) {
	c.store(logger.GetStringsMap(useseries, s), val, expires, imdb, lastscan)
}

// CheckcachedMovieTitleHistory checks if the given movie title exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the title exists, false
// otherwise.
func CheckcachedTitleHistory(useseries bool, file *string) bool {
	if file == nil || *file == "" {
		return false
	}
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryTitle, *file)
	}
	return Getdatarow1Map[uint](false, useseries, logger.DBCountHistoriesByTitle, file) >= 1
}

// CheckcachedMovieUrlHistory checks if the given movie URL exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the URL exists, false
// otherwise.
func CheckcachedUrlHistory(useseries bool, file *string) bool {
	if file == nil || *file == "" {
		return false
	}
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(useseries, logger.CacheHistoryUrl, *file)
	}
	return Getdatarow1Map[uint](false, useseries, logger.DBCountHistoriesByUrl, file) >= 1
}

// InvalidateImdbStmt iterates over the cache.items sync.Map and deletes any
// cached items that have the imdb field set to true. This is likely used to
// invalidate any cached IMDB-related data when needed.
func InvalidateImdbStmt() {
	cache.itemsstmt = slices.DeleteFunc(cache.itemsstmt, func(d tcacheitem[sqlx.Stmt]) bool {
		return d.imdb
	})
}

// getregex returns a compiled regular expression for the given key.
// It has some predefined keys that map to common regexes.
// Otherwise it compiles the key directly as the regex pattern.
func getregex(key string) *regexp.Regexp {
	switch key {
	case "RegexSeriesTitle":
		return regexp.MustCompile(`^(.*)(?i)(?:(?:\.| - |-)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$))`)
	case "RegexSeriesTitleDate":
		return regexp.MustCompile(`^(.*)(?i)(?:\.|-| |_)(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$)`)
	case "RegexSeriesIdentifier":
		return regexp.MustCompile(`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`)
	default:
		return regexp.MustCompile(key)
	}
}

// Getfirstsubmatchindex returns the indexes of the first submatch found
// in matchfor using the compiled regular expression stored in cache
// under key.
func Getfirstsubmatchindex(key string, matchfor string) []int {
	return globalCache.setRegexp(key, globalCache.defaultextension).FindStringSubmatchIndex(matchfor)
}

func Getallsubmatchindex(key string, matchfor string) [][]int {
	return globalCache.setRegexp(key, globalCache.defaultextension).FindAllStringSubmatchIndex(matchfor, 10)
}

// getmatches returns the indexes of the first submatch found in matchfor
// using the compiled regular expression stored in cache under key. If cached
// is false, it will compile the regex and find matches without caching.
func getmatches(cached bool, key string, matchfor string) []int {
	if !cached {
		return regexp.MustCompile(key).FindStringSubmatchIndex(matchfor)
	}
	return Getfirstsubmatchindex(key, matchfor)
}

// RegexGetMatchesStr1Str2 searches for regex matches in a string.
// It takes a boolean to indicate if the regex should use the cached compiled version,
// the regex string, and the string to search.
// It returns two submatch strings from the match or empty strings if no match.
func RegexGetMatchesStr1Str2(cached bool, key string, matchfor string) (string, string) {
	matches := getmatches(cached, key, matchfor)
	//defer clear(matches)
	lenm := len(matches)
	if lenm == 0 {
		return "", ""
	}
	//ex Date := [0,8,-1,-1,0,8]
	if lenm >= 6 {
		if matches[3] != -1 && matches[5] != -1 {
			return matchfor[matches[2]:matches[3]], matchfor[matches[4]:matches[5]]
		}
		if matches[3] == -1 && matches[5] != -1 {
			return "", matchfor[matches[4]:matches[5]]
		}
	}
	if lenm >= 4 && matches[3] != -1 {
		return matchfor[matches[2]:matches[3]], ""
	}

	return "", ""
}

// RegexGetMatchesFind checks if the regular expression matches the input
// string at least mincount times. It returns true if there are at least
// mincount matches, false otherwise. The regular expression is retrieved
// from the global cache.
func RegexGetMatchesFind(key string, matchfor string, mincount int) bool {
	if mincount == 1 {
		return len(globalCache.setRegexp(key, globalCache.defaultextension).FindStringIndex(matchfor)) >= 1
	}
	return len(globalCache.setRegexp(key, globalCache.defaultextension).FindAllStringIndex(matchfor, mincount)) >= mincount
}

// StartJanitor starts the background cache janitor goroutine
func startJanitor() {
	cache.janitor = time.NewTimer(cache.interval)
	go func() {
		defer logger.HandlePanic()
		for {
			<-cache.janitor.C
			now := time.Now().UnixNano()
			for idx := range cache.items {
				if cache.items[idx].expires == 0 || now < cache.items[idx].expires {
					return
				}
				cache.items = slices.Delete(cache.items, idx, idx+1)
			}
			for idx := range cache.itemsstmt {
				if cache.itemsstmt[idx].expires == 0 || now < cache.itemsstmt[idx].expires {
					return
				}
				cache.itemsstmt = slices.Delete(cache.itemsstmt, idx, idx+1)
			}
			for idx := range cache.itemsregex {
				if cache.itemsregex[idx].expires == 0 || now < cache.itemsregex[idx].expires {
					return
				}
				cache.itemsregex = slices.Delete(cache.itemsregex, idx, idx+1)
			}
			// cache.items.Range(func(key, value any) bool {
			// 	cachedeleteexpire(key.(string), value)

			// 	return true
			// })
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
func NewCache(cleaningInterval time.Duration, extension time.Duration) {
	if cleaningInterval >= 1 {
		startJanitor() //for cache items
	}

	globalCache = &globalcache{
		defaultextension: extension,
	}
}
func SetRegexp(key string, duration time.Duration) *regexp.Regexp {
	return globalCache.setRegexp(key, duration)
}
