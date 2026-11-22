package bolt

import (
	"bytes"

	bolt "go.etcd.io/bbolt"
)

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
