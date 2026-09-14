// Caching for the bioclim provider.
//
// This provider keeps its OWN cache rather than being wrapped in
// application.CachingProvider, because its unit of work is a 30-year normal
// period rather than a single instant. Entries therefore live for a year, which
// is what makes the attribution guard below load-bearing: a malformed entry
// would otherwise be served, rejected downstream, and never refetched until the
// TTL expired.
package bioclim

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jobrunner/tempus/internal/domain"
)

// cachedFeature returns a usable cached feature, if there is one.
//
// A hit only counts when it still carries complete attribution. This provider
// keeps its OWN cache and is registered directly rather than wrapped in
// application.CachingProvider, so it needs that guard itself: entries live for a
// year, and without this a poisoned one would be served, rejected downstream,
// and never refetched until the TTL expired. Treating it as a miss lets a
// corrected provider recover on the next request.
func (p *Provider) cachedFeature(ctx context.Context, key string) (domain.Feature, bool) {
	if p.cache == nil {
		return domain.Feature{}, false
	}
	raw, ok, _ := p.cache.Get(ctx, key)
	if !ok {
		return domain.Feature{}, false
	}
	var feat domain.Feature
	if json.Unmarshal(raw, &feat) != nil || feat.License.Validate() != nil {
		return domain.Feature{}, false
	}
	return feat, true
}

// cacheFeature stores feat. Callers validate the licence first: an entry here
// lives for a year, so a malformed feature would outlive whatever produced it
// and keep failing downstream long after the cause was fixed.
func (p *Provider) cacheFeature(ctx context.Context, key string, feat domain.Feature) {
	if p.cache == nil {
		return
	}
	if raw, err := json.Marshal(feat); err == nil {
		_ = p.cache.Set(ctx, key, raw, cacheTTL)
	}
}

// cacheKey rounds the coordinate to ~10 m (4 decimals). This is far finer than
// ERA5's native grid (~0.1–0.25°), so two coordinates that share a key resolve
// to the same ERA5 cell and thus the same climate — no incorrect collisions —
// while co-located records (same georeference) still share a cache entry.
func cacheKey(coord domain.Coordinate, startY, endY int) string {
	return fmt.Sprintf("%s|%s|%.4f|%.4f|%d-%d", providerID, cacheVersion, coord.Lat, coord.Lon, startY, endY)
}
