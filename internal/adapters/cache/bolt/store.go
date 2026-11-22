package bolt

import (
	"bytes"
	"encoding/binary"
	"math"

	bolt "go.etcd.io/bbolt"
)

// Layout:
//
//	entries: key → exp (8) | atime (8) | value   (unix nanoseconds, big-endian)
//	lru:     atime (8) | key → ∅                 (iterates oldest access first)
//	meta:    "bytes" → accounted size (8)
var (
	entriesBucket = []byte("entries")
	lruBucket     = []byte("lru")
	metaBucket    = []byte("meta")
	bytesKey      = []byte("bytes")

	// legacyBucket held the uncapped format (exp | value, no index).
	legacyBucket = []byte("cache")
)

const headerLen = 16

type header struct{ exp, atime int64 }

func (h header) expired(now int64) bool { return h.exp != 0 && now > h.exp }

// toU64 and toI64 convert timestamps and byte counts, which are never
// negative, between the int64 the code computes with and the uint64 it stores.
func toU64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func toI64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

func decodeHeader(rec []byte) header {
	return header{
		exp:   toI64(binary.BigEndian.Uint64(rec[:8])),
		atime: toI64(binary.BigEndian.Uint64(rec[8:16])),
	}
}

func encodeRecord(h header, value []byte) []byte {
	rec := make([]byte, headerLen+len(value))
	binary.BigEndian.PutUint64(rec[:8], toU64(h.exp))
	binary.BigEndian.PutUint64(rec[8:16], toU64(h.atime))
	copy(rec[headerLen:], value)
	return rec
}

func indexKey(atime int64, key []byte) []byte {
	ik := make([]byte, 8+len(key))
	binary.BigEndian.PutUint64(ik[:8], toU64(atime))
	copy(ik[8:], key)
	return ik
}

// entryCost is what one entry accounts for: the record under its key plus
// its index key.
func entryCost(key []byte, valueLen int) int64 {
	return int64(len(key) + headerLen + valueLen + 8 + len(key))
}

func prepare(tx *bolt.Tx) error {
	if tx.Bucket(legacyBucket) != nil {
		if err := tx.DeleteBucket(legacyBucket); err != nil {
			return err
		}
	}
	for _, name := range [][]byte{entriesBucket, lruBucket, metaBucket} {
		if _, err := tx.CreateBucketIfNotExists(name); err != nil {
			return err
		}
	}
	return nil
}

func storedBytes(tx *bolt.Tx) int64 {
	raw := tx.Bucket(metaBucket).Get(bytesKey)
	if len(raw) != 8 {
		return 0
	}
	return toI64(binary.BigEndian.Uint64(raw))
}

func addBytes(tx *bolt.Tx, delta int64) error {
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, toU64(storedBytes(tx)+delta))
	return tx.Bucket(metaBucket).Put(bytesKey, raw)
}

func insert(tx *bolt.Tx, key []byte, h header, value []byte) error {
	if err := tx.Bucket(entriesBucket).Put(key, encodeRecord(h, value)); err != nil {
		return err
	}
	if err := tx.Bucket(lruBucket).Put(indexKey(h.atime, key), nil); err != nil {
		return err
	}
	return addBytes(tx, entryCost(key, len(value)))
}

// remove deletes key with its index entry and accounting; absent is fine.
func remove(tx *bolt.Tx, key []byte) error {
	rec := tx.Bucket(entriesBucket).Get(key)
	if len(rec) < headerLen {
		return nil
	}
	h, n := decodeHeader(rec), len(rec)-headerLen
	if err := tx.Bucket(lruBucket).Delete(indexKey(h.atime, key)); err != nil {
		return err
	}
	if err := tx.Bucket(entriesBucket).Delete(key); err != nil {
		return err
	}
	return addBytes(tx, -entryCost(key, n))
}

// removeIfExpired re-checks inside the write transaction: a concurrent Set
// may have replaced the expired entry since the read.
func removeIfExpired(tx *bolt.Tx, key []byte, now int64) error {
	rec := tx.Bucket(entriesBucket).Get(key)
	if len(rec) < headerLen || !decodeHeader(rec).expired(now) {
		return nil
	}
	return remove(tx, key)
}

// touch moves key to the most recently used end. It re-checks the access
// time inside the write transaction: concurrent readers that all saw the same
// stale time rewrite the entry once, and a delayed touch never moves the time
// backwards past a newer touch or Set.
func touch(tx *bolt.Tx, key []byte, now int64) error {
	rec := tx.Bucket(entriesBucket).Get(key)
	if len(rec) < headerLen {
		return nil
	}
	h := decodeHeader(rec)
	if now-h.atime < int64(touchInterval) {
		return nil
	}
	value := append([]byte(nil), rec[headerLen:]...)
	if err := remove(tx, key); err != nil {
		return err
	}
	h.atime = now
	return insert(tx, key, h, value)
}

// evict removes least recently used entries until the cache is at 90 % of
// limit, so a full cache does not evict on every single Set. keep (the entry
// just written, if any) is never evicted: Set only stores entries that fit
// the cap, so evicting all others is enough, and the 90 % target is best
// effort when keep alone exceeds it.
func evict(tx *bolt.Tx, limit int64, keep []byte) error {
	total := storedBytes(tx)
	if limit <= 0 || total <= limit {
		return nil
	}
	target := limit / 10 * 9
	var victims [][]byte
	entries := tx.Bucket(entriesBucket)
	c := tx.Bucket(lruBucket).Cursor()
	for ik, _ := c.First(); ik != nil && total > target; ik, _ = c.Next() {
		if bytes.Equal(ik[8:], keep) {
			continue
		}
		key := append([]byte(nil), ik[8:]...)
		total -= entryCost(key, len(entries.Get(key))-headerLen)
		victims = append(victims, key)
	}
	for _, key := range victims {
		if err := remove(tx, key); err != nil {
			return err
		}
	}
	return nil
}
