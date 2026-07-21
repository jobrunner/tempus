// Package memory is an in-process output.Cache (used for tests and dev).
package memory

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	value   []byte
	expires time.Time // zero = never
}

// Cache is a concurrency-safe in-memory cache.
type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	now func() time.Time
}

// New returns an empty cache.
func New() *Cache {
	return &Cache{m: map[string]entry{}, now: time.Now}
}

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expires.IsZero() && c.now().After(e.expires) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false, nil
	}
	return e.value, true, nil
}

// Set stores value under key. ttl == 0 means never expire.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = entry{value: value, expires: exp}
	c.mu.Unlock()
	return nil
}
