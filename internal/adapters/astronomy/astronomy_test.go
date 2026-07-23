package astronomy

import (
	"context"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

func testReq(instant time.Time) domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 52.52, Lon: 13.405},
		Instant:    instant,
	}
}

func TestSunProvider_Fetch(t *testing.T) {
	p := NewSun()
	if p.ID() != "sun" || p.Kind() != "sun" {
		t.Fatalf("ID/Kind = %q/%q, want sun/sun", p.ID(), p.Kind())
	}
	if p.Attribution().Attribution == "" {
		t.Error("attribution must not be empty")
	}

	res, err := p.Fetch(context.Background(), testReq(time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if props["kind"] != "sun" {
		t.Errorf("kind = %v, want sun", props["kind"])
	}
	for _, key := range []string{keyElevationDeg, keyAzimuthDeg, keyZenithDeg, "lightPhase", "sunrise", "sunset", "twilight"} {
		if _, ok := props[key]; !ok {
			t.Errorf("missing property %q", key)
		}
	}
	if lp, ok := props["lightPhase"].(map[string]string); !ok || lp["de"] == "" {
		t.Errorf("lightPhase = %v, want bilingual map", props["lightPhase"])
	}
	tw, ok := props["twilight"].(map[string]any)
	if !ok {
		t.Fatalf("twilight = %v, want nested map", props["twilight"])
	}
	if _, ok := tw["civil"]; !ok {
		t.Error("twilight.civil missing")
	}
	// Attribution must be attached to the feature.
	if res.Feature.License.Attribution == "" {
		t.Error("feature license attribution empty")
	}
}

func TestMoonProvider_Fetch(t *testing.T) {
	p := NewMoon()
	if p.ID() != "moon" || p.Kind() != "moon" {
		t.Fatalf("ID/Kind = %q/%q, want moon/moon", p.ID(), p.Kind())
	}

	res, err := p.Fetch(context.Background(), testReq(time.Date(2025, 6, 11, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if props["kind"] != "moon" {
		t.Errorf("kind = %v, want moon", props["kind"])
	}
	for _, key := range []string{keyElevationDeg, keyAzimuthDeg, keyDistanceKm, keyIllumination, keyPhaseAngle, keyAgeDays, "phase"} {
		if _, ok := props[key]; !ok {
			t.Errorf("missing property %q", key)
		}
	}
	if ph, ok := props["phase"].(map[string]string); !ok || ph["en"] == "" {
		t.Errorf("phase = %v, want bilingual map", props["phase"])
	}
}

// Astronomy providers must serve future dates without error (unlike weather).
func TestProviders_FutureWorks(t *testing.T) {
	future := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := NewSun().Fetch(context.Background(), testReq(future)); err != nil {
		t.Errorf("sun future Fetch: %v", err)
	}
	if _, err := NewMoon().Fetch(context.Background(), testReq(future)); err != nil {
		t.Errorf("moon future Fetch: %v", err)
	}
}
