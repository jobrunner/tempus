package bolt

import (
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
	// MaxBytes caps the live entries (keys, values and bookkeeping), not
	// the file. Zero or negative means unbounded. bbolt never shrinks its
	// file: freed pages are reused, so the file keeps its high-water mark —
	// a little above the cap in steady state, more if it once held more
	// (an uncapped or larger-capped cache, or the pre-cap format).
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
// A file written by the uncapped format is emptied: it is a cache. A file
// over opts.MaxBytes (written under a larger cap) is trimmed right away.
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
	if err := db.Update(func(tx *bolt.Tx) error {
		if err := prepare(tx); err != nil {
			return err
		}
		return evict(tx, opts.MaxBytes, nil)
	}); err != nil {
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
