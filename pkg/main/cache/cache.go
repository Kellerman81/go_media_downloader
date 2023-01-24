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
type globalcache struct {
	items       map[string]int64
	itemsstatic map[string]interface{}
	mu          *sync.Mutex
	close       chan struct{}
}
type regex struct {
	items       map[string]int64
	itemsstatic map[string]*regexp.Regexp
	mu          *sync.Mutex
	close       chan struct{}
}
type stmt struct {
	items       map[string]int64
	itemsstatic map[string]*sqlx.Stmt
	mu          *sync.Mutex
	close       chan struct{}
}
type stmtNamed struct {
	items       map[string]int64
	itemsstatic map[string]*sqlx.NamedStmt
	mu          *sync.Mutex
	close       chan struct{}
}
type Return struct {
	Value interface{}
}

// New creates a new c that asynchronously cleans
// expired entries after the given time passes.
func New(cleaningInterval time.Duration) *globalcache {
	c := globalcache{
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

					for key := range c.items {
						c.mu.Lock()
						if c.items[key] != 0 && now > c.items[key] {
							delete(c.items, key)
							delete(c.itemsstatic, key)
						}
						c.mu.Unlock()
					}

				case <-c.close:
					return
				}
			}
		}()
	}

	return &c
}

// Get gets the value for the given key.
func (c *globalcache) Get(key string) (*Return, bool) {
	c.mu.Lock()
	item, exists := c.items[key]
	if exists && item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
		return nil, false
	}

	data, exists := c.itemsstatic[key]
	c.mu.Unlock()
	if exists {
		return &Return{Value: data}, true
	}
	return nil, false
}

// Get gets the value for the given key.
func (c *globalcache) Check(key string, kind reflect.Type) bool {
	c.mu.Lock()
	item, exists := c.items[key]
	if !exists {
		_, exists = c.itemsstatic[key]
		c.mu.Unlock()
		return exists
	}
	if item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
	}

	data, exists := c.itemsstatic[key]
	c.mu.Unlock()

	if !exists {
		return false
	}
	if reflect.TypeOf(data) != kind {
		exists = false
	}
	return exists
}

// Get gets the value for the given key.
func (c *globalcache) CheckNoType(key string) bool {
	c.mu.Lock()
	item, exists := c.items[key]
	if !exists {
		_, exists = c.itemsstatic[key]
		c.mu.Unlock()
		return exists
	}
	if item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
	}

	_, exists = c.itemsstatic[key]
	c.mu.Unlock()
	return exists
}
func (c *globalcache) GetData(key string) *Return {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &Return{Value: c.itemsstatic[key]}
}

func (c *globalcache) GetAll() map[string]interface{} {
	c.mu.Lock()

	ret := make(map[string]interface{})
	for key := range c.items {
		if c.items[key] != 0 && time.Now().UnixNano() > c.items[key] {
			delete(c.items, key)
			delete(c.itemsstatic, key)
		}
	}
	for key := range c.itemsstatic {
		ret[key] = c.itemsstatic[key]
	}
	c.mu.Unlock()
	return ret
}
func (c *globalcache) GetPrefix(prefix string) map[string]interface{} {
	c.mu.Lock()

	ret := make(map[string]interface{})
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			if c.items[key] != 0 && time.Now().UnixNano() > c.items[key] {
				delete(c.items, key)
				delete(c.itemsstatic, key)
			}
		}
	}
	for key := range c.itemsstatic {
		if strings.HasPrefix(key, prefix) {
			ret[key] = c.itemsstatic[key]
		}
	}
	c.mu.Unlock()
	return ret
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (c *globalcache) Set(key string, value interface{}, duration time.Duration) {
	c.mu.Lock()
	if duration > 0 {
		c.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(c.items, key)
	}
	c.itemsstatic[key] = value
	c.mu.Unlock()
}

// Delete deletes the key and its value from the c.
func (c *globalcache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	delete(c.itemsstatic, key)
	c.mu.Unlock()
}

// Close closes the c and frees up resources.
func (c *globalcache) Close() {
	c.items = map[string]int64{}
	c.itemsstatic = map[string]interface{}{}
}

// New Regex Cache
func NewRegex(cleaningInterval time.Duration) *regex {
	c := regex{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]*regexp.Regexp, 100),
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
					for key := range c.items {
						c.mu.Lock()
						c.clearexpired(key)
						c.mu.Unlock()
					}

				case <-c.close:
					return
				}
			}
		}()
	}
	return &c
}
func (c *regex) clearexpired(key string) {
	item, exists := c.items[key]
	if !exists {
		return
	}
	if item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
		return
	}
}
func (c *stmt) clearexpired(key string) {
	item, exists := c.items[key]
	if !exists {
		return
	}
	if item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
		return
	}
}
func (c *stmtNamed) clearexpired(key string) {
	item, exists := c.items[key]
	if !exists {
		return
	}
	if item != 0 && time.Now().UnixNano() > item {
		delete(c.items, key)
		delete(c.itemsstatic, key)
		return
	}
}
func (c *regex) CheckRegexp(key string) bool {
	c.mu.Lock()

	c.clearexpired(key)

	val, exists := c.itemsstatic[key]
	if val == nil {
		exists = false
	}
	c.mu.Unlock()
	return exists
}

func (c *regex) getregex(key string) *regexp.Regexp {

	var regexstr string
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
		c.mu.Lock()
		if expires != 0 {
			c.items[key] = expires
		}
		c.itemsstatic[key] = setdata
		c.mu.Unlock()
		//defer clearVar(&setdata)
		return setdata
	}
	return nil
}
func (c *regex) GetRegexpDirect(key string) *regexp.Regexp {

	if c.CheckRegexp(key) {
		return c.itemsstatic[key]
	}
	return c.getregex(key)
}
func (c *regex) SetRegexp(key string, duration time.Duration) {
	reg, err := regexp.Compile(key)
	if err != nil {
		return
	}
	c.mu.Lock()
	if duration > 0 {
		c.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(c.items, key)
	}
	c.itemsstatic[key] = reg
	c.mu.Unlock()
}

// Delete deletes the key and its value from the c.
func (c *regex) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	delete(c.itemsstatic, key)
	c.mu.Unlock()
}

// Close closes the c and frees up resources.
func (c *regex) Close() {
	c.items = map[string]int64{}
	c.itemsstatic = map[string]*regexp.Regexp{}
}

// New SQLx Stmt Cache
func NewStmt(cleaningInterval time.Duration) *stmt {
	c := stmt{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]*sqlx.Stmt, 100),
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
					for key := range c.items {
						c.mu.Lock()
						c.clearexpired(key)
						c.mu.Unlock()
					}

				case <-c.close:
					return
				}
			}
		}()
	}
	return &c
}

// Get gets the value for the given key.
func (c *stmt) Check(key string) bool {
	c.mu.Lock()
	c.clearexpired(key)
	_, exists := c.items[key]
	if exists {
		var val *sqlx.Stmt
		val, exists = c.itemsstatic[key]
		if val == nil {
			exists = false
		}
		c.mu.Unlock()

		return exists
	}
	c.mu.Unlock()
	return false
}

func (c *stmt) GetData(key string) *sqlx.Stmt {

	if c.Check(key) {
		return c.itemsstatic[key]
	}
	return nil
}
func (c *stmt) SetStmt(key string, value *sqlx.Stmt, duration time.Duration) {
	c.mu.Lock()
	if duration > 0 {
		c.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(c.items, key)
	}
	c.itemsstatic[key] = value
	c.mu.Unlock()
}

// Delete deletes the key and its value from the c.
func (c *stmt) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	delete(c.itemsstatic, key)
	c.mu.Unlock()
}

// Close closes the c and frees up resources.
func (c *stmt) Close() {
	c.items = map[string]int64{}
	c.itemsstatic = map[string]*sqlx.Stmt{}
}

// New SQLx NamedStmt Cache
func NewStmtNamed(cleaningInterval time.Duration) *stmtNamed {
	c := stmtNamed{
		items:       make(map[string]int64, 100),
		itemsstatic: make(map[string]*sqlx.NamedStmt, 100),
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
					for key := range c.items {
						c.mu.Lock()
						c.clearexpired(key)
						c.mu.Unlock()
					}

				case <-c.close:
					return
				}
			}
		}()
	}
	return &c
}

// Get gets the value for the given key.
func (c *stmtNamed) Check(key string) bool {
	c.mu.Lock()
	c.clearexpired(key)
	_, exists := c.items[key]
	if exists {
		var val *sqlx.NamedStmt
		val, exists = c.itemsstatic[key]
		if val == nil {
			exists = false
		}
		c.mu.Unlock()
		return exists
	}
	c.mu.Unlock()
	return false
}

func (c *stmtNamed) GetData(key string) *sqlx.NamedStmt {

	if c.Check(key) {
		return c.itemsstatic[key]
	}
	return nil
}
func (c *stmtNamed) SetNamedStmt(key string, value *sqlx.NamedStmt, duration time.Duration) {
	c.mu.Lock()
	if duration > 0 {
		c.items[key] = time.Now().Add(duration).UnixNano()
	} else {
		delete(c.items, key)
	}

	c.itemsstatic[key] = value
	c.mu.Unlock()
}

// Delete deletes the key and its value from the c.
func (c *stmtNamed) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	delete(c.itemsstatic, key)
	c.mu.Unlock()
}

// Close closes the c and frees up resources.
func (c *stmtNamed) Close() {
	c.items = map[string]int64{}
	c.itemsstatic = map[string]*sqlx.NamedStmt{}
}
