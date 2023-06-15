package cache

import (
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Cache stores arbitrary data with expiration time.
type Globalcache struct {
	//items       map[string]int64
	itemsstatic      map[string]item
	mu               *sync.Mutex
	defaultextension time.Duration
	//close       chan struct{}
}
type item struct {
	expires int64
	value   interface{}
}

var (
	regexSeriesTitle      string = `^(.*)(?i)(?:(?:\.| - |-)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:[^0-9]|$))`
	regexSeriesIdentifier string = `(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex-][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`
)

// New creates a new c that asynchronously cleans
// expired entries after the given time passes.
func New(cleaningInterval time.Duration, extension time.Duration) *Globalcache {
	c := Globalcache{
		//items:       make(map[string]int64, 100),
		itemsstatic:      make(map[string]item, 100),
		mu:               &sync.Mutex{},
		defaultextension: extension,
		//close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for range ticker.C {
				c.clearexpired()
			}
		}()
	}

	return &c
}

// Get gets the value for the given key.
func (c *Globalcache) Get(key string) (interface{}, bool) {
	c.clearexpiredkey(key)
	if !c.CheckNoType(key) {
		return nil, false
	}
	data, exists := c.getitem(key)
	return data.value, exists
}

// Get gets the value for the given key.
func (c *Globalcache) Check(key string, kind reflect.Type) bool {
	data, exists := c.getitem(key)
	if !exists || data.value == nil {
		c.Delete(key)
		return false
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		c.Delete(key)
		return false
	}
	if reflect.TypeOf(data) != kind {
		exists = false
	}
	return exists
}

// Get gets the value for the given key.
func (c *Globalcache) CheckNoType(key string) bool {
	data, exists := c.getitem(key)
	if !exists || data.value == nil {
		c.Delete(key)
		return false
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		c.Delete(key)
		return false
	}
	return exists
}

// Get gets the value for the given key.
func (c *Globalcache) CheckStringArrValue(key string, value string) bool {
	return c.GetIndexStrings(key, value) != -1
}

// Get gets the value for the given key.
func CheckFunc[T any](c *Globalcache, key string, fun func(elem T) bool) bool {
	return GetFuncNil(c, key, fun) != nil
}

// Get gets the value for the given key.
func GetFunc[T any](c *Globalcache, key string, fun func(elem T) bool) *T {
	d := GetFuncNil(c, key, fun)
	if d == nil {
		return new(T)
	}
	return d
}

// Get gets the value for the given key.
func GetFuncNil[T any](c *Globalcache, key string, fun func(elem T) bool) *T {
	data, exists := c.getitem(key)
	if exists {
		val := data.value.(*[]T)
		for idx := range *val {
			if fun((*val)[idx]) {
				return &(*val)[idx]
			}
		}
	}
	return nil
}

// Searches in cached string slice
func (c *Globalcache) GetIndexStrings(key string, search string) int {
	//GetFuncNil(c, key, func(elem string) bool { return elem == search })
	data, exists := c.getitem(key)
	if exists {
		val := data.value.(*[]string)
		for idx := range *val {
			if (*val)[idx] == search {
				return idx
			}
		}
	}
	return -1
}

// Get gets the value for the given key.
func GetAllFunc[T any](c *Globalcache, key string, fun func(elem T) bool) *[]T {
	data, exists := c.getitem(key)
	if exists {
		val := data.value.(*[]T)
		u := (*val)[:0]
		for idx := range *val {
			if fun((*val)[idx]) {
				u = append(u, (*val)[idx])
			}
		}
		return &u
	}
	return nil
}

// Auto Extends Expiration
func GetDataT[T any](c *Globalcache, key string) T {
	data, _ := c.getitem(key)
	dur := getexpires(c.defaultextension)
	if data.expires != 0 && data.expires < dur {
		c.mu.Lock()
		data.expires = dur
		c.itemsstatic[key] = data
		c.mu.Unlock()
	}
	return data.value.(T)
}

func (c *Globalcache) GetData(key string) interface{} {
	data, _ := c.getitem(key)
	return data.value
}

func (c *Globalcache) GetAll() map[string]interface{} {
	ret := make(map[string]interface{})
	c.clearexpired()
	c.mu.Lock()
	for key := range c.itemsstatic {
		ret[key] = c.itemsstatic[key].value
	}
	c.mu.Unlock()
	return ret
}
func (c *Globalcache) GetPrefix(prefix string) map[string]interface{} {
	ret := make(map[string]interface{})
	c.clearexpired()
	c.mu.Lock()
	for key := range c.itemsstatic {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			ret[key] = c.itemsstatic[key].value
		}
	}
	c.mu.Unlock()
	return ret
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (c *Globalcache) Set(key string, value interface{}, duration time.Duration, keepduration bool) {
	if keepduration {
		data, exists := c.getitem(key)
		if exists {
			c.mu.Lock()
			data.value = value
			c.itemsstatic[key] = data
			c.mu.Unlock()
			return
		}
	}
	c.mu.Lock()
	c.itemsstatic[key] = item{value: value, expires: getexpires(duration)}
	c.mu.Unlock()
}

func getexpires(duration time.Duration) int64 {
	if duration > 0 {
		return time.Now().Add(duration).UnixNano()
	}
	return 0
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func SetTable[T any](c *Globalcache, key string, value *[]T, duration time.Duration) {
	c.mu.Lock()
	c.itemsstatic[key] = item{value: value, expires: getexpires(duration)}
	c.mu.Unlock()
}

// Append to any slice in cache
func Append[T any](c *Globalcache, key string, value T) {
	if !c.CheckNoType(key) {
		return
	}
	data, exists := c.getitem(key)

	if exists {
		val := data.value.(*[]T)
		*val = append(*val, value)
		data.value = val
		c.mu.Lock()
		c.itemsstatic[key] = data
		c.mu.Unlock()
	}
}

// Delete from string slice in cache
func (c *Globalcache) DeleteString(key string, search string) {
	if !c.CheckNoType(key) {
		return
	}
	data, exists := c.getitem(key)
	if exists {
		val := data.value.(*[]string)
		for idx := range *val {
			if (*val)[idx] == search {
				c.mu.Lock()
				(*val)[idx] = (*val)[len((*val))-1] //Replace found entry with last
				x := (*val)[:len((*val))-1]
				data.value = &x
				c.itemsstatic[key] = data
				c.mu.Unlock()
				return
			}
		}
	}
}

// Delete deletes the key and its value from the c.
func (c *Globalcache) Delete(key string) {
	c.mu.Lock()
	delete(c.itemsstatic, key)
	c.mu.Unlock()
}

func (c *Globalcache) clearexpired() {
	c.mu.Lock()

	now := time.Now().UnixNano()
	for key := range c.itemsstatic {
		if c.itemsstatic[key].expires != 0 && now > c.itemsstatic[key].expires {
			delete(c.itemsstatic, key)
		} else if c.itemsstatic[key].value == nil {
			delete(c.itemsstatic, key)
		}
	}

	c.mu.Unlock()
}

func (c *Globalcache) clearexpiredkey(key string) {
	data, exists := c.getitem(key)
	if !exists || data.value == nil {
		c.Delete(key)
		return
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		c.Delete(key)
	}
}

func (c *Globalcache) getitem(key string) (item, bool) {
	c.mu.Lock()
	data, exists := c.itemsstatic[key]
	c.mu.Unlock()
	return data, exists
}

// Cache stores arbitrary data with expiration time.
type GlobalcacheStmt struct {
	//items       map[string]int64
	itemsstatic      map[string]itemstmt
	mu               *sync.Mutex
	defaultextension time.Duration
	//close       chan struct{}
}
type itemstmt struct {
	expires int64
	value   *sqlx.Stmt
}

// New creates a new c that asynchronously cleans
// expired entries after the given time passes.
func NewStmt(cleaningInterval time.Duration, extension time.Duration) *GlobalcacheStmt {
	c := GlobalcacheStmt{
		//items:       make(map[string]int64, 100),
		itemsstatic:      make(map[string]itemstmt, 100),
		mu:               &sync.Mutex{},
		defaultextension: extension,
		//close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			var now int64
			for range ticker.C {
				c.mu.Lock()
				now = time.Now().UnixNano()
				for key := range c.itemsstatic {
					if c.itemsstatic[key].expires != 0 && now > c.itemsstatic[key].expires {
						delete(c.itemsstatic, key)
					}
				}
				c.mu.Unlock()
			}
		}()
	}

	return &c
}

// Auto Extends Expiration
func (c *GlobalcacheStmt) GetData(key *string) *sqlx.Stmt {
	c.mu.Lock()
	data := c.itemsstatic[*key]
	dur := getexpires(c.defaultextension)
	if data.expires != 0 && data.expires < dur {
		data.expires = dur
		c.itemsstatic[*key] = data
	}
	c.mu.Unlock()
	return data.value
}

// Get gets the value for the given key.
func (c *GlobalcacheStmt) CheckNoType(key *string) bool {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if !exists {
		c.mu.Unlock()
		return false
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		delete(c.itemsstatic, *key)
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()
	return exists
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (c *GlobalcacheStmt) Set(key *string, value *sqlx.Stmt, duration time.Duration, keepduration bool) {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if keepduration && exists {
		data.value = value
		c.itemsstatic[*key] = data
		c.mu.Unlock()
		return
	}
	c.itemsstatic[*key] = itemstmt{value: value, expires: getexpires(duration)}
	c.mu.Unlock()
}

// Cache stores arbitrary data with expiration time.
type GlobalcacheNamed struct {
	//items       map[string]int64
	itemsstatic      map[string]itemnamed
	mu               *sync.Mutex
	defaultextension time.Duration
	//close       chan struct{}
}
type itemnamed struct {
	expires int64
	value   *sqlx.NamedStmt
}

// New creates a new c that asynchronously cleans
// expired entries after the given time passes.
func NewNamed(cleaningInterval time.Duration, extension time.Duration) *GlobalcacheNamed {
	c := GlobalcacheNamed{
		//items:       make(map[string]int64, 100),
		itemsstatic:      make(map[string]itemnamed, 100),
		mu:               &sync.Mutex{},
		defaultextension: extension,
		//close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			var now int64
			for range ticker.C {
				c.mu.Lock()
				now = time.Now().UnixNano()
				for key := range c.itemsstatic {
					if c.itemsstatic[key].expires != 0 && now > c.itemsstatic[key].expires {
						delete(c.itemsstatic, key)
					}
				}
				c.mu.Unlock()
			}
		}()
	}

	return &c
}

// Auto Extends Expiration
func (c *GlobalcacheNamed) GetData(key *string) *sqlx.NamedStmt {
	c.mu.Lock()
	data := c.itemsstatic[*key]
	dur := getexpires(c.defaultextension)
	if data.expires != 0 && data.expires < dur {
		data.expires = dur
		c.itemsstatic[*key] = data
	}
	c.mu.Unlock()
	return data.value
}

// Get gets the value for the given key.
func (c *GlobalcacheNamed) CheckNoType(key *string) bool {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if !exists {
		c.mu.Unlock()
		return false
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		delete(c.itemsstatic, *key)
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()
	return exists
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (c *GlobalcacheNamed) Set(key *string, value *sqlx.NamedStmt, duration time.Duration, keepduration bool) {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if keepduration && exists {
		data.value = value
		c.itemsstatic[*key] = data
		c.mu.Unlock()
		return
	}
	c.itemsstatic[*key] = itemnamed{value: value, expires: getexpires(duration)}
	c.mu.Unlock()
}

// Cache stores arbitrary data with expiration time.
type GlobalcacheRegex struct {
	//items       map[string]int64
	itemsstatic      map[string]itemregex
	mu               *sync.Mutex
	defaultextension time.Duration
	//close       chan struct{}
}
type itemregex struct {
	expires int64
	value   *regexp.Regexp
}

// New creates a new c that asynchronously cleans
// expired entries after the given time passes.
func NewRegex(cleaningInterval time.Duration, extension time.Duration) *GlobalcacheRegex {
	c := GlobalcacheRegex{
		//items:       make(map[string]int64, 100),
		itemsstatic:      make(map[string]itemregex, 100),
		mu:               &sync.Mutex{},
		defaultextension: extension,
		//close:       make(chan struct{}),
	}

	if cleaningInterval >= 1 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			var now int64
			for range ticker.C {
				c.mu.Lock()
				now = time.Now().UnixNano()
				for key := range c.itemsstatic {
					if c.itemsstatic[key].expires != 0 && now > c.itemsstatic[key].expires {
						delete(c.itemsstatic, key)
					}
				}
				c.mu.Unlock()
			}
		}()
	}

	return &c
}

// Auto Extends Expiration
func (c *GlobalcacheRegex) GetData(key *string) *regexp.Regexp {
	c.mu.Lock()
	data := c.itemsstatic[*key]
	dur := getexpires(c.defaultextension)
	if data.expires != 0 && data.expires < dur {
		data.expires = dur
		c.itemsstatic[*key] = data
	}
	c.mu.Unlock()
	return data.value
}

// Get gets the value for the given key.
func (c *GlobalcacheRegex) CheckNoType(key *string) bool {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if !exists {
		c.mu.Unlock()
		return false
	}
	if exists && data.expires != 0 && time.Now().UnixNano() > data.expires {
		delete(c.itemsstatic, *key)
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()
	return exists
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (c *GlobalcacheRegex) Set(key *string, value *regexp.Regexp, duration time.Duration, keepduration bool) {
	c.mu.Lock()
	data, exists := c.itemsstatic[*key]
	if keepduration && exists {
		data.value = value
		c.itemsstatic[*key] = data
		c.mu.Unlock()
		return
	}
	c.itemsstatic[*key] = itemregex{value: value, expires: getexpires(duration)}
	c.mu.Unlock()
}

func (c *GlobalcacheRegex) getregex(key *string) *regexp.Regexp {
	switch *key {
	case "RegexSeriesTitle":
		return c.setRegexp(key, &regexSeriesTitle, 0)
	case "RegexSeriesIdentifier":
		return c.setRegexp(key, &regexSeriesIdentifier, 0)
	default:
		return c.setRegexp(key, key, 20*time.Minute)
	}
}
func (c *GlobalcacheRegex) GetRegexpDirect(key *string) *regexp.Regexp {
	if c.CheckNoType(key) {
		return c.GetData(key)
	}
	return c.getregex(key)
}

func (c *GlobalcacheRegex) SetRegexp(key *string, duration time.Duration) {
	c.setRegexp(key, key, duration)
}
func (c *GlobalcacheRegex) setRegexp(key *string, reg *string, duration time.Duration) *regexp.Regexp {
	re := compileregex(reg)
	c.Set(key, re, duration, false)
	return re
}

func compileregex(reg *string) *regexp.Regexp {
	re, err := regexp.Compile(*reg)
	if err != nil {
		return nil
	}
	return re
}
