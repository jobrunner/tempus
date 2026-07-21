package domain

import "testing"

func TestNewPointFeatureShape(t *testing.T) {
	lic := License{Name: "CC-BY 4.0", URL: "https://x", Attribution: "by X"}
	f := NewPointFeature(Coordinate{Lat: 49.79, Lon: 9.93}, map[string]any{"temperature2m": 21.4}, lic)

	if f.Type != "Feature" {
		t.Errorf("Type = %q, want Feature", f.Type)
	}
	if f.Geometry.Type != "Point" {
		t.Errorf("Geometry.Type = %q, want Point", f.Geometry.Type)
	}
	// GeoJSON order is [lon, lat].
	if got := f.Geometry.Coordinates; len(got) != 2 || got[0] != 9.93 || got[1] != 49.79 {
		t.Errorf("Coordinates = %v, want [9.93 49.79]", got)
	}
	if f.License.Attribution == "" {
		t.Error("feature must carry attribution")
	}
}
