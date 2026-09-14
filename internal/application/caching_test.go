package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

type fakeCache struct {
	store  map[string][]byte
	sets   int
	setTTL time.Duration
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string][]byte{}} }
func (c *fakeCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	v, ok := c.store[k]
	return v, ok, nil
}
func (c *fakeCache) Set(_ context.Context, k string, v []byte, ttl time.Duration) error {
	c.store[k] = v
	c.sets++
	c.setTTL = ttl
	return nil
}

const (
	testProviderID   = "open-meteo"
	testProviderKind = "weather"
)

type countingProvider struct {
	calls int
	feat  domain.Feature
}

func (p *countingProvider) ID() string                  { return testProviderID }
func (p *countingProvider) Kind() string                { return testProviderKind }
func (p *countingProvider) Attribution() domain.License { return domain.License{Name: "CC-BY 4.0"} }
func (p *countingProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	p.calls++
	f := p.feat
	// Only an UNSET feature gets a default. Tests that do not care about
	// attribution should still exercise what they are about, but a test that
	// deliberately supplies a bad licence must get exactly that one back.
	if f.Type == "" {
		f = domain.NewPointFeature(domain.Coordinate{Lat: 1, Lon: 2},
			map[string]any{"v": 1.0}, goodLicense())
	}
	return domain.ProviderResult{Feature: f}, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func req(instant time.Time) domain.QueryRequest {
	return domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.79123, Lon: 9.93456}, Instant: instant}
}

func opts() CachingOptions {
	return CachingOptions{Version: "1", ArchiveDelay: 5 * 24 * time.Hour,
		MatureTTL: 365 * 24 * time.Hour, ImmatureTTL: time.Hour, LatLonPrecision: 2}
}

func TestCaching_MissThenHit(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	inner := &countingProvider{feat: domain.NewPointFeature(domain.Coordinate{Lat: 49.79, Lon: 9.93}, map[string]any{"t": 1.0}, goodLicense())}
	cp := NewCachingProvider(inner, newFakeCache(), fixedClock{now}, opts())

	old := req(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) // well before archive delay
	r1, err := cp.Fetch(context.Background(), old)
	if err != nil || r1.Cached {
		t.Fatalf("first fetch: cached=%v err=%v", r1.Cached, err)
	}
	r2, err := cp.Fetch(context.Background(), old)
	if err != nil || !r2.Cached {
		t.Fatalf("second fetch must be cached: cached=%v err=%v", r2.Cached, err)
	}
	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1", inner.calls)
	}
}

func TestCaching_TTLByMaturity(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	mature := newFakeCache()
	if _, err := NewCachingProvider(&countingProvider{}, mature, fixedClock{now}, opts()).
		Fetch(context.Background(), req(now.Add(-30*24*time.Hour))); err != nil {
		t.Fatalf("mature fetch: %v", err)
	}
	if mature.setTTL != 365*24*time.Hour {
		t.Errorf("mature TTL = %v, want 8760h", mature.setTTL)
	}
	immature := newFakeCache()
	if _, err := NewCachingProvider(&countingProvider{}, immature, fixedClock{now}, opts()).
		Fetch(context.Background(), req(now.Add(-2*time.Hour))); err != nil {
		t.Fatalf("immature fetch: %v", err)
	}
	if immature.setTTL != time.Hour {
		t.Errorf("immature TTL = %v, want 1h", immature.setTTL)
	}
}

func TestCacheKey_RoundsCoords(t *testing.T) {
	instant := time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)
	a := CacheKey(testProviderID, "1", domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.791, Lon: 9.934}, Instant: instant}, 2, "")
	b := CacheKey(testProviderID, "1", domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.789, Lon: 9.931}, Instant: instant}, 2, "")
	if a != b {
		t.Errorf("coords within rounding must share a key: %s vs %s", a, b)
	}
}

// TestCacheKey_EmptyParamsKeepLegacyFormat pins the raw key format for
// params=="" byte-for-byte, so existing cache entries stay valid across
// deploys that add KeyParams for a provider that did not use it before.
func TestCacheKey_EmptyParamsKeepLegacyFormat(t *testing.T) {
	r := domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 49.79345, Lon: 9.95341},
		Instant:    time.Date(2025, 6, 3, 14, 0, 0, 0, time.UTC),
	}
	// The legacy raw format, reproduced literally: params must not change it.
	raw := fmt.Sprintf("%s|%s|%.*f|%.*f|%s",
		"open-meteo", "1", 2, 49.79, 2, 9.95, r.Instant.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got := CacheKey("open-meteo", "1", r, 2, ""); got != want {
		t.Errorf("CacheKey with empty params = %s, want legacy %s", got, want)
	}
	if same := CacheKey("open-meteo", "1", r, 2, "gdd=7.00"); same == want {
		t.Error("non-empty params must produce a different key")
	}
}

// TestCachingProvider_KeyParamsSeparateEntries verifies that KeyParams
// partitions the cache: requests differing only in GDDBaseCelsius must miss
// independently, while identical GDDBaseCelsius values hit.
func TestCachingProvider_KeyParamsSeparateEntries(t *testing.T) {
	base7, base9 := 7.0, 9.0
	inner := &countingProvider{}
	cp := NewCachingProvider(inner, newFakeCache(), fixedClock{}, CachingOptions{
		Version: "1", MatureTTL: time.Hour, ImmatureTTL: time.Hour, LatLonPrecision: 2,
		KeyParams: func(req domain.QueryRequest) string {
			if req.GDDBaseCelsius == nil {
				return ""
			}
			return fmt.Sprintf("gdd=%.2f", *req.GDDBaseCelsius)
		},
	})
	r := domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 1, Lon: 2}, Instant: time.Unix(0, 0).UTC()}
	r.GDDBaseCelsius = &base7
	_, _ = cp.Fetch(context.Background(), r)
	r.GDDBaseCelsius = &base9
	_, _ = cp.Fetch(context.Background(), r)
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2 (distinct gddBase must miss)", inner.calls)
	}
	r.GDDBaseCelsius = &base7
	res, _ := cp.Fetch(context.Background(), r)
	if inner.calls != 2 || !res.Cached {
		t.Fatalf("inner calls = %d, cached = %v; want 2, true", inner.calls, res.Cached)
	}
}

func TestCaching_InnerErrorNotCached(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	fc := newFakeCache()
	failing := errorProvider{err: errors.New("boom")}
	cp := NewCachingProvider(failing, fc, fixedClock{now}, opts())
	if _, err := cp.Fetch(context.Background(), req(now.Add(-time.Hour))); err == nil {
		t.Fatal("expected error to propagate")
	}
	if fc.sets != 0 {
		t.Errorf("errors must not be cached, sets=%d", fc.sets)
	}
}

type errorProvider struct{ err error }

func (e errorProvider) ID() string                  { return testProviderID }
func (e errorProvider) Kind() string                { return testProviderKind }
func (e errorProvider) Attribution() domain.License { return domain.License{} }
func (e errorProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, e.err
}

func goodLicense() domain.License {
	return domain.License{Name: "CC-BY 4.0", URL: "https://example.org/l", Attribution: "Example"}
}

// A malformed feature must never enter the cache. Mature entries live for up to
// a year, so caching one would keep the fault alive long after the provider was
// fixed — the gate would go on rejecting a stale cached copy.
func TestCachingProvider_DoesNotCacheIncompleteLicense(t *testing.T) {
	bad := domain.NewPointFeature(domain.Coordinate{Lat: 1, Lon: 2},
		map[string]any{"v": 1.0}, domain.License{Name: "x", Attribution: "y"}) // no URL
	cache := newFakeCache()
	cp := NewCachingProvider(&countingProvider{feat: bad}, cache, fixedClock{}, opts())

	_, err := cp.Fetch(context.Background(), domain.QueryRequest{Instant: time.Unix(0, 0).UTC()})
	if err == nil {
		t.Fatal("Fetch must fail on an incomplete licence rather than pass it on")
	}
	if cache.sets != 0 {
		t.Errorf("cache.Set called %d times; a malformed feature must not be stored", cache.sets)
	}
}

// An entry stored before the rule was enforced must not be served forever. A
// cache hit that fails validation is treated as a miss, so a corrected provider
// recovers on the next request instead of after the TTL.
func TestCachingProvider_TreatsInvalidCacheHitAsMiss(t *testing.T) {
	bad := domain.NewPointFeature(domain.Coordinate{Lat: 1, Lon: 2},
		map[string]any{"v": 1.0}, domain.License{Name: "x", Attribution: "y"})
	raw, err := json.Marshal(bad)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 1, Lon: 2}, Instant: time.Unix(0, 0).UTC()}

	cache := newFakeCache()
	good := domain.NewPointFeature(domain.Coordinate{Lat: 1, Lon: 2}, map[string]any{"v": 2.0}, goodLicense())
	inner := &countingProvider{feat: good}
	cp := NewCachingProvider(inner, cache, fixedClock{}, opts())

	// Poison the cache under the key this request will use.
	cache.store[CacheKey(testProviderID, "1", req, 2, "")] = raw

	res, err := cp.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if inner.calls != 1 {
		t.Errorf("inner calls = %d, want 1 — the poisoned hit must fall through to a real fetch", inner.calls)
	}
	if res.Cached {
		t.Error("result reported as cached although the cached entry was rejected")
	}
	if res.Feature.License.Validate() != nil {
		t.Errorf("served a feature that still fails validation: %+v", res.Feature.License)
	}
}
