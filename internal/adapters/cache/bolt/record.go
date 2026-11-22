package bolt

import (
	"encoding/binary"
	"math"
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
