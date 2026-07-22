package domain

import (
	"testing"
	"time"
)

func TestIsDaylight(t *testing.T) {
	cases := []struct {
		name    string
		lat     float64
		lon     float64
		t       time.Time
		wantDay bool
		// elevSign: +1 means we expect elev > 0, -1 means < 0; 0 means don't check
		elevSign  int
		elevAbove float64 // only checked when elevSign == +1
	}{
		{
			name: "historical midday 1954 is daylight",
			lat:  49, lon: 10,
			t:         time.Date(1954, 7, 23, 11, 0, 0, 0, time.UTC),
			wantDay:   true,
			elevSign:  +1,
			elevAbove: 30,
		},
		{
			name: "summer solstice midday is daylight",
			lat:  49, lon: 10,
			t:         time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC),
			wantDay:   true,
			elevSign:  +1,
			elevAbove: 50,
		},
		{
			name: "winter solstice midnight is night",
			lat:  49, lon: 10,
			t:        time.Date(2025, 12, 21, 23, 0, 0, 0, time.UTC),
			wantDay:  false,
			elevSign: -1,
		},
		{
			name: "summer midnight is night",
			lat:  49, lon: 10,
			t:        time.Date(2025, 6, 21, 0, 0, 0, 0, time.UTC),
			wantDay:  false,
			elevSign: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			elev := SolarElevationDeg(tc.lat, tc.lon, tc.t)
			day := IsDaylight(tc.lat, tc.lon, tc.t)

			if day != tc.wantDay {
				t.Errorf("IsDaylight = %v, want %v (elevation=%.2f°)", day, tc.wantDay, elev)
			}
			if tc.elevSign == +1 && elev <= tc.elevAbove {
				t.Errorf("elevation = %.2f°, want > %.0f°", elev, tc.elevAbove)
			}
			if tc.elevSign == -1 && elev >= 0 {
				t.Errorf("elevation = %.2f°, want < 0°", elev)
			}
		})
	}
}
