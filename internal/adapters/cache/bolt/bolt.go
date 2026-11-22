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
	"time"

	bolt "go.etcd.io/bbolt"
)

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	k := []byte(key)
	var (
		out   []byte
		found bool
		h     header
	)
	now := c.now().UnixNano()
	err := c.db.View(func(tx *bolt.Tx) error {
		rec := tx.Bucket(entriesBucket).Get(k)
		if len(rec) < headerLen {
			return nil
		}
		found, h = true, decodeHeader(rec)
		if !h.expired(now) { // an expired value is never copied
			out = append([]byte(nil), rec[headerLen:]...)
		}
		return nil
	})
	if err != nil || !found {
		return nil, false, err
	}
	if h.expired(now) {
		return nil, false, c.db.Update(func(tx *bolt.Tx) error { return removeIfExpired(tx, k, now) })
	}
	if now-h.atime >= int64(touchInterval) {
		_ = c.db.Update(func(tx *bolt.Tx) error { return touch(tx, k, now) })
	}
	return out, true, nil
}

// Set stores value under key. ttl == 0 means never expire; a negative ttl
// is already expired and stores nothing. A value that alone exceeds the cap
// is not stored.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	k := []byte(key)
	if ttl < 0 || (c.max > 0 && entryCost(k, len(value)) > c.max) {
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
		return evict(tx, c.max, k)
	})
}
