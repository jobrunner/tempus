package bioclim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
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
// years, following a Berlin-like temperate pattern (→ Cfb), precip 50 mm/month.
func berlinBody() []byte {
	pattern := []float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}
	var times []string
	var tmax, tmin, tmean, precip []float64
	for _, y := range []int{1991, 1992} {
		for m := 1; m <= 12; m++ {
			times = append(times, fmt.Sprintf("%d-%02d-15", y, m))
			v := pattern[m-1]
			tmean = append(tmean, v)
			tmin = append(tmin, v-5)
			tmax = append(tmax, v+5)
			precip = append(precip, 50)
		}
	}
	body, _ := json.Marshal(map[string]any{
		"latitude": 52.5, "longitude": 13.4,
		"daily": map[string]any{
			"time": times, "temperature_2m_max": tmax, "temperature_2m_min": tmin,
			"temperature_2m_mean": tmean, "precipitation_sum": precip,
		},
	})
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
	if tt, ok := ab["thermotype"].(map[string]any); !ok || tt[keyCode] == "" || tt[keyCode] == nil {
		t.Errorf("altitudinalBelt.thermotype = %v, want a code", ab["thermotype"])
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
