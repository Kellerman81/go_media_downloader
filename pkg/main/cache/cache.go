package cache

import (
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Cache stores arbitrary data with expiration time.
type Cache struct {
	items       map[string]int64
	itemsstatic map[string]interface{}
	mu          *sync.Mutex
	close       chan struct{}
}
type CacheRegex struct {
	items       map[string]int64
	itemsstatic map[string]regexp.Regexp
	mu          *sync.Mutex
	close       chan struct{}
}
type CacheStmt struct {
	items       map[string]int64
	itemsstatic map[string]sqlx.Stmt
	mu          *sync.Mutex
	close       chan struct{}
}
type CacheStmtNamed struct {
	items       map[string]int64
	itemsstatic map[string]sqlx.NamedStmt
	mu          *sync.Mutex
	close       chan struct{}
}

// New creates a new cache that asynchronously cleans
// expired entries after the given time passes.
func New(cleaningInterval time.Duration) *Cache {
	cache := &Cache{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]interface{}, 100),
		mu:          &sync.Mutex{},
		close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					now := time.Now().UnixNano()

					for key := range cache.items {
						if cache.items[key] != 0 && now > cache.items[key] {
							delete(cache.items, key)
							delete(cache.itemsstatic, key)
						}
					}

				case <-cache.close:
					return
				}
			}
		}()
	}

	return cache
}

type CacheReturn struct {
	Value interface{}
}

func (cache *CacheReturn) Close() {
	if cache != nil {
		cache = nil
	}
}

// Get gets the value for the given key.
func (cache *Cache) Get(key string) (*CacheReturn, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}
	}

	data, exists := cache.itemsstatic[key]
	if exists {
		return &CacheReturn{Value: data}, true
	}
	return nil, false
}

// Get gets the value for the given key.
func (cache *Cache) Check(key string, kind reflect.Type) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}

		data, exists := cache.itemsstatic[key]

		if exists {
			if reflect.TypeOf(data) != kind {
				exists = false
			}
		}
		return exists
	} else {
		_, exists = cache.itemsstatic[key]
		return exists
	}
}

// Get gets the value for the given key.
func (cache *Cache) CheckNoType(key string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}

		_, exists := cache.itemsstatic[key]
		return exists
	} else {
		_, exists = cache.itemsstatic[key]
		return exists
	}
}
func (cache *Cache) GetData(key string) *CacheReturn {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return &CacheReturn{Value: cache.itemsstatic[key]}
}

func (cache *Cache) GetAll() map[string]interface{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	ret := make(map[string]interface{})
	for key := range cache.items {
		if cache.items[key] != 0 && time.Now().UnixNano() > cache.items[key] {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}
	}
	for key := range cache.itemsstatic {
		ret[key] = cache.itemsstatic[key]
	}
	return ret
}
func (cache *Cache) GetPrefix(prefix string) map[string]interface{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	ret := make(map[string]interface{})
	for key := range cache.items {
		if strings.HasPrefix(key, prefix) {
			if cache.items[key] != 0 && time.Now().UnixNano() > cache.items[key] {
				delete(cache.items, key)
				delete(cache.itemsstatic, key)
			}
		}
	}
	for key := range cache.itemsstatic {
		if strings.HasPrefix(key, prefix) {
			ret[key] = cache.itemsstatic[key]
		}
	}
	return ret
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (cache *Cache) Set(key string, value interface{}, duration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if duration > 0 {
		cache.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(cache.items, key)
	}
	cache.itemsstatic[key] = value
}

// Delete deletes the key and its value from the cache.
func (cache *Cache) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.items, key)
	delete(cache.itemsstatic, key)
}

// Close closes the cache and frees up resources.
func (cache *Cache) Close() {
	cache.items = map[string]int64{}
	cache.itemsstatic = map[string]interface{}{}
}

// New Regex Cache
func NewRegex(cleaningInterval time.Duration) *CacheRegex {
	cache := &CacheRegex{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]regexp.Regexp, 100),
		mu:          &sync.Mutex{},
		close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					for key := range cache.items {
						cache.clearexpired(key)
					}

				case <-cache.close:
					return
				}
			}
		}()
	}
	return cache
}
func (cache *CacheRegex) clearexpired(key string) {
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
			return
		}
	}
	item2, exists2 := cache.itemsstatic[key]
	if exists2 {
		if item2.String() == "" {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}
	}
}
func (cache *CacheStmt) clearexpired(key string) {
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
			return
		}
	}
	item2, exists2 := cache.itemsstatic[key]
	if exists2 {
		if item2.Stmt == nil {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}
	}
}
func (cache *CacheStmtNamed) clearexpired(key string) {
	item, exists := cache.items[key]
	if exists {
		if item != 0 && time.Now().UnixNano() > item {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
			return
		}
	}
	item2, exists2 := cache.itemsstatic[key]
	if exists2 {
		if item2.Stmt == nil {
			delete(cache.items, key)
			delete(cache.itemsstatic, key)
		}
	}
}
func (cache *CacheRegex) CheckRegexp(key string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.clearexpired(key)

	_, exists := cache.itemsstatic[key]
	return exists
}

func (cache *CacheRegex) getregex(key string) *regexp.Regexp {

	regexstr := ""
	var expires int64
	switch key {
	case "RegexSeriesTitle":
		regexstr = `^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`
	case "RegexSeriesIdentifier":
		regexstr = `(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`
	default:
		regexstr = key
		expires = time.Now().Add(20 * time.Minute).UnixNano()
	}
	setdata, err := regexp.Compile(regexstr)
	if err == nil {
		if expires != 0 {
			cache.items[key] = expires
		}
		cache.itemsstatic[key] = *setdata
		return setdata
	}
	return nil
}
func (cache *CacheRegex) GetRegexpDirect(key string) *regexp.Regexp {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.clearexpired(key)

	val, exists := cache.itemsstatic[key]
	if exists {
		return &val
	}
	return cache.getregex(key)
}
func (cache *CacheRegex) SetRegexp(key string, value string, duration time.Duration) {
	reg, err := regexp.Compile(value)
	if err != nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if duration > 0 {
		cache.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(cache.items, key)
	}
	cache.itemsstatic[key] = *reg
	reg = nil
}

// Delete deletes the key and its value from the cache.
func (cache *CacheRegex) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.items, key)
	delete(cache.itemsstatic, key)
}

// Close closes the cache and frees up resources.
func (cache *CacheRegex) Close() {
	cache.items = map[string]int64{}
	cache.itemsstatic = map[string]regexp.Regexp{}
}

// New SQLx Stmt Cache
func NewStmt(cleaningInterval time.Duration) *CacheStmt {
	cache := &CacheStmt{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]sqlx.Stmt, 100),
		mu:          &sync.Mutex{},
		close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					for key := range cache.items {
						cache.clearexpired(key)
					}

				case <-cache.close:
					return
				}
			}
		}()
	}
	return cache
}

// Get gets the value for the given key.
func (cache *CacheStmt) Check(key string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.clearexpired(key)
	_, exists := cache.items[key]
	if exists {
		_, exists = cache.itemsstatic[key]

		return exists
	} else {
		return false
	}
}

func ClearVar(i interface{}) {
	v := reflect.ValueOf(i)
	if !v.IsZero() && v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	}
}
func (cache *CacheStmt) GetData(key string) *sqlx.Stmt {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	val, exists := cache.itemsstatic[key]
	if exists {
		return &val
	}
	return nil
}
func (cache *CacheStmt) SetStmt(key string, value *sqlx.Stmt, duration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if duration > 0 {
		cache.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(cache.items, key)
	}
	cache.itemsstatic[key] = *value
}

// Delete deletes the key and its value from the cache.
func (cache *CacheStmt) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.items, key)
	delete(cache.itemsstatic, key)
}

// Close closes the cache and frees up resources.
func (cache *CacheStmt) Close() {
	cache.items = map[string]int64{}
	cache.itemsstatic = map[string]sqlx.Stmt{}
}

// New SQLx NamedStmt Cache
func NewStmtNamed(cleaningInterval time.Duration) *CacheStmtNamed {
	cache := &CacheStmtNamed{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]sqlx.NamedStmt, 100),
		mu:          &sync.Mutex{},
		close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					for key := range cache.items {
						cache.clearexpired(key)
					}

				case <-cache.close:
					return
				}
			}
		}()
	}
	return cache
}

// Get gets the value for the given key.
func (cache *CacheStmtNamed) Check(key string) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.clearexpired(key)
	_, exists := cache.items[key]
	if exists {
		_, exists = cache.itemsstatic[key]

		return exists
	} else {
		return false
	}
}

func (cache *CacheStmtNamed) GetData(key string) *sqlx.NamedStmt {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	val, exists := cache.itemsstatic[key]
	if exists {
		return &val
	}
	return nil
}
func (cache *CacheStmtNamed) SetNamedStmt(key string, value *sqlx.NamedStmt, duration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if duration > 0 {
		cache.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(cache.items, key)
	}

	cache.itemsstatic[key] = *value
}

// Delete deletes the key and its value from the cache.
func (cache *CacheStmtNamed) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.items, key)
	delete(cache.itemsstatic, key)
}

// Close closes the cache and frees up resources.
func (cache *CacheStmtNamed) Close() {
	cache.items = map[string]int64{}
	cache.itemsstatic = map[string]sqlx.NamedStmt{}
}
