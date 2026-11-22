package application

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// SeriesCacheOptions configures CachingSeriesReader.
type SeriesCacheOptions struct {
	Version     string        // cache schema version — bump to invalidate
	Maturity    time.Duration // hours older than this are final
	MatureTTL   time.Duration // TTL for a month whose hours are all final
	ImmatureTTL time.Duration // TTL for a month still inside the maturity window
	// MaxSpan bounds the reads that go through the cache. Longer ones (the
	// 30-year normals behind bioclim) pass straight through: bioclim caches
	// its result itself, and decades of hourly blocks per cell would push
	// everything else out of a capped cache.
	MaxSpan time.Duration
}

// CachingSeriesReader caches archive reads per UTC calendar month of one
// variable at one grid location of one model. Everything a provider computes
// from those reads — cell choice, elevation correction, mixing, the feature —
// is cheap and stays exact for the requested point; this is the expensive
// part, and every point in the same model cell shares it.
type CachingSeriesReader struct {
	inner output.SeriesReader
	cache output.Cache
	clock output.Clock
	opts  SeriesCacheOptions
}

var _ output.SeriesReader = (*CachingSeriesReader)(nil)

// NewCachingSeriesReader wraps inner with cache.
func NewCachingSeriesReader(inner output.SeriesReader, cache output.Cache, clock output.Clock, opts SeriesCacheOptions) *CachingSeriesReader {
	return &CachingSeriesReader{inner: inner, cache: cache, clock: clock, opts: opts}
}

// ReadSeries implements output.SeriesReader. Months missing from the cache
// are read upstream in as few calls as possible (adjacent ones merged) and
// cached; errors are never cached.
func (c *CachingSeriesReader) ReadSeries(ctx context.Context, domain, variable string, coord []uint64, from, to time.Time) ([]float32, error) {
	from, to = from.UTC().Truncate(time.Hour), to.UTC().Truncate(time.Hour)
	if !to.After(from) || to.Sub(from) > c.opts.MaxSpan {
		return c.inner.ReadSeries(ctx, domain, variable, coord, from, to)
	}
	months := monthsCovering(from, to)
	blocks := make([][]float32, len(months))
	for i, m := range months {
		blocks[i] = c.load(ctx, c.key(domain, variable, coord, m), m)
	}
	for i := 0; i < len(months); {
		if blocks[i] != nil {
			i++
			continue
		}
		j := i
		for j < len(months) && blocks[j] == nil {
			j++
		}
		if err := c.fill(ctx, domain, variable, coord, months[i:j], blocks[i:j]); err != nil {
			return nil, err
		}
		i = j
	}
	return cut(months[0], blocks, from, to), nil
}

// fill reads the adjacent months upstream in one call and caches each.
func (c *CachingSeriesReader) fill(ctx context.Context, domain, variable string, coord []uint64, months []time.Time, blocks [][]float32) error {
	end := nextMonth(months[len(months)-1])
	vals, err := c.inner.ReadSeries(ctx, domain, variable, coord, months[0], end)
	if err != nil {
		return err
	}
	if len(vals) != hoursBetween(months[0], end) {
		return fmt.Errorf("series cache: %s/%s: read %d hours, want %d", domain, variable, len(vals), hoursBetween(months[0], end))
	}
	off := 0
	for i, m := range months {
		n := hoursBetween(m, nextMonth(m))
		blocks[i] = vals[off : off+n]
		off += n
		_ = c.cache.Set(ctx, c.key(domain, variable, coord, m), encodeFloats(blocks[i]), c.ttlFor(nextMonth(m)))
	}
	return nil
}

// load returns the cached month, or nil on a miss or an unusable entry.
func (c *CachingSeriesReader) load(ctx context.Context, key string, month time.Time) []float32 {
	raw, ok, err := c.cache.Get(ctx, key)
	if err != nil || !ok || len(raw) != 4*hoursBetween(month, nextMonth(month)) {
		return nil
	}
	return decodeFloats(raw)
}

// ttlFor: a month is final once its last hour is older than the maturity.
func (c *CachingSeriesReader) ttlFor(monthEnd time.Time) time.Duration {
	if c.clock.Now().UTC().Sub(monthEnd) >= c.opts.Maturity {
		return c.opts.MatureTTL
	}
	return c.opts.ImmatureTTL
}

func (c *CachingSeriesReader) key(domain, variable string, coord []uint64, month time.Time) string {
	parts := make([]string, len(coord))
	for i, v := range coord {
		parts[i] = fmt.Sprint(v)
	}
	raw := fmt.Sprintf("series|%s|%s|%s|%s|%s",
		c.opts.Version, domain, variable, strings.Join(parts, ","), month.Format("2006-01"))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

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
