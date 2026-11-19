package openmeteo

import "testing"

// A weatherCode of an unexpected type must not be interpreted. The old code
// left the parsed code at 0 in that case, which would have attached the
// description for code 0 ("Klarer Himmel" / "Clear sky") to a value it could
// not read. Unreachable through Fetch today — normalize always yields int — so
// this test is what keeps the guard from being "simplified" back out.
func TestEnrichWeatherCode_UnexpectedTypeIsIgnored(t *testing.T) {
	for name, code := range map[string]any{
		"string": "3",
		"bool":   true,
		"nil":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			props := map[string]any{propWeatherCode: code}
			enrichWeatherCode(props)
			for _, key := range []string{"weatherCodeDescription", "weatherCodeSource", "weatherCodeSourceURL"} {
				if v, present := props[key]; present {
					t.Errorf("%s = %v, want absent for a %s weatherCode", key, v, name)
				}
			}
		})
	}
}

func TestEnrichWeatherCode_AcceptsIntAndFloat(t *testing.T) {
	// The wire value is an int after normalize, but a float64 arrives when a
	// caller hands over raw JSON numbers, so both are accepted.
	for name, code := range map[string]any{"int": 3, "float64": float64(3)} {
		t.Run(name, func(t *testing.T) {
			props := map[string]any{propWeatherCode: code}
			enrichWeatherCode(props)
			desc, ok := props["weatherCodeDescription"].(map[string]string)
			if !ok {
				t.Fatalf("weatherCodeDescription missing for a %s code: %v", name, props)
			}
			if desc["de"] != "Bedeckt" {
				t.Errorf("weatherCodeDescription.de = %q, want %q", desc["de"], "Bedeckt")
			}
		})
	}
}

// The provider always reports km/h, but enrichBeaufort converts whatever unit
// it is handed and refuses to guess when the unit is unknown or missing.
func TestEnrichBeaufort_Units(t *testing.T) {
	tests := []struct {
		name  string
		speed float64
		units map[string]string
		want  any // nil = no Beaufort properties
	}{
		{unitKmh, 12.1, map[string]string{propWindSpeed: unitKmh}, 3},
		{"mph is converted (20 mph ≈ 32 km/h)", 20, map[string]string{propWindSpeed: "mph"}, 5},
		{"unknown unit is left unclassified", 20, map[string]string{propWindSpeed: "furlongs/fortnight"}, nil},
		{"missing unit is left unclassified", 500, map[string]string{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			props := map[string]any{propWindSpeed: tc.speed}
			enrichBeaufort(props, tc.units)
			if got := props["windBeaufort"]; got != tc.want {
				t.Errorf("windBeaufort = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnrichBeaufort_NoWindSpeed(t *testing.T) {
	props := map[string]any{}
	enrichBeaufort(props, map[string]string{propWindSpeed: unitKmh})
	if v, present := props["windBeaufort"]; present {
		t.Errorf("windBeaufort = %v, want absent without a wind speed", v)
	}
}

func TestDisplayRound_MatchesFloat32Reference(t *testing.T) {
	tests := []struct {
		x        float64
		decimals int
		want     float64
	}{
		{0.05, 1, 0.1},   // and not float32 noise like 0.10000000149
		{-0.25, 1, -0.3}, // half away from zero, not to even
		{21.44, 1, 21.4},
		{72.5, 0, 73},
		{3, 0, 3},
	}
	for _, tc := range tests {
		if got := displayRound(tc.x, tc.decimals); got != tc.want {
			t.Errorf("displayRound(%v, %d) = %v, want %v", tc.x, tc.decimals, got, tc.want)
		}
	}
}
