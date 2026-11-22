package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
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

	// KeyParams optionally contributes provider-specific request parameters to
	// the cache key (e.g. the aggregate provider's gddBase). Empty/nil keeps
	// the legacy key format, so existing cache entries stay valid.
	KeyParams func(domain.QueryRequest) string
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
	params := ""
	if c.opts.KeyParams != nil {
		params = c.opts.KeyParams(req)
	}
	key := CacheKey(c.inner.ID(), c.opts.Version, req, params)

	if raw, ok, err := c.cache.Get(ctx, key); err == nil && ok {
		var f domain.Feature
		// A hit is only usable if it still satisfies the attribution contract.
		// Entries written before that was enforced — or by a provider that has
		// since been fixed — would otherwise be served (and rejected) until the
		// TTL expires, which for mature data is a year. Treating such a hit as a
		// miss lets a corrected provider recover on the very next request.
		if json.Unmarshal(raw, &f) == nil && f.License.Validate() == nil {
			return domain.ProviderResult{Feature: f, Cached: true}, nil
		}
	}

	res, err := c.inner.Fetch(ctx, req)
	if err != nil {
		return domain.ProviderResult{}, err
	}

	// Validate BEFORE writing. Caching a malformed feature would outlive the
	// bug that produced it: the entry keeps failing validation downstream long
	// after the provider is fixed.
	if lerr := res.Feature.License.Validate(); lerr != nil {
		return domain.ProviderResult{}, output.NewPermanentError(
			fmt.Errorf("provider %s: %w", c.inner.ID(), lerr))
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

// CacheKey derives a deterministic key from the exact request. Every feature
// carries its point — the geometry, and values that follow from the latitude
// or the elevation — so nearby points must not share an entry. What they do
// share, the archive reads of their model cells, is cached one layer down by
// CachingSeriesReader. params, when non-empty, partitions the key further
// (e.g. by a provider-specific request parameter).
func CacheKey(providerID, version string, req domain.QueryRequest, params string) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		providerID, version, exact(req.Coordinate.Lat), exact(req.Coordinate.Lon),
		exact(req.Elevation), req.Instant.UTC().Format(time.RFC3339))
	if params != "" {
		raw += "|" + params
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// exact renders a number in its shortest round-trip form.
func exact(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
