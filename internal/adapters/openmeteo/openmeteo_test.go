package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func req(instant time.Time) domain.QueryRequest {
	return domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93}, Instant: instant}
}

func newProvider(t *testing.T, handler http.HandlerFunc) (*Provider, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	p := New(Options{
		ArchiveBaseURL:  srv.URL,
		ForecastBaseURL: srv.URL,
		Timeout:         2 * time.Second,
		ArchiveDelay:    5 * 24 * time.Hour,
		Clock:           fixedClock{time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
	})
	return p, srv.Close
}

func TestFetch_SelectsHourAndAttributes(t *testing.T) {
	body, _ := os.ReadFile("testdata/archive_ok.json")
	p, done := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("latitude") == "" {
			t.Errorf("missing latitude param: %s", r.URL.RawQuery)
		}
		_, _ = w.Write(body)
	})
	defer done()

	res, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := res.Feature.Properties["temperature2m"]; got != 21.4 {
		t.Errorf("temperature2m = %v, want 21.4", got)
	}
	if res.Feature.Properties["isDay"] != true {
		t.Errorf("isDay = %v, want true (solar-computed)", res.Feature.Properties["isDay"])
	}
	if src, ok := res.Feature.Properties["isDaySource"].(string); !ok || src == "" {
		t.Errorf("isDaySource = %v, want non-empty string", res.Feature.Properties["isDaySource"])
	}
	// geometry uses the provider-resolved grid cell.
	if c := res.Feature.Geometry.Coordinates; c[0] != 9.94 || c[1] != 49.8 {
		t.Errorf("geometry = %v, want [9.94 49.8]", c)
	}
	if res.Feature.License.Attribution == "" || res.Feature.License.Name == "" {
		t.Error("feature must carry attribution")
	}
	if res.Feature.License.URL == "" {
		t.Error("feature must carry license URL")
	}
}

func TestFetch_MissingHourIsNotYetAvailable(t *testing.T) {
	p, done := newProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"latitude":49.8,"longitude":9.94,"hourly_units":{},"hourly":{"time":["2025-06-15T13:00"],"temperature_2m":[null]}}`))
	})
	defer done()
	_, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassNotYetAvailable {
		t.Fatalf("want not-yet-available, got %v", err)
	}
}

func TestFetch_ServerErrorIsTransient(t *testing.T) {
	p, done := newProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	defer done()
	_, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassTransient {
		t.Fatalf("want transient, got %v", err)
	}
}

func TestFetch_WeatherCodeDescription(t *testing.T) {
	// archive_ok.json at 13:00 has weather_code=3 → "Bedeckt"/"Overcast"
	body, _ := os.ReadFile("testdata/archive_ok.json")
	p, done := newProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	})
	defer done()

	res, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	desc, ok := res.Feature.Properties["weatherCodeDescription"].(map[string]string)
	if !ok {
		t.Fatalf("weatherCodeDescription missing or wrong type: %T %v",
			res.Feature.Properties["weatherCodeDescription"],
			res.Feature.Properties["weatherCodeDescription"])
	}
	if desc["de"] != "Bedeckt" {
		t.Errorf("weatherCodeDescription.de = %q, want %q", desc["de"], "Bedeckt")
	}
	if desc["en"] != "Overcast" {
		t.Errorf("weatherCodeDescription.en = %q, want %q", desc["en"], "Overcast")
	}

	src, ok := res.Feature.Properties["weatherCodeSource"].(string)
	if !ok || src == "" {
		t.Errorf("weatherCodeSource missing or empty: %v", res.Feature.Properties["weatherCodeSource"])
	}
}
