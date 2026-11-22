package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// SeriesCacheOptions configures CachingSeriesReader.
type SeriesCacheOptions struct {
	Version     string        // cache schema version — bump to invalidate
	Maturity    time.Duration // hours older than this are final (whole hours)
	MatureTTL   time.Duration // TTL for a month whose hours are all final
	ImmatureTTL time.Duration // TTL cap for a month still inside the maturity window
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
	flight blockFlight
}

var _ output.SeriesReader = (*CachingSeriesReader)(nil)

// NewCachingSeriesReader wraps inner with cache.
func NewCachingSeriesReader(inner output.SeriesReader, cache output.Cache, clock output.Clock, opts SeriesCacheOptions) *CachingSeriesReader {
	return &CachingSeriesReader{inner: inner, cache: cache, clock: clock, opts: opts}
}

// series names one variable at one grid location of one model.
type series struct {
	domain, variable string
	coord            []uint64
}

// ReadSeries implements output.SeriesReader. Months missing from the cache
// are read upstream in as few calls as possible (adjacent ones merged) and
// cached; errors are never cached.
func (c *CachingSeriesReader) ReadSeries(ctx context.Context, domain, variable string, coord []uint64, from, to time.Time) ([]float32, error) {
	from, to = from.UTC().Truncate(time.Hour), to.UTC().Truncate(time.Hour)
	if !to.After(from) || to.Sub(from) > c.opts.MaxSpan {
		return c.inner.ReadSeries(ctx, domain, variable, coord, from, to)
	}
	s := series{domain, variable, coord}
	months := monthsCovering(from, to)
	blocks := make([][]float32, len(months))
	keys := make([]string, len(months))
	var missing []int
	for i, m := range months {
		keys[i] = c.key(s, m)
		if blocks[i] = c.load(ctx, keys[i], m); blocks[i] == nil {
			missing = append(missing, i)
		}
	}
	if err := c.fill(ctx, s, months, keys, missing, blocks); err != nil {
		return nil, err
	}
	return cut(months[0], blocks, from, to), nil
}

// load returns the cached month, or nil on a miss or an unusable entry.
func (c *CachingSeriesReader) load(ctx context.Context, key string, month time.Time) []float32 {
	raw, ok, err := c.cache.Get(ctx, key)
	if err != nil || !ok || len(raw) != 4*hoursBetween(month, nextMonth(month)) {
		return nil
	}
	return decodeFloats(raw)
}

// ttlFor decides how long a month read from start on may be served. A month
// whose last hour was final at start keeps for MatureTTL. A month still
// holding younger hours must be gone by the moment the next of them turns
// final — the next whole hour past start − maturity — because the result
// cache keeps a result for a final hour for a year and must never compute it
// from a value read while that hour was still being revised. Zero or less
// means the read crossed that moment and must not be cached at all.
func (c *CachingSeriesReader) ttlFor(monthEnd, start time.Time) time.Duration {
	if start.Sub(monthEnd) >= c.opts.Maturity {
		return c.opts.MatureTTL
	}
	nextFinal := start.Add(-c.opts.Maturity).Truncate(time.Hour).Add(time.Hour + c.opts.Maturity)
	return min(c.opts.ImmatureTTL, nextFinal.Sub(c.clock.Now().UTC()))
}

func (c *CachingSeriesReader) key(s series, month time.Time) string {
	parts := make([]string, len(s.coord))
	for i, v := range s.coord {
		parts[i] = fmt.Sprint(v)
	}
	raw := fmt.Sprintf("series|%s|%s|%s|%s|%s",
		c.opts.Version, s.domain, s.variable, strings.Join(parts, ","), month.Format("2006-01"))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
