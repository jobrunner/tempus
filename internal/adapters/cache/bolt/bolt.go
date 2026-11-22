// Package bolt is a persistent output.Cache backed by a single BoltDB file.
//
// The cache can be capped in bytes. Every entry carries its last access time
// and an index orders entries by it, so going over the cap evicts the least
// recently used ones. Access times are only rewritten once per touchInterval:
// a read stays a read transaction on a warm entry, and the eviction order is
// LRU at that granularity.
package bolt

import (
	"context"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// touchInterval is how stale an entry's access time may get before a read
// rewrites it.
const touchInterval = time.Hour

// Options configures a Cache.
type Options struct {
	// MaxBytes caps the stored entries (keys, values and bookkeeping).
	// Zero or negative means unbounded. The file itself does not shrink:
	// bbolt reuses freed pages, so it settles a little above the cap.
	MaxBytes int64
	// Now is the clock for expiry and access times; nil means time.Now.
	Now func() time.Time
}

// Cache is a BoltDB-backed cache.
type Cache struct {
	db  *bolt.DB
	max int64
	now func() time.Time
}

// Open opens (creating parent dirs and buckets as needed) the cache at path.
// A file written by the uncapped format is emptied: it is a cache.
func Open(path string, opts Options) (*Cache, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	if err := db.Update(prepare); err != nil {
		_ = db.Close()
		return nil, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Cache{db: db, max: opts.MaxBytes, now: now}, nil
}

// Close closes the underlying database.
func (c *Cache) Close() error { return c.db.Close() }

// Bytes reports the accounted size of all stored entries.
func (c *Cache) Bytes() int64 {
	var n int64
	_ = c.db.View(func(tx *bolt.Tx) error {
		n = storedBytes(tx)
		return nil
	})
	return n
}

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	k := []byte(key)
	var (
		out   []byte
		found bool
		h     header
	)
	err := c.db.View(func(tx *bolt.Tx) error {
		rec := tx.Bucket(entriesBucket).Get(k)
		if len(rec) < headerLen {
			return nil
		}
		found, h = true, decodeHeader(rec)
		out = append([]byte(nil), rec[headerLen:]...)
		return nil
	})
	if err != nil || !found {
		return nil, false, err
	}
	now := c.now().UnixNano()
	if h.expired(now) {
		return nil, false, c.db.Update(func(tx *bolt.Tx) error { return removeIfExpired(tx, k, now) })
	}
	if now-h.atime >= int64(touchInterval) {
		_ = c.db.Update(func(tx *bolt.Tx) error { return touch(tx, k, now) })
	}
	return out, true, nil
}

// Set stores value under key. ttl == 0 means never expire. A value that
// alone exceeds the cap is not stored.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	k := []byte(key)
	if c.max > 0 && entryCost(k, len(value)) > c.max {
		return nil
	}
	now := c.now()
	h := header{atime: now.UnixNano()}
	if ttl != 0 {
		h.exp = now.Add(ttl).UnixNano()
	}
	return c.db.Update(func(tx *bolt.Tx) error {
		if err := remove(tx, k); err != nil {
			return err
		}
		if err := insert(tx, k, h, value); err != nil {
			return err
		}
		return evict(tx, c.max)
	})
}
