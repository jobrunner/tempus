package bioclim

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
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

// fakeReanalysis is an output.Reanalysis that serves a fixed DailySeries (or
// err) and records what it was asked for.
type fakeReanalysis struct {
	series      domain.DailySeries
	err         error
	calls       int
	point       domain.GeoPoint
	first, last time.Time
}

func (f *fakeReanalysis) Hourly(context.Context, domain.GeoPoint, time.Time, time.Time, ...string) (domain.HourlySeries, error) {
	return domain.HourlySeries{}, errors.New("not used by bioclim")
}

func (f *fakeReanalysis) Daily(_ context.Context, p domain.GeoPoint, first, last time.Time) (domain.DailySeries, error) {
	f.calls++
	f.point, f.first, f.last = p, first, last
	return f.series, f.err
}

// berlinSeries returns a Berlin-like temperate pattern (→ Cfb) with precip
// 50 mm/month.
func berlinSeries() domain.DailySeries {
	pattern := [12]float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}
	var tmin, tmax, precip [12]float64
	for m, v := range pattern {
		tmin[m], tmax[m], precip[m] = v-5, v+5, 50
	}
	return cannedSeries(pattern, tmin, tmax, precip)
}

// cannedSeries builds a daily series for 1991–1992 from monthly normals: every
// day of a month carries the month's temperatures, and the month's
// precipitation total falls on the 15th (0 on the other days).
func cannedSeries(tmean, tmin, tmax, precip [12]float64) domain.DailySeries {
	first := time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)
	s := domain.DailySeries{FirstDay: first}
	for d := first; d.Year() <= 1992; d = d.AddDate(0, 0, 1) {
		m := int(d.Month()) - 1
		pr := 0.0
		if d.Day() == 15 {
			pr = precip[m]
		}
		s.TMean = append(s.TMean, tmean[m])
		s.TMin = append(s.TMin, tmin[m])
		s.TMax = append(s.TMax, tmax[m])
		s.PrecipSum = append(s.PrecipSum, pr)
	}
	return s
}

func newProvider(series domain.DailySeries) (*Provider, *fakeReanalysis) {
	fake := &fakeReanalysis{series: series}
	return New(Options{Reanalysis: fake, Cache: newMemCache()}), fake
}

// req is a Berlin request at 34 m.
func req(refPeriod string) domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 52.5, Lon: 13.4},
		Elevation:  34,
		Instant:    time.Date(2005, 6, 15, 12, 0, 0, 0, time.UTC),
		RefPeriod:  refPeriod,
	}
}

// Port errors are already ProviderErrors (transient for upstream trouble, with
// their retry hint); the provider must hand them on unchanged.
func TestFetch_PassesPortErrorThrough(t *testing.T) {
	portErr := output.ProviderError{Class: output.ClassTransient, Retryable: true, RetryAfter: 42 * time.Second, Err: errors.New("upstream")}
	p := New(Options{Reanalysis: &fakeReanalysis{err: portErr}})

	_, err := p.Fetch(context.Background(), req("1991-1992"))
	var pe output.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("Fetch err = %v, want ProviderError", err)
	}
	if pe != portErr {
		t.Errorf("ProviderError = %+v, want the port's error unchanged", pe)
	}
}

func TestFetch_AsksForTheReferencePeriodAtTheRequestPoint(t *testing.T) {
	p, fake := newProvider(berlinSeries())
	if _, err := p.Fetch(context.Background(), req("1991-1992")); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if want := (domain.GeoPoint{Lat: 52.5, Lon: 13.4, Elevation: 34}); fake.point != want {
		t.Errorf("point = %+v, want %+v", fake.point, want)
	}
	if want := time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC); !fake.first.Equal(want) {
		t.Errorf("first day = %v, want %v", fake.first, want)
	}
	if want := time.Date(1992, 12, 31, 0, 0, 0, 0, time.UTC); !fake.last.Equal(want) {
		t.Errorf("last day = %v, want %v", fake.last, want)
	}
}

// A day with any missing value (NaN) is left out of its month; a month without
// a single complete day means the normals are not available.
func TestMonthlyNormals_MissingValues(t *testing.T) {
	s := berlinSeries()
	// Spoil every January day but one in a way that would shift the mean.
	for i := 1; i < 31; i++ {
		s.TMean[i] = math.NaN()
		s.TMin[i] = 100
	}
	clim, ok := monthlyNormals(s, 2)
	if !ok {
		t.Fatal("monthlyNormals not ok")
	}
	if clim.Tmin[0] != -4 {
		t.Errorf("January Tmin = %v, want -4 (incomplete days skipped)", clim.Tmin[0])
	}

	for i := range s.PrecipSum {
		if s.FirstDay.AddDate(0, 0, i).Month() == time.March {
			s.PrecipSum[i] = math.NaN()
		}
	}
	if _, ok := monthlyNormals(s, 2); ok {
		t.Error("monthlyNormals ok with a month without complete days")
	}
}

func TestFetch_IncompleteNormalsAreNotYetAvailable(t *testing.T) {
	p, _ := newProvider(domain.DailySeries{FirstDay: time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)})
	_, err := p.Fetch(context.Background(), req("1991-1992"))
	var pe output.ProviderError
	if !errors.As(err, &pe) || pe.Class != output.ClassNotYetAvailable {
		t.Errorf("Fetch err = %v, want a not-yet-available ProviderError", err)
	}
}

func TestFetch_ComputesBioAndKoppen(t *testing.T) {
	p, _ := newProvider(berlinSeries())

	res, err := p.Fetch(context.Background(), req("1991-1992"))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if c := res.Feature.Geometry.Coordinates; len(c) != 2 || c[0] != 13.4 || c[1] != 52.5 {
		t.Errorf("geometry = %v, want the request coordinate", res.Feature.Geometry.Coordinates)
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
	lic := res.Feature.License
	if lic.Name != domain.ReanalysisLicense().Name || lic.URL != domain.ReanalysisLicense().URL ||
		!strings.HasPrefix(lic.Attribution, domain.ReanalysisLicense().Attribution) ||
		!strings.Contains(lic.Attribution, "1991-1992") {
		t.Errorf("license = %+v, want the reanalysis license with the period", lic)
	}

	ab, ok := props["altitudinalBelt"].(map[string]any)
	if !ok {
		t.Fatalf("altitudinalBelt missing")
	}
	if belt, ok := ab["belt"].(map[string]any); !ok || belt["de"] == "" || belt["de"] == nil {
		t.Errorf("altitudinalBelt.belt = %v, want bilingual name", ab["belt"])
	}
	if ind, ok := ab["indicators"].(map[string]any); !ok || ind["matC"] == nil {
		t.Errorf("altitudinalBelt.indicators = %v, want matC", ab["indicators"])
	}
	checkBerlinRivasMartinez(t, ab)
}

// checkBerlinRivasMartinez checks the Rivas-Martínez part of the Berlin
// pattern's altitudinal belt.
func checkBerlinRivasMartinez(t *testing.T, ab map[string]any) {
	t.Helper()
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
		t.Errorf("macrobioclimate.elevationM = %v, want 34 (from the request)", mb["elevationM"])
	}
}

func TestFetch_CachesPerCoordinatePeriodElevation(t *testing.T) {
	p, fake := newProvider(berlinSeries())

	if _, err := p.Fetch(context.Background(), req("1991-1992")); err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	res, err := p.Fetch(context.Background(), req("1991-1992"))
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("reanalysis calls = %d, want 1 (second served from cache)", fake.calls)
	}
	if !res.Cached {
		t.Error("second result should be Cached=true")
	}

	// Sub-metre differences share the entry; another whole metre does not.
	r := req("1991-1992")
	r.Elevation = 34.3
	if res, _ := p.Fetch(context.Background(), r); !res.Cached {
		t.Error("34.3 m should share the 34 m entry")
	}
	r.Elevation = 35
	res, err = p.Fetch(context.Background(), r)
	if err != nil {
		t.Fatalf("Fetch at 35 m: %v", err)
	}
	if res.Cached || fake.calls != 2 {
		t.Errorf("35 m: Cached=%v calls=%d, want a fresh fetch", res.Cached, fake.calls)
	}
}

func TestFetch_BadRefPeriod(t *testing.T) {
	p, _ := newProvider(berlinSeries())
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
	fake := &fakeReanalysis{series: berlinSeries()}
	cache := newMemCache()
	p := New(Options{Reanalysis: fake, Cache: cache})
	r := req("1991-1992")

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
	if err := cache.Set(context.Background(), cacheKey(r.PointOf(), startY, endY), raw, time.Hour); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	res, err := p.Fetch(context.Background(), r)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("upstream calls = %d, want 1 — the poisoned entry must not be served", fake.calls)
	}
	if res.Cached {
		t.Error("result reported as cached although the cached entry was rejected")
	}
	if err := res.Feature.License.Validate(); err != nil {
		t.Errorf("served a feature that still fails validation: %v", err)
	}
}
