package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// CachingOptions configures the caching decorator.
type CachingOptions struct {
	Version         string        // cache schema version — bump to invalidate
	ArchiveDelay    time.Duration // instants older than this are immutable
	MatureTTL       time.Duration // TTL for immutable (mature) data
	ImmatureTTL     time.Duration // TTL for data still inside the maturity window
	LatLonPrecision int           // decimal places for the cache-key coordinate rounding
}

// CachingProvider decorates a FeatureProvider with a Cache. It implements
// output.FeatureProvider so the service treats it like any provider.
type CachingProvider struct {
	inner output.FeatureProvider
	cache output.Cache
	clock output.Clock
	opts  CachingOptions
}

// NewCachingProvider wraps inner with cache.
func NewCachingProvider(inner output.FeatureProvider, cache output.Cache, clock output.Clock, opts CachingOptions) *CachingProvider {
	return &CachingProvider{inner: inner, cache: cache, clock: clock, opts: opts}
}

func (c *CachingProvider) ID() string                  { return c.inner.ID() }
func (c *CachingProvider) Kind() string                { return c.inner.Kind() }
func (c *CachingProvider) Attribution() domain.License { return c.inner.Attribution() }

// Fetch returns a cached feature when present, else calls the inner provider and
// caches the result with a maturity-based TTL. Errors are never cached.
func (c *CachingProvider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	key := CacheKey(c.inner.ID(), c.opts.Version, req, c.opts.LatLonPrecision)

	if raw, ok, err := c.cache.Get(ctx, key); err == nil && ok {
		var f domain.Feature
		if json.Unmarshal(raw, &f) == nil {
			return domain.ProviderResult{Feature: f, Cached: true}, nil
		}
	}

	res, err := c.inner.Fetch(ctx, req)
	if err != nil {
		return domain.ProviderResult{}, err
	}

	if raw, mErr := json.Marshal(res.Feature); mErr == nil {
		_ = c.cache.Set(ctx, key, raw, c.ttlFor(req.Instant))
	}
	res.Cached = false
	return res, nil
}

func (c *CachingProvider) ttlFor(instant time.Time) time.Duration {
	if c.clock.Now().UTC().Sub(instant) >= c.opts.ArchiveDelay {
		return c.opts.MatureTTL
	}
	return c.opts.ImmatureTTL
}

// CacheKey derives a deterministic key. Coordinates are rounded to precision
// decimals so nearby points share the coarse weather grid cell (better hit rate);
// determinism preserves idempotency.
func CacheKey(providerID, version string, req domain.QueryRequest, precision int) string {
	lat := roundTo(req.Coordinate.Lat, precision)
	lon := roundTo(req.Coordinate.Lon, precision)
	raw := fmt.Sprintf("%s|%s|%.*f|%.*f|%s",
		providerID, version, precision, lat, precision, lon, req.Instant.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func roundTo(v float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	// round half away from zero
	if v >= 0 {
		return float64(int64(v*p+0.5)) / p
	}
	return float64(int64(v*p-0.5)) / p
}
