package dewpoint_test

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/dewpoint"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

func weatherFeature(temp any, rh any) domain.Feature {
	props := map[string]any{
		"kind":          "weather",
		"provider":      "open-meteo",
		"observedAt":    "2025-06-15T13:00:00Z",
		"temperature2m": temp,
	}
	if rh != nil {
		props["relativeHumidity2m"] = rh
	}
	return domain.Feature{
		Type:       "Feature",
		Geometry:   domain.Geometry{Type: "Point", Coordinates: []float64{9.93, 49.79}},
		Properties: props,
		License: domain.License{
			Name:        "CC-BY 4.0",
			URL:         "https://open-meteo.com/en/license",
			Attribution: "Weather data by Open-Meteo.com",
		},
	}
}

func sampleReq() domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93},
		Instant:    time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC),
	}
}

func TestDeriver_IntRH(t *testing.T) {
	d := dewpoint.New()
	src := weatherFeature(21.4, int(55))
	feats, err := d.Derive(context.Background(), sampleReq(), []domain.Feature{src})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feats) != 1 {
		t.Fatalf("want 1 feature, got %d", len(feats))
	}
	f := feats[0]
	dp, ok := f.Properties["dewPoint2m"].(float64)
	if !ok {
		t.Fatalf("dewPoint2m not float64: %T %v", f.Properties["dewPoint2m"], f.Properties["dewPoint2m"])
	}
	if math.Abs(dp-12.0) > 0.5 {
		t.Errorf("dewPoint2m = %v, want ≈12.0 (±0.5)", dp)
	}
	if !strings.Contains(f.License.Attribution, "Weather data by Open-Meteo.com") {
		t.Errorf("license attribution missing source: %q", f.License.Attribution)
	}
}

func TestDeriver_Float64RH(t *testing.T) {
	d := dewpoint.New()
	src := weatherFeature(21.4, float64(55))
	feats, err := d.Derive(context.Background(), sampleReq(), []domain.Feature{src})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feats) != 1 {
		t.Fatalf("want 1 feature, got %d", len(feats))
	}
	dp, ok := feats[0].Properties["dewPoint2m"].(float64)
	if !ok {
		t.Fatalf("dewPoint2m not float64")
	}
	if math.Abs(dp-12.0) > 0.5 {
		t.Errorf("dewPoint2m = %v, want ≈12.0 (±0.5)", dp)
	}
}

func TestDeriver_NoWeatherSource(t *testing.T) {
	d := dewpoint.New()
	_, err := d.Derive(context.Background(), sampleReq(), nil)
	if err == nil {
		t.Fatal("expected error for no sources")
	}
	pe, ok := output.AsProviderError(err)
	if !ok {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if !pe.Retryable {
		t.Error("no-source error should be retryable (NotYetAvailable)")
	}
}

func TestDeriver_SourcePresentButMissingRH(t *testing.T) {
	d := dewpoint.New()
	// Pass nil for rh so weatherFeature omits relativeHumidity2m.
	src := weatherFeature(21.4, nil)
	_, err := d.Derive(context.Background(), sampleReq(), []domain.Feature{src})
	if err == nil {
		t.Fatal("expected error for missing rh")
	}
	pe, ok := output.AsProviderError(err)
	if !ok {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Retryable {
		t.Error("missing-field error should not be retryable (Permanent)")
	}
}
