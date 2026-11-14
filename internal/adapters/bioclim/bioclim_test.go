package bioclim

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// memCache is a minimal in-memory output.Cache for the test.
type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newMemCache() *memCache { return &memCache{m: map[string][]byte{}} }

func (c *memCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok, nil
}

func (c *memCache) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
	return nil
}

// berlinBody returns a canned archive response: one day per month for two
// years, following a Berlin-like temperate pattern (→ Cfb), precip 50 mm/month,
// with the grid-cell elevation Open-Meteo reports (34 m).
func berlinBody() []byte {
	pattern := [12]float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}
	var tmin, tmax, precip [12]float64
	for m, v := range pattern {
		tmin[m], tmax[m], precip[m] = v-5, v+5, 50
	}
	return cannedBody(52.5, 13.4, 34.0, pattern, tmin, tmax, precip)
}

// cannedBody builds an archive response from monthly normals: one day per
// month for two years. elev is the top-level "elevation" field; nil omits it.
func cannedBody(lat, lon float64, elev any, tmean, tmin, tmax, precip [12]float64) []byte {
	var times []string
	var dmax, dmin, dmean, dprecip []float64
	for _, y := range []int{1991, 1992} {
		for m := 1; m <= 12; m++ {
			times = append(times, fmt.Sprintf("%d-%02d-15", y, m))
			dmean = append(dmean, tmean[m-1])
			dmin = append(dmin, tmin[m-1])
			dmax = append(dmax, tmax[m-1])
			dprecip = append(dprecip, precip[m-1])
		}
	}
	resp := map[string]any{
		"latitude": lat, "longitude": lon,
		"daily": map[string]any{
			"time": times, "temperature_2m_max": dmax, "temperature_2m_min": dmin,
			"temperature_2m_mean": dmean, "precipitation_sum": dprecip,
		},
	}
	if elev != nil {
		resp["elevation"] = elev
	}
	body, _ := json.Marshal(resp)
	return body
}

func newProvider(t *testing.T, calls *int) (*Provider, func()) {
	t.Helper()
	body := berlinBody()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*calls++
		_, _ = w.Write(body)
	}))
	p := New(Options{ArchiveBaseURL: srv.URL, Timeout: 2 * time.Second, Cache: newMemCache()})
	return p, srv.Close
}

func req(refPeriod string) domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 52.5, Lon: 13.4},
		Instant:    time.Date(2005, 6, 15, 12, 0, 0, 0, time.UTC),
		RefPeriod:  refPeriod,
	}
}

func TestFetch_RetryAfterReflectsHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// A plain client: the adapter's own retryAfter parsing must be reached
	// directly, without omhttp.Transport's internal retries intercepting the 429.
	p := New(Options{ArchiveBaseURL: srv.URL, Timeout: 2 * time.Second, HTTPClient: &http.Client{}})

	_, err := p.Fetch(context.Background(), req("1991-1992"))
	var pe output.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("Fetch err = %v, want ProviderError", err)
	}
	if pe.RetryAfter != 42*time.Second {
		t.Errorf("RetryAfter = %v, want 42s", pe.RetryAfter)
	}
}

func TestFetch_ComputesBioAndKoppen(t *testing.T) {
	calls := 0
	p, done := newProvider(t, &calls)
	defer done()

	res, err := p.Fetch(context.Background(), req("1991-1992"))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if props["kind"] != "bioclim" {
		t.Errorf("kind = %v, want bioclim", props["kind"])
	}
	if props["referencePeriod"] != "1991-1992" {
		t.Errorf("referencePeriod = %v, want 1991-1992", props["referencePeriod"])
	}
	kop, ok := props["koppen"].(map[string]any)
	if !ok || kop[keyCode] != "Cfb" {
		t.Errorf("koppen = %v, want code Cfb", props["koppen"])
	}
	// Berlin pattern: coldest month ≈ +1 °C → C/D borderline flagged with a D* adjacent.
	if kop["coldestMonthMeanC"] == nil {
		t.Error("koppen.coldestMonthMeanC missing")
	}
	if kop["borderline"] != true {
		t.Errorf("koppen.borderline = %v, want true (coldest month ≈ +1 °C)", kop["borderline"])
	}
	if adj, ok := kop["adjacent"].(map[string]any); !ok || adj[keyCode] == nil {
		t.Errorf("koppen.adjacent = %v, want a code", kop["adjacent"])
	}
	bio, ok := props["bio"].(map[string]any)
	if !ok {
		t.Fatalf("bio missing")
	}
	// BIO1 = annual mean of the pattern = 115/12 ≈ 9.6.
	if bio["bio1"] != 9.6 {
		t.Errorf("bio1 = %v, want 9.6", bio["bio1"])
	}
	// BIO12 = annual precip = 12 × 50 = 600 mm.
	if bio["bio12"] != 600.0 {
		t.Errorf("bio12 = %v, want 600", bio["bio12"])
	}
	if res.Feature.License.Attribution == "" {
		t.Error("attribution empty")
	}

	ab, ok := props["altitudinalBelt"].(map[string]any)
	if !ok {
		t.Fatalf("altitudinalBelt missing")
	}
	if belt, ok := ab["belt"].(map[string]any); !ok || belt["de"] == "" || belt["de"] == nil {
		t.Errorf("altitudinalBelt.belt = %v, want bilingual name", ab["belt"])
	}
	// Berlin pattern: Ic 18 (no compensation), It = (9.58 − 4 + 6)·10 = 115.8 <
	// 120 → Tp = 1150 → supratemperate (WBCS08 Fig. 19: 800–1400).
	tt, ok := ab["thermotype"].(map[string]any)
	if !ok || tt[keyCode] != "supratemperate" || tt["de"] != "supratemperat" || tt["en"] != "supratemperate" {
		t.Errorf("altitudinalBelt.thermotype = %v, want supratemperate", ab["thermotype"])
	}
	if tt["index"] != "Tp" || tt["value"] != 1150.0 {
		t.Errorf("thermotype index/value = %v/%v, want Tp/1150", tt["index"], tt["value"])
	}
	mb, ok := ab["macrobioclimate"].(map[string]any)
	if !ok || mb[keyCode] != "temperate" || mb["de"] != "temperat" || mb["en"] != "temperate" {
		t.Errorf("altitudinalBelt.macrobioclimate = %v, want temperate", ab["macrobioclimate"])
	}
	if mb["elevationM"] != 34.0 {
		t.Errorf("macrobioclimate.elevationM = %v, want 34 (from the archive response)", mb["elevationM"])
	}
	if ind, ok := ab["indicators"].(map[string]any); !ok || ind["matC"] == nil {
		t.Errorf("altitudinalBelt.indicators = %v, want matC", ab["indicators"])
	}
}

func TestFetch_CachesPerCoordinatePeriod(t *testing.T) {
	calls := 0
	p, done := newProvider(t, &calls)
	defer done()

	if _, err := p.Fetch(context.Background(), req("1991-1992")); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	res, err := p.Fetch(context.Background(), req("1991-1992"))
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if calls != 1 {
		t.Errorf("HTTP calls = %d, want 1 (second served from cache)", calls)
	}
	if !res.Cached {
		t.Error("second result should be Cached=true")
	}
}

func TestFetch_BadRefPeriod(t *testing.T) {
	calls := 0
	p, done := newProvider(t, &calls)
	defer done()
	if _, err := p.Fetch(context.Background(), req("2000-1990")); err == nil {
		t.Error("want error for invalid refPeriod")
	}
}

func TestProviderIDKind(t *testing.T) {
	p := New(Options{})
	if p.ID() != "bioclim" || p.Kind() != "bioclim" {
		t.Errorf("ID/Kind = %q/%q", p.ID(), p.Kind())
	}
	if p.Attribution().Attribution == "" {
		t.Error("attribution empty")
	}
}

// This provider keeps its OWN cache and is registered directly, not wrapped in
// application.CachingProvider — so it needs the same attribution guard. A
// poisoned entry lives for a year otherwise: it would be served as Cached, be
// rejected downstream, and never refetch until the TTL runs out.
func TestFetch_InvalidCachedFeatureIsRefetched(t *testing.T) {
	calls := 0
	body := berlinBody()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cache := newMemCache()
	p := New(Options{ArchiveBaseURL: srv.URL, Timeout: 2 * time.Second, Cache: cache})
	r := req("")

	// Poison the cache with a feature whose licence is incomplete.
	startY, endY, err := domain.NormalPeriod(r.Instant, r.RefPeriod)
	if err != nil {
		t.Fatalf("NormalPeriod: %v", err)
	}
	bad := domain.NewPointFeature(r.Coordinate, map[string]any{"kind": "bioclim"},
		domain.License{Name: "x", Attribution: "y"}) // no URL
	raw, err := json.Marshal(bad)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := cache.Set(context.Background(), cacheKey(r.Coordinate, startY, endY), raw, time.Hour); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	res, err := p.Fetch(context.Background(), r)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if calls != 1 {
		t.Errorf("upstream calls = %d, want 1 — the poisoned entry must not be served", calls)
	}
	if res.Cached {
		t.Error("result reported as cached although the cached entry was rejected")
	}
	if err := res.Feature.License.Validate(); err != nil {
		t.Errorf("served a feature that still fails validation: %v", err)
	}
}
