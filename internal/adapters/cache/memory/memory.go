// Package memory is an in-process output.Cache (used for tests and dev). It
// can be capped in bytes; going over the cap evicts the least recently used
// entries.
package memory

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type entry struct {
	key     string
	value   []byte
	expires time.Time // zero = never
}

// entryCost is what one entry accounts for against the cap.
func entryCost(key string, valueLen int) int64 { return int64(len(key) + valueLen) }

// Cache is a concurrency-safe in-memory cache.
type Cache struct {
	mu    sync.Mutex
	m     map[string]*list.Element
	order *list.List // front = most recently used
	bytes int64
	max   int64
	now   func() time.Time
}

// New returns an empty cache capped at maxBytes; zero or negative means
// unbounded.
func New(maxBytes int64) *Cache {
	return &Cache{m: map[string]*list.Element{}, order: list.New(), max: maxBytes, now: time.Now}
}

// Bytes reports the accounted size of all stored entries.
func (c *Cache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.m[key]
	if !ok {
		return nil, false, nil
	}
	e := el.Value.(*entry)
	if !e.expires.IsZero() && c.now().After(e.expires) {
		c.remove(el)
		return nil, false, nil
	}
	c.order.MoveToFront(el)
	return e.value, true, nil
}

// Set stores value under key. ttl == 0 means never expire. A value that
// alone exceeds the cap is not stored.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	cost := entryCost(key, len(value))
	if c.max > 0 && cost > c.max {
		return nil
	}
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		c.remove(el)
	}
	c.m[key] = c.order.PushFront(&entry{key: key, value: value, expires: exp})
	c.bytes += cost
	for c.max > 0 && c.bytes > c.max {
		c.remove(c.order.Back())
	}
	return nil
}

func (c *Cache) remove(el *list.Element) {
	e := el.Value.(*entry)
	c.order.Remove(el)
	delete(c.m, e.key)
	c.bytes -= entryCost(e.key, len(e.value))
}
