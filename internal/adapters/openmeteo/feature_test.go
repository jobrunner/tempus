package openmeteo

import (
	"encoding/json"
	"testing"
)

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
			props := map[string]any{"weatherCode": code}
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
			props := map[string]any{"weatherCode": code}
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

// hourlyValues distinguishes two ways the primary variable can fail to produce a
// value, and they are NOT treated the same. This pins today's behaviour so the
// difference is a decision on record rather than an accident.
func TestHourlyValues_PrimaryVariableHandling(t *testing.T) {
	tests := []struct {
		name    string
		hourly  string
		wantOK  bool
		wantKey bool // is temperature2m in the result?
	}{
		{
			name:    "present and set",
			hourly:  `{"temperature_2m":[21.4]}`,
			wantOK:  true,
			wantKey: true,
		},
		{
			name: "present but null — the hour exists and the provider has not " +
				"filled it in yet, so the caller reports not-yet-available",
			hourly:  `{"temperature_2m":[null]}`,
			wantOK:  false,
			wantKey: false,
		},
		{
			name: "key absent entirely — a malformed response for a request that " +
				"always asks for temperature; reported as available without it",
			hourly:  `{"cloud_cover":[60]}`,
			wantOK:  true,
			wantKey: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var hourly map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.hourly), &hourly); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			values, _, ok := hourlyValues(apiResponse{Hourly: hourly}, 0)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if _, present := values["temperature2m"]; present != tc.wantKey {
				t.Errorf("temperature2m present = %v, want %v", present, tc.wantKey)
			}
		})
	}
}
