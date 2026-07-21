// Package bolt is a persistent output.Cache backed by a single BoltDB file.
package bolt

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("cache")

// Cache is a BoltDB-backed cache.
type Cache struct {
	db *bolt.DB
}

// Open opens (creating parent dirs and the bucket as needed) the cache at path.
func Open(path string) (*Cache, error) {
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
		_, e := tx.CreateBucketIfNotExists(bucket)
		return e
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

// Close closes the underlying database.
func (c *Cache) Close() error { return c.db.Close() }

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	var (
		out     []byte
		found   bool
		expired bool
	)
	err := c.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucket).Get([]byte(key))
		if len(raw) < 8 {
			return nil
		}
		found = true
		expNano := int64(binary.BigEndian.Uint64(raw[:8])) //nolint:gosec // nanosecond timestamp written by this same code; value is always a valid int64
		if expNano != 0 && time.Now().UnixNano() > expNano {
			expired = true
			return nil
		}
		out = append(out, raw[8:]...)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if expired {
		_ = c.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucket).Delete([]byte(key)) })
		return nil, false, nil
	}
	if !found {
		return nil, false, nil
	}
	return out, true, nil
}

// Set stores value under key. ttl == 0 means never expire.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	var expNano int64
	if ttl != 0 {
		expNano = time.Now().Add(ttl).UnixNano()
	}
	buf := make([]byte, 8+len(value))
	binary.BigEndian.PutUint64(buf[:8], uint64(expNano)) //nolint:gosec // expNano is time.Now().Add(ttl).UnixNano(); always non-negative when ttl > 0
	copy(buf[8:], value)
	return c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Put([]byte(key), buf)
	})
}
