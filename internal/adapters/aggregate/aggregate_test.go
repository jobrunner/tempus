package aggregate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

const dailyJSON = `{"latitude":49.8,"longitude":9.94,"daily":{
  "time":["2025-06-13","2025-06-14","2025-06-15"],
  "temperature_2m_max":[20,22,24],
  "temperature_2m_min":[10,12,14],
  "temperature_2m_mean":[15,17,19]}}`

const hourlyJSON = `{"hourly":{
  "time":["2025-06-15T10:00","2025-06-15T11:00","2025-06-15T12:00","2025-06-15T13:00"],
  "precipitation":[1.0,2.0,0.0,3.0]}}`

func newProvider(t *testing.T) (*Provider, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("daily") != "" {
			_, _ = w.Write([]byte(dailyJSON))
			return
		}
		_, _ = w.Write([]byte(hourlyJSON))
	}))
	p := New(Options{
		ArchiveBaseURL:  srv.URL,
		ForecastBaseURL: srv.URL,
		Timeout:         2 * time.Second,
		ArchiveDelay:    5 * 24 * time.Hour,
		Clock:           fixedClock{time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
	})
	return p, srv.Close
}

func req(instant time.Time, base *float64) domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate:     domain.Coordinate{Lat: 49.79, Lon: 9.93},
		Instant:        instant,
		GDDBaseCelsius: base,
	}
}

func TestFetch_ComputesAggregates(t *testing.T) {
	p, done := newProvider(t)
	defer done()

	res, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC), nil))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if props["kind"] != "aggregate" {
		t.Errorf("kind = %v, want aggregate", props["kind"])
	}

	precip, ok := props["precipitation"].(map[string]any)
	if !ok {
		t.Fatalf("precipitation missing: %v", props["precipitation"])
	}
	if precip["last24h"] != 6.0 { // 1+2+0+3
		t.Errorf("last24h = %v, want 6", precip["last24h"])
	}

	day, ok := props["temperatureDay"].(map[string]any)
	if !ok {
		t.Fatalf("temperatureDay missing")
	}
	if day["min"] != 14.0 || day["max"] != 24.0 || day["mean"] != 19.0 {
		t.Errorf("temperatureDay = %v, want min14/max24/mean19", day)
	}

	gdd, ok := props["growingDegreeDays"].(map[string]any)
	if !ok {
		t.Fatalf("growingDegreeDays missing")
	}
	// base 10: (5)+(7)+(9) = 21; base 5: (10)+(12)+(14) = 36. Both always present.
	if gdd["base10"] != 21.0 {
		t.Errorf("gdd base10 = %v, want 21", gdd["base10"])
	}
	if gdd["base5"] != 36.0 {
		t.Errorf("gdd base5 = %v, want 36", gdd["base5"])
	}
	if gdd["since"] != "2025-01-01" {
		t.Errorf("since = %v, want 2025-01-01", gdd["since"])
	}
	if gdd["days"] != 3 {
		t.Errorf("days = %v, want 3", gdd["days"])
	}
	if _, hasCustom := gdd["custom"]; hasCustom {
		t.Errorf("custom present without a gddBase override: %v", gdd["custom"])
	}

	att, ok := props["arrheniusThermalTime"].(map[string]any)
	if !ok {
		t.Fatalf("arrheniusThermalTime missing")
	}
	if att["referenceTempC"] != 20.0 || att["activationEnergyEv"] != 0.65 || att["days"] != 3 {
		t.Errorf("arrhenius meta = %v, want Tref 20 / E 0.65 / days 3", att)
	}
	if v, ok := att["value"].(float64); !ok || v <= 0 {
		t.Errorf("arrhenius value = %v, want > 0", att["value"])
	}

	if res.Feature.License.Attribution == "" {
		t.Error("attribution empty")
	}
}

func TestFetch_GDDBaseOverride(t *testing.T) {
	p, done := newProvider(t)
	defer done()

	base := 7.0 // distinct from the fixed 5 and 10
	res, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC), &base))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	gdd := res.Feature.Properties["growingDegreeDays"].(map[string]any)
	// base5/base10 remain; the override is added under "custom".
	if gdd["base5"] != 36.0 || gdd["base10"] != 21.0 {
		t.Errorf("gdd base5/base10 = %v/%v, want 36/21", gdd["base5"], gdd["base10"])
	}
	custom, ok := gdd["custom"].(map[string]any)
	if !ok {
		t.Fatalf("custom missing for gddBase=7")
	}
	// base 7: (15-7)+(17-7)+(19-7) = 8+10+12 = 30.
	if custom["baseCelsius"] != 7.0 || custom["value"] != 30.0 {
		t.Errorf("custom = %v, want baseCelsius 7 / value 30", custom)
	}
}

func TestFetch_FutureRejected(t *testing.T) {
	p, done := newProvider(t)
	defer done()

	_, err := p.Fetch(context.Background(), req(time.Date(2026, 7, 21, 13, 0, 0, 0, time.UTC), nil))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Retryable {
		t.Fatalf("want non-retryable ProviderError for future date, got %v", err)
	}
}

func TestProviderIDKind(t *testing.T) {
	p := New(Options{Clock: fixedClock{time.Now().UTC()}})
	if p.ID() != "aggregate" || p.Kind() != "aggregate" {
		t.Errorf("ID/Kind = %q/%q, want aggregate/aggregate", p.ID(), p.Kind())
	}
	if p.Attribution().Attribution == "" {
		t.Error("attribution empty")
	}
}
