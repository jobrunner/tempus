package bolt

import (
	"encoding/binary"

	bolt "go.etcd.io/bbolt"
)

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
