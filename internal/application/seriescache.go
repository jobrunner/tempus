package application

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
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
	inner  output.SeriesReader
	cache  output.Cache
	clock  output.Clock
	opts   SeriesCacheOptions
	flight seriesFlight
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
// Concurrent fills of the same months share one upstream read: nearby
// points of a batch land in the same cells and would otherwise each read
// the same month.
func (c *CachingSeriesReader) fill(ctx context.Context, domain, variable string, coord []uint64, months []time.Time, blocks [][]float32) error {
	last := months[len(months)-1]
	key := c.key(domain, variable, coord, months[0]) + "…" + last.Format("2006-01")
	vals, err := c.flight.do(ctx, key, func(ctx context.Context) ([]float32, error) {
		return c.readMonths(ctx, domain, variable, coord, months)
	})
	if err != nil {
		return err
	}
	off := 0
	for i, m := range months {
		n := hoursBetween(m, nextMonth(m))
		blocks[i] = vals[off : off+n]
		off += n
	}
	return nil
}

// readMonths reads the months upstream and caches each. It first re-checks
// the cache: a fill that finished just before this one started has them.
func (c *CachingSeriesReader) readMonths(ctx context.Context, domain, variable string, coord []uint64, months []time.Time) ([]float32, error) {
	end := nextMonth(months[len(months)-1])
	if vals, ok := c.loadAll(ctx, domain, variable, coord, months); ok {
		return vals, nil
	}
	vals, err := c.inner.ReadSeries(ctx, domain, variable, coord, months[0], end)
	if err != nil {
		return nil, err
	}
	if len(vals) != hoursBetween(months[0], end) {
		return nil, fmt.Errorf("series cache: %s/%s: read %d hours, want %d", domain, variable, len(vals), hoursBetween(months[0], end))
	}
	off := 0
	for _, m := range months {
		n := hoursBetween(m, nextMonth(m))
		_ = c.cache.Set(ctx, c.key(domain, variable, coord, m), encodeFloats(vals[off:off+n]), c.ttlFor(nextMonth(m)))
		off += n
	}
	return vals, nil
}

// loadAll returns the months joined when every one of them is cached.
func (c *CachingSeriesReader) loadAll(ctx context.Context, domain, variable string, coord []uint64, months []time.Time) ([]float32, bool) {
	var out []float32
	for _, m := range months {
		b := c.load(ctx, c.key(domain, variable, coord, m), m)
		if b == nil {
			return nil, false
		}
		out = append(out, b...)
	}
	return out, true
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
// A month still holding younger hours expires no later than the moment the
// next of them turns final (the next whole hour past now − maturity): the
// result cache keeps a result for a final hour for a year, so it must never
// be computed from a block read while that hour was still being revised.
// This assumes a maturity of whole hours, as hours are the archive's unit.
func (c *CachingSeriesReader) ttlFor(monthEnd time.Time) time.Duration {
	now := c.clock.Now().UTC()
	if now.Sub(monthEnd) >= c.opts.Maturity {
		return c.opts.MatureTTL
	}
	nextFinal := now.Add(-c.opts.Maturity).Truncate(time.Hour).Add(time.Hour + c.opts.Maturity)
	return min(c.opts.ImmatureTTL, nextFinal.Sub(now))
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

// seriesFlightTimeout bounds a shared fill. It runs detached from the caller
// that started it, so one caller giving up never fails the others.
const seriesFlightTimeout = 2 * time.Minute

// seriesFlight collapses concurrent fills with the same key into one.
type seriesFlight struct {
	mu    sync.Mutex
	calls map[string]*seriesCall
}

type seriesCall struct {
	done chan struct{}
	vals []float32
	err  error
}

// do runs fn once per key at a time; every caller waits only as long as its
// own context allows.
func (g *seriesFlight) do(ctx context.Context, key string, fn func(context.Context) ([]float32, error)) ([]float32, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*seriesCall{}
	}
	call, ok := g.calls[key]
	if !ok {
		call = &seriesCall{done: make(chan struct{})}
		g.calls[key] = call
		go g.run(context.WithoutCancel(ctx), key, call, fn)
	}
	g.mu.Unlock()
	select {
	case <-call.done:
		return call.vals, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *seriesFlight) run(detached context.Context, key string, call *seriesCall, fn func(context.Context) ([]float32, error)) {
	ctx, cancel := context.WithTimeout(detached, seriesFlightTimeout)
	defer cancel()
	call.vals, call.err = fn(ctx)
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	close(call.done)
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
