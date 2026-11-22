package application

import (
	"encoding/binary"
	"math"
	"time"
)

// Month blocks: the unit CachingSeriesReader caches.

func monthsCovering(from, to time.Time) []time.Time {
	var out []time.Time
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); m.Before(to); m = nextMonth(m) {
		out = append(out, m)
	}
	return out
}

func nextMonth(m time.Time) time.Time { return m.AddDate(0, 1, 0) }

func hoursBetween(a, b time.Time) int { return int(b.Sub(a) / time.Hour) }

// cut joins the month blocks, which start at first, and returns [from, to).
func cut(first time.Time, blocks [][]float32, from, to time.Time) []float32 {
	start := hoursBetween(first, from)
	out := make([]float32, 0, hoursBetween(from, to))
	pos := 0
	for _, b := range blocks {
		lo, hi := max(start-pos, 0), min(start+cap(out)-pos, len(b))
		if lo < hi {
			out = append(out, b[lo:hi]...)
		}
		pos += len(b)
	}
	return out
}

func encodeFloats(v []float32) []byte {
	raw := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(f))
	}
	return raw
}

func decodeFloats(raw []byte) []float32 {
	v := make([]float32, len(raw)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return v
}
