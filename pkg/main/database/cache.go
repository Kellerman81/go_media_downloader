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
	"github.com/rs/zerolog"
)

// cacheStringExpire is a struct that stores a cached array of strings,
// an expiration time, and the last time the cache was scanned
// type cacheStringExpire struct {
// 	// Arr is the cached array of strings
// 	Arr []string
// 	// expires is the expiration time for the cache
// 	expires int64
// 	// lastscan is the last time the cache was scanned
// 	lastscan time.Time
// }

// // CacheTwoStringIntExpire is a struct that stores a cached array of
// // DbstaticTwoStringOneInt values, an expiration time, and the last time
// // the cache was scanned
// type CacheTwoStringIntExpire struct {
// 	// Arr is the cached array of DbstaticTwoStringOneInt values
// 	Arr []DbstaticTwoStringOneInt
// 	// expires is the expiration time for the cache
// 	expires int64
// 	// lastscan is the last time the cache was scanned
// 	lastscan time.Time
// }

// type cacheOneStringIntExpire struct {
// 	Arr     []DbstaticOneStringOneInt
// 	expires int64
// }

// // CacheOneStringTwoIntExpire is a struct that stores a cached array of
// // DbstaticOneStringTwoInt values, an expiration time, and the last time
// // the cache was scanned
// type CacheOneStringTwoIntExpire struct {
// 	// Arr is the cached array of DbstaticOneStringTwoInt values
// 	Arr []DbstaticOneStringTwoInt
// 	// expires is the expiration time for the cache
// 	expires int64
// 	// lastscan is the last time the cache was scanned
// 	lastscan time.Time
// }

// // CacheThreeStringTwoIntExpire is a struct that stores a cached array of
// // DbstaticThreeStringTwoInt values, an expiration time, and the last time
// // the cache was scanned
// type CacheThreeStringTwoIntExpire struct {
// 	// Arr is the cached array of DbstaticThreeStringTwoInt values
// 	Arr []DbstaticThreeStringTwoInt
// 	// expires is the expiration time for the cache
// 	expires int64
// 	// lastscan is the last time the cache was scanned
// 	lastscan time.Time
// }

// type cacheTwoIntExpire struct {
// 	Arr     []DbstaticTwoInt
// 	expires int64
// }

type cacheTypeExpire[T any] struct {
	Arr     []T
	expires int64
	// lastscan is the last time the cache was scanned
	lastscan time.Time
}

var GlobalCache *globalcache
var mu = sync.Mutex{} //To make sure that the cache is only initialized once

// InitCache initializes the global cache by creating a new Cache instance
// with the provided expiration times and logger. It is called on startup
// to initialize the cache before it is used.
func InitCache() {
	GlobalCache = NewCache(1*time.Hour, time.Hour*time.Duration(config.SettingsGeneral.CacheDuration), &logger.Log)
}

// ClearCaches iterates over the cached string, three string two int, and two string int arrays, sets the Expire field on each cached array object to two hours in the past based on the config cache duration, and updates the cache with the expired array object. This effectively clears those cached arrays by expiring all entries.
func ClearCaches() {
	oldi := time.Now().Add(time.Hour * -time.Duration(2+config.SettingsGeneral.CacheDuration)).UnixNano()
	cache.items.Range(func(key, value any) bool {
		switch item := value.(type) {
		// case *cacheStringExpire:
		// 	item.expires = oldi
		// case *CacheThreeStringTwoIntExpire:
		// 	item.expires = oldi
		case *itemregex:
			item.expires = oldi
		case *cacheTypeExpire[string]:
			item.expires = oldi
		case *cacheTypeExpire[DbstaticThreeStringTwoInt]:
			item.expires = oldi
		case *cacheTypeExpire[DbstaticTwoStringOneInt]:
			item.expires = oldi
		case *cacheTypeExpire[DbstaticOneStringOneInt]:
			item.expires = oldi
		case *cacheTypeExpire[DbstaticOneStringTwoInt]:
			item.expires = oldi
		case *cacheTypeExpire[DbstaticTwoInt]:
			item.expires = oldi
		case *cacheTypeExpire[any]:
			item.expires = oldi
		case *itemstmt:
			item.expires = oldi
			item.value.Close()
		}

		return true
	})
}

// AppendStringCache appends the string value v to the cached string array
// identified by a. It acquires a lock on the cache, appends the value to
// the array, updates the cache with the modified array, and unlocks before
// returning.
func AppendStringCache(a string, v string) {
	s := GetCachedTypeObj[string](a)
	if s == nil {
		return
	}
	if getcacheidx(s, v) != -1 {
		return
	}
	s.Arr = append(s.Arr, v)
}

// AppendOneStringTwoIntCache appends the DbstaticOneStringTwoInt value v to the cached one string two int array identified by a. It acquires a lock on the cache, appends the value, updates the cache with the modified array, and unlocks before returning.
func AppendOneStringTwoIntCache(a string, v DbstaticOneStringTwoInt) {
	s := GetCachedTypeObj[DbstaticOneStringTwoInt](a)
	if s == nil {
		return
	}
	for idx := range s.Arr {
		if v.Num1 == s.Arr[idx].Num1 && v.Num2 == s.Arr[idx].Num2 && v.Str == s.Arr[idx].Str {
			return
		}
	}
	s.Arr = append(s.Arr, v)
}

// AppendTwoStringIntCache appends the DbstaticTwoStringOneInt value v to the
// cached two string one int array identified by a. It acquires a lock on the
// cache, appends the value to the array, updates the cache with the
// modified array, and unlocks before returning.
func AppendTwoStringIntCache(s string, v DbstaticTwoStringOneInt) {
	a := GetCachedTypeObj[DbstaticTwoStringOneInt](s)
	if a == nil {
		return
	}
	for idx := range a.Arr {
		if v.Num == a.Arr[idx].Num && v.Str1 == a.Arr[idx].Str1 && v.Str2 == a.Arr[idx].Str2 {
			return
		}
	}
	a.Arr = append(a.Arr, v)
}

// AppendThreeStringTwoIntCache appends the DbstaticThreeStringTwoInt value v
// to the cached three string two int array identified by a. It acquires a lock
// on the cache, defers unlocking, gets the cached array object, appends the
// value, updates the cache with the modified array, and returns.
func AppendThreeStringTwoIntCache(s string, v DbstaticThreeStringTwoInt) {
	a := GetCachedTypeObj[DbstaticThreeStringTwoInt](s)
	if a == nil {
		return
	}
	for idx := range a.Arr {
		if v.Num1 == a.Arr[idx].Num1 && v.Num2 == a.Arr[idx].Num2 && v.Str1 == a.Arr[idx].Str1 && v.Str2 == a.Arr[idx].Str2 && v.Str3 == a.Arr[idx].Str3 {
			return
		}
	}
	a.Arr = append(a.Arr, v)
}

// SlicesCacheContainsI checks if string v exists in the cached string array
// identified by s using a case-insensitive comparison. It gets the cached
// array, iterates over it, compares each element to v with EqualFold,
// and returns true if a match is found.
func SlicesCacheContainsI(s string, v string) bool {
	a := GetCachedTypeObjArr[string](s)
	if a == nil {
		return false
	}
	return logger.SlicesContainsI(a, v)
}

// SlicesCacheContains checks if the cached string array identified by s contains
// the value v. It iterates over the array and returns true if a match is found.
func SlicesCacheContains(s string, q *string) bool {
	a := GetCachedTypeObjArr[string](s)
	if a == nil {
		return false
	}
	v := *q
	for idx := range a {
		if v == a[idx] {
			return true
		}
	}
	return false
}

// SlicesCacheContainsDelete removes an element matching v from the cached string
// array identified by s. It acquires a lock, defers unlocking, and iterates
// over the array to find a match. When found, it uses slices.Delete to remove
// the element while preserving order, updates the cache, and returns.
func SlicesCacheContainsDelete(s string, v string) {
	a := GetCachedTypeObj[string](s)
	if a == nil {
		return
	}
	idx := getcacheidx(a, v)
	if idx != -1 {
		a.Arr = slices.Delete(a.Arr, idx, idx+1)
	}
}

// getcachestringidx searches for string v in the string array a.Arr.
// It returns the index of v in a.Arr if found, otherwise -1.
func getcacheidx[T comparable](a *cacheTypeExpire[T], v T) int {
	for idx := range a.Arr {
		if v == a.Arr[idx] {
			return idx
		}
	}
	return -1
}

// CacheOneStringTwoIntIndexFunc looks up the cached one string two int array
// identified by s and calls the passed in function f on each element.
// Returns true if f returns true for any element.
func CacheOneStringTwoIntIndexFunc(s string, f func(DbstaticOneStringTwoInt) bool) bool {
	a := GetCachedTypeObjArr[DbstaticOneStringTwoInt](s)
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
func CacheOneStringTwoIntIndexFuncRet(s string, id int, listname string) uint {
	a := GetCachedTypeObjArr[DbstaticOneStringTwoInt](s)
	if a == nil {
		return 0
	}
	for idx := range a {
		if a[idx].Num1 == id && strings.EqualFold(a[idx].Str, listname) {
			return uint(a[idx].Num2)
		}
	}
	return 0
}

// CacheOneStringTwoIntIndexFuncStr looks up the cached one string, two int array
// identified by s and returns the string value for the entry where the
// second int matches the passed in uint i. It stores the returned string in
// listname. If no match is found, it sets listname to an empty string.
func CacheOneStringTwoIntIndexFuncStr(s string, i uint) string {
	j := int(i)
	a := GetCachedTypeObjArr[DbstaticOneStringTwoInt](s)
	if a == nil {
		return ""
	}
	for idx := range a {
		if a[idx].Num2 == j {
			return a[idx].Str
		}
	}
	return ""
}

// CacheTwoStringIntIndexFunc searches the cached two string int array identified by s
// to find an entry matching str. If usestr1 is true, it matches on Str1, otherwise
// it matches on Str2. If a match is found, it sets m.DbserieID to the Num value
// for the matching entry and returns.
func CacheTwoStringIntIndexFunc(s string, usestr1 bool, str *string, m *ParseInfo) {
	a := GetCachedTypeObjArr[DbstaticTwoStringOneInt](s)
	if a == nil {
		return
	}
	t := *str
	for idx := range a {
		if usestr1 && strings.EqualFold(a[idx].Str1, t) {
			m.DbserieID = uint(a[idx].Num)
			return
		}
		if !usestr1 && strings.EqualFold(a[idx].Str2, t) {
			m.DbserieID = uint(a[idx].Num)
			return
		}
	}
}

// CacheThreeStringIntIndexFunc looks up the cached three string, two int array
// identified by s and returns the second int value for the entry where the
// third string matches the string str. Returns 0 if no match found.
func CacheThreeStringIntIndexFunc(s string, str *string) uint {
	if str == nil || *str == "" {
		return 0
	}
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s)
	if a == nil {
		return 0
	}
	t := *str
	for idx := range a {
		if a[idx].Str3 == t || strings.EqualFold(a[idx].Str3, t) {
			return uint(a[idx].Num2)
		}
	}
	return 0
}

// CacheThreeStringIntIndexFuncGetImdb searches the cached three string two int array
// identified by s and returns the imdb ID string for the entry where Num2 matches
// the passed in integer str. It stores the returned string in m.Imdb.
func CacheThreeStringIntIndexFuncGetImdb(s string, str int, m *ParseInfo) {
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s)
	if a == nil {
		return
	}
	for idx := range a {
		if a[idx].Num2 == str {
			m.Imdb = a[idx].Str3
			return
		}
	}
}

// CacheThreeStringIntIndexFuncGetYear looks up the cached three string, two int array
// identified by s and returns the first int value for the entry where the second int
// matches the int str. Returns 0 if no match found.
func CacheThreeStringIntIndexFuncGetYear(s string, str int) int {
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s)
	if a == nil {
		return 0
	}
	for idx := range a {
		if a[idx].Num2 == str {
			return a[idx].Num1
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
	if config.SettingsGeneral.CacheDuration == 0 {
		config.SettingsGeneral.CacheDuration = 12
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	var doupdate bool
	if useseries {
		item := GetCachedTypeObj[DbstaticTwoStringOneInt](logger.GetStringsMap(useseries, logger.CacheDBMedia))
		if item != nil {
			if len(item.Arr) == GetdatarowN[int](false, "select count() from dbseries") {
				if config.SettingsGeneral.CacheAutoExtend {
					item.expires = time.Now().Add(dur).UnixNano()
				}
			} else {
				doupdate = true
			}
		} else {
			doupdate = true
		}
	} else {
		item := GetCachedTypeObj[DbstaticThreeStringTwoInt](logger.GetStringsMap(useseries, logger.CacheDBMedia))
		if item != nil {
			if len(item.Arr) == GetdatarowN[int](false, "select count() from dbmovies") {
				if config.SettingsGeneral.CacheAutoExtend {
					item.expires = time.Now().Add(dur).UnixNano()
				}
			} else {
				doupdate = true
			}
		} else {
			doupdate = true
		}
	}
	item := GetCachedTypeObj[DbstaticOneStringTwoInt](logger.GetStringsMap(useseries, logger.CacheMedia))
	if item != nil {
		if len(item.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountMedia)) {
			if config.SettingsGeneral.CacheAutoExtend {
				item.expires = time.Now().Add(dur).UnixNano()
			} else {
				doupdate = true
			}
		} else {
			doupdate = true
		}
	}
	if !doupdate {
		return
	}
	if item != nil && item.lastscan.After(time.Now().Add(-1*time.Minute)) {
		return
	}
	logger.LogDynamic("debug", "refresh media cache")
	if !useseries {
		SetCachedObj(logger.CacheDBMovie, &cacheTypeExpire[DbstaticThreeStringTwoInt]{
			Arr: GetrowsN[DbstaticThreeStringTwoInt](false, GetdatarowN[int](false, "select count() from dbmovies")+100,
				"select title, slug, imdb_id, year, id from dbmovies"),
			expires:  time.Now().Add(dur).UnixNano(),
			lastscan: time.Now(),
		})
	} else {
		SetCachedObj(logger.CacheDBSeries, &cacheTypeExpire[DbstaticTwoStringOneInt]{
			Arr: GetrowsN[DbstaticTwoStringOneInt](false, GetdatarowN[int](false, "select count() from dbseries")+100,
				"select seriename, slug, id from dbseries"),
			expires:  time.Now().Add(dur).UnixNano(),
			lastscan: time.Now(),
		})
	}
	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheMedia), &cacheTypeExpire[DbstaticOneStringTwoInt]{
		Arr: GetrowsN[DbstaticOneStringTwoInt](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountMedia))+100,
			logger.GetStringsMap(useseries, logger.DBCacheMedia)),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})
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
	if config.SettingsGeneral.CacheDuration == 0 {
		config.SettingsGeneral.CacheDuration = 12
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	var doupdate bool
	item := GetCachedTypeObj[string](logger.GetStringsMap(useseries, logger.CacheHistoryTitle))
	if item != nil {
		if len(item.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesTitle)) {
			if config.SettingsGeneral.CacheAutoExtend {
				item.expires = time.Now().Add(dur).UnixNano()
			}
		} else {
			doupdate = true
		}
	} else {
		doupdate = true
	}

	item2 := GetCachedTypeObj[string](logger.GetStringsMap(useseries, logger.CacheHistoryUrl))
	if item2 != nil {
		if len(item2.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesUrl)) {
			if config.SettingsGeneral.CacheAutoExtend {
				item2.expires = time.Now().Add(dur).UnixNano()
			}
		} else {
			doupdate = true
		}
	} else {
		doupdate = true
	}
	if !doupdate {
		return
	}
	if item != nil && item.lastscan.After(time.Now().Add(-1*time.Minute)) {
		return
	}
	logger.LogDynamic("debug", "refresh history cache")
	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheHistoryTitle), &cacheTypeExpire[string]{
		Arr: GetrowsN[string](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesTitle))+100,
			logger.GetStringsMap(useseries, logger.DBHistoriesTitle)),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})

	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheHistoryUrl), &cacheTypeExpire[string]{
		Arr: GetrowsN[string](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesUrl))+100,
			logger.GetStringsMap(useseries, logger.DBHistoriesUrl)),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})
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
	if config.SettingsGeneral.CacheDuration == 0 {
		config.SettingsGeneral.CacheDuration = 12
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	item := GetCachedTypeObj[DbstaticTwoStringOneInt](logger.GetStringsMap(useseries, logger.CacheMediaTitles))
	if item != nil {
		if len(item.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountDBTitles)) {
			if config.SettingsGeneral.CacheAutoExtend {
				item.expires = time.Now().Add(dur).UnixNano()
			}
			return
		}
	}
	if item != nil && item.lastscan.After(time.Now().Add(-1*time.Minute)) {
		return
	}
	logger.LogDynamic("debug", "refresh media titles cache")
	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheMediaTitles), &cacheTypeExpire[DbstaticTwoStringOneInt]{
		Arr: GetrowsN[DbstaticTwoStringOneInt](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountDBTitles))+100,
			logger.GetStringsMap(useseries, logger.DBCacheDBTitles)),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})
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
	if config.SettingsGeneral.CacheDuration == 0 {
		config.SettingsGeneral.CacheDuration = 12
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour

	item := GetCachedTypeObj[string](logger.GetStringsMap(useseries, logger.CacheFiles))
	if item != nil {
		if len(item.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountFiles)) {
			if config.SettingsGeneral.CacheAutoExtend {
				item.expires = time.Now().Add(dur).UnixNano()
			}
			return
		}
	}

	if item != nil && item.lastscan.After(time.Now().Add(-1*time.Minute)) {
		return
	}
	logger.LogDynamic("debug", "refresh files cache")
	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheFiles), &cacheTypeExpire[string]{
		Arr: GetrowsN[string](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountFiles))+300,
			logger.GetStringsMap(useseries, logger.DBCacheFiles)),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})
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

	if config.SettingsGeneral.CacheDuration == 0 {
		config.SettingsGeneral.CacheDuration = 12
	}
	dur := time.Duration(config.SettingsGeneral.CacheDuration) * time.Hour
	//ti := sql.NullTime{Time: logger.TimeGetNow().Add(-dur), Valid: true}
	ti := logger.TimeGetNow().Add(-dur)

	item := GetCachedTypeObj[string](logger.GetStringsMap(useseries, logger.CacheUnmatched))
	if item != nil {
		if len(item.Arr) == GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountUnmatched), &ti) {
			if config.SettingsGeneral.CacheAutoExtend {
				item.expires = time.Now().Add(dur).UnixNano()
			}
			return
		}
	}
	if item != nil && item.lastscan.After(time.Now().Add(-1*time.Minute)) {
		return
	}
	logger.LogDynamic("debug", "refresh unmatched cache")
	SetCachedObj(logger.GetStringsMap(useseries, logger.CacheUnmatched), &cacheTypeExpire[string]{
		Arr: GetrowsN[string](false, GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountUnmatched), &ti)+300,
			logger.GetStringsMap(useseries, logger.DBCacheUnmatched), &ti),
		expires:  time.Now().Add(dur).UnixNano(),
		lastscan: time.Now(),
	})
}

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

func GetCachedTypeObj[T any](key string) *cacheTypeExpire[T] {
	data, exists := cache.items.Load(key)
	if !exists {
		return nil
	}
	item, ok := data.(*cacheTypeExpire[T])
	if !ok || !CheckcacheexpireDataType(item) {
		return nil
	}
	return item
}
func GetCachedTypeObjArr[T any](key string) []T {
	i := GetCachedTypeObj[T](key)
	if i == nil {
		return nil
	}
	return i.Arr
}

// CheckcacheexpireData checks if the given cached data has expired based
// on its internal timestamp. Returns false if the cache entry has expired.
func CheckcacheexpireDataType[T any](a *cacheTypeExpire[T]) bool {
	if a == nil || (a.expires != 0 && a.expires < time.Now().UnixNano()) {
		return false
	}
	return true
}

// SetCachedObj stores the given value in the cache under the given key.
// If the key already exists, the existing value is deleted first.
func SetCachedObj(key string, val any) {
	item, ok := cache.items.Load(key)
	if ok {
		cachedelete(key, item)
	}
	cache.items.Store(key, val)
}

// CheckcachedMovieTitleHistory checks if the given movie title exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the title exists, false
// otherwise.
func CheckcachedTitleHistory(useseries bool, file string) bool {
	if file == "" {
		return false
	}
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(logger.GetStringsMap(useseries, logger.CacheHistoryTitle), file)
	}
	return GetdatarowN[uint](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesByTitle), file) >= 1
}

// CheckcachedMovieUrlHistory checks if the given movie URL exists in the
// movie_histories table. It first checks the file cache if enabled,
// otherwise queries the database. Returns true if the URL exists, false
// otherwise.
func CheckcachedUrlHistory(useseries bool, file string) bool {
	if file == "" {
		return false
	}
	if config.SettingsGeneral.UseFileCache {
		return SlicesCacheContainsI(logger.GetStringsMap(useseries, logger.CacheHistoryUrl), file)
	}
	return GetdatarowN[uint](false, logger.GetStringsMap(useseries, logger.DBCountHistoriesByUrl), file) >= 1
}

// globalcache is a struct that stores configuration values for the cache
// including the default cache expiration time and a logger instance.
type globalcache struct {
	// defaultextension is the default cache expiration duration
	defaultextension time.Duration
	// log is a logger instance for logging cache events
	log *zerolog.Logger
}

// itemregex is a struct that stores a compiled regular expression and its expiration time
// expires is the expiration time for the cached regex
// value is the compiled regular expression
type itemregex struct {
	expires int64
	value   *regexp.Regexp
}

// itemstmt stores a cached prepared statement, expiration time,
// and whether it is for the imdb database.
type itemstmt struct {
	// expires is the expiration time for the cached statement
	expires int64
	// imdb indicates if the statement is for the imdb database
	imdb bool
	// value is the prepared statement
	value *sqlx.Stmt
}

const (
	regexSeriesTitle      = `^(.*)(?i)(?:(?:\.| - |-)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$))`
	regexSeriesTitleDate  = `^(.*)(?i)(?:\.|-| |_)(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$)`
	regexSeriesIdentifier = `(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`
)

// cache is a struct that stores the cache items map,
// a janitor timer, and an interval duration
// items is a sync.Map that stores cached items
// janitor is a timer that periodically cleans up expired cache items
// interval is the duration between janitor cleanups
var cache = struct {
	items    sync.Map
	janitor  *time.Timer
	interval time.Duration
}{
	items:    sync.Map{},      // Initialize empty sync.Map
	interval: 5 * time.Minute, // Set default interval to 5 minutes
}

// StartJanitor starts the background cache janitor goroutine
func startJanitor() {
	cache.janitor = time.NewTimer(cache.interval)
	go func() {
		for {
			<-cache.janitor.C
			cache.items.Range(func(key, value any) bool {
				cachedeleteexpire(key, value)

				return true
			})
			cache.janitor.Reset(cache.interval)
		}
	}()
}

// cachedeleteexpire removes expired items from the global statement
// cache by closing any open database statements and setting cache
// values to nil if they have expired based on the expiration time.
// It checks each cache item type for nil or expired values before
// deleting the key from the cache.
func cachedeleteexpire(key, value any) {
	now := time.Now().UnixNano()
	switch item := value.(type) {
	case *itemstmt:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[DbstaticOneStringOneInt]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[DbstaticOneStringTwoInt]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[DbstaticThreeStringTwoInt]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[string]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[DbstaticTwoInt]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *cacheTypeExpire[DbstaticTwoStringOneInt]:
		if item.expires == 0 || now < item.expires {
			return
		}
	case *itemregex:
		if item.expires == 0 || now < item.expires {
			return
		}
	}
	cachedelete(key, value)
}

// getexpireskey returns the expiration time in nanoseconds for the given cache key
// and duration. Special cases like 0 duration or predefined keys will not expire.
func getexpireskey(duration time.Duration, key string) int64 {
	if duration == 0 || key == "RegexSeriesIdentifier" || key == "RegexSeriesTitle" {
		return 0
	}
	return time.Now().Add(duration).UnixNano()
}

// NewRegex creates a new GlobalcacheRegex instance.
// cleaningInterval specifies the interval to clean up expired regex entries.
// extension specifies the default expiration duration to use for cached regexes.
// log is the logger used for logging.
// It initializes the cache and starts a goroutine to clean up expired entries
// based on the cleaningInterval.
func NewCache(cleaningInterval time.Duration, extension time.Duration, log *zerolog.Logger) *globalcache {
	c := globalcache{
		defaultextension: extension,
		log:              log,
	}

	if cleaningInterval >= 1 {
		startJanitor()
	}

	return &c
}

// GetRegexpDirect retrieves the regular expression for the given key from the cache.
// If it does not exist, it is created using getregex().
// It also handles expiry and clearing of the cached regex if needed.
func (c *globalcache) GetRegexpDirect(key string) *regexp.Regexp {
	return c.SetRegexp(key, c.defaultextension)
}

func InvalidateImdbStmt() {
	cache.items.Range(func(key, value any) bool {
		if item, ok := value.(*itemstmt); ok {
			if item.imdb {
				cachedelete(key, value)
			}
		}
		return true
	})
}

// set caches a prepared statement for the given key in the global cache.
// It prepares the statement, stores it in the cache with an expiry time,
// and returns the prepared statement.
func (c *globalcache) setsql(key string, imdb bool, db *sqlx.DB) *sqlx.Stmt {
	//dont lock cache since it may already be locked
	sq, err := db.Preparex(key)
	if err != nil {
		return nil
	}
	SetCachedObj(key, &itemstmt{
		value:   sq,
		imdb:    imdb,
		expires: c.getexpiressql(),
	})
	return sq
}

// getexpiressql returns the expiry time in nanoseconds for cached statements.
// It checks the defaultextension field, and if greater than 0, sets the expiry to that duration from the current time.
// Otherwise it returns 0 indicating no expiry.
func (c *globalcache) getexpiressql() int64 {
	if c.defaultextension > 0 {
		return time.Now().Add(c.defaultextension).UnixNano()
	}
	return 0
}

// MapLoadP loads a value for the given key from the sync.Map m.
// It returns a pointer to the value and a bool indicating if it was found.
// This allows convenient loading of typed values from a sync.Map.
// The Type needs to be stored in the sync.Map as a pointer.
func MapLoadP[T any](m *sync.Map, key string) (*T, bool) {
	data, exists := m.Load(key)
	if !exists {
		return nil, false
	}
	item, ok := data.(*T)
	if !ok {
		return nil, false
	}
	return item, true
}

// GetStmt retrieves a prepared statement from the cache, or prepares it if not cached.
// It locks access during retrieval and update.
// Key is the statement text.
// Db is the database connection.
// It returns the prepared statement.
func (c *globalcache) GetStmt(key string, imdb bool, db *sqlx.DB) *sqlx.Stmt {
	item, ok := MapLoadP[itemstmt](&cache.items, key)
	if !ok || item == nil || item.value == nil {
		return c.setsql(key, imdb, db)
	}
	if item.expires != 0 && time.Now().UnixNano() > item.expires {
		cachedeleteexpire(key, item)
		return c.setsql(key, imdb, db)
	}
	if item.expires != 0 {
		if config.SettingsGeneral.CacheAutoExtend {
			item.expires = c.getexpiressql()
		}
	}
	//cache.items.Store(key, item)
	return item.value
}

// set caches the given regular expression for the specified key and duration.
// It first compiles the regex based on pre-defined keys, or the key itself.
// It then stores the regex and expiration in the cache.
// Returns the compiled regex.
func (c *globalcache) set(key string, dur time.Duration) *regexp.Regexp {
	regex := getregex(key)
	if regex == nil {
		return nil
	}

	if dur == 0 {
		dur = c.defaultextension
	}
	SetCachedObj(key, &itemregex{
		value:   regex,
		expires: getexpireskey(dur, key),
	})
	return regex
}

// getregex returns a compiled regular expression for the given key.
// It has some predefined keys that map to common regexes.
// Otherwise it compiles the key directly as the regex pattern.
func getregex(key string) *regexp.Regexp {
	switch key {
	case "RegexSeriesTitle":
		return regexp.MustCompile(regexSeriesTitle)
	case "RegexSeriesTitleDate":
		return regexp.MustCompile(regexSeriesTitleDate)
	case "RegexSeriesIdentifier":
		return regexp.MustCompile(regexSeriesIdentifier)
	default:
		return regexp.MustCompile(key)
	}
}

// SetRegexp sets a regular expression in the cache for the given key, to expire after the given duration.
// If the key already exists, the existing regex is cleared and replaced with the new one.
func (c *globalcache) SetRegexp(key string, duration time.Duration) *regexp.Regexp {
	item, ok := MapLoadP[itemregex](&cache.items, key)
	if !ok || item == nil || item.value == nil {
		return c.set(key, duration)
	}

	if item.expires != 0 && time.Now().UnixNano() > item.expires {
		cachedeleteexpire(key, item)
		return c.set(key, duration)
	}
	if item.expires != 0 {
		if config.SettingsGeneral.CacheAutoExtend {
			item.expires = getexpireskey(c.defaultextension, key)
		}
	}
	return item.value
}

// Getfirstsubmatchindex returns the indexes of the first submatch found
// in matchfor using the compiled regular expression stored in cache
// under key.
func Getfirstsubmatchindex(key string, matchfor string) []int {
	return GlobalCache.GetRegexpDirect(key).FindStringSubmatchIndex(matchfor)
}

// getmatches returns the indexes of the first submatch found in matchfor
// using the compiled regular expression stored in cache under key. If cached
// is false, it will compile the regex and find matches without caching.
func getmatches(cached bool, key string, matchfor string) []int {
	if !cached {
		return regexp.MustCompile(key).FindStringSubmatchIndex(matchfor)
	}
	//return GlobalCache.GetRegexpDirect(key).FindStringSubmatchIndex(matchfor)
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
	re := GlobalCache.GetRegexpDirect(key)
	if mincount == 1 {
		return len(re.FindStringIndex(matchfor)) >= 1
	}
	return len(re.FindAllStringIndex(matchfor, mincount)) >= mincount
}
