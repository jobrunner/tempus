package domain

import (
	"math"
	"testing"
	"time"
)

// Meeus, Astronomical Algorithms, 2nd ed., example 47.a: 1992 April 12, 0h TD.
// Expected geocentric λ = 133.162655°, β = −3.229126°, Δ = 368409.7 km.
func TestMoonEcliptic_MeeusExample(t *testing.T) {
	instant := time.Date(1992, 4, 12, 0, 0, 0, 0, time.UTC)
	lambda, beta, dist := moonEcliptic(instant)
	if math.Abs(lambda-133.162655) > 0.02 {
		t.Errorf("λ = %.6f°, want 133.162655°", lambda)
	}
	if math.Abs(beta-(-3.229126)) > 0.02 {
		t.Errorf("β = %.6f°, want -3.229126°", beta)
	}
	if math.Abs(dist-368409.7) > 50 {
		t.Errorf("Δ = %.1f km, want 368409.7 km", dist)
	}
}

func TestMoonPhaseName(t *testing.T) {
	cases := []struct {
		elong float64
		de    string
	}{
		{0, phaseNewMoonDE},
		{45, "zunehmende Sichel"},
		{90, "erstes Viertel"},
		{135, "zunehmender Mond"},
		{180, phaseFullMoonDE},
		{225, "abnehmender Mond"},
		{270, "letztes Viertel"},
		{315, "abnehmende Sichel"},
		{359, phaseNewMoonDE},
	}
	for _, tc := range cases {
		if de, _ := moonPhaseName(tc.elong); de != tc.de {
			t.Errorf("moonPhaseName(%.0f) = %q, want %q", tc.elong, de, tc.de)
		}
	}
}

// A known full moon (2025-06-11, ~07:44 UTC) is ~100% illuminated and named
// "Vollmond"; a known new moon (2025-06-25, ~10:31 UTC) is near 0%.
func TestMoonPhase_KnownFullAndNew(t *testing.T) {
	illumFull, _, _, deFull, _ := MoonPhase(time.Date(2025, 6, 11, 7, 44, 0, 0, time.UTC))
	if illumFull < 98 {
		t.Errorf("full-moon illumination = %.1f%%, want >98%%", illumFull)
	}
	if deFull != phaseFullMoonDE {
		t.Errorf("full-moon phase = %q, want Vollmond", deFull)
	}
	illumNew, _, _, deNew, _ := MoonPhase(time.Date(2025, 6, 25, 10, 31, 0, 0, time.UTC))
	if illumNew > 2 {
		t.Errorf("new-moon illumination = %.1f%%, want <2%%", illumNew)
	}
	if deNew != phaseNewMoonDE {
		t.Errorf("new-moon phase = %q, want Neumond", deNew)
	}
}

// The rise/set solver is self-consistent: the moon's altitude at the computed
// moonrise/moonset equals the standard altitude threshold.
func TestMoonTimes_CrossingSelfConsistent(t *testing.T) {
	rise, set := MoonTimes(52.52, 13.405, time.Date(2025, 6, 11, 12, 0, 0, 0, time.UTC))
	if rise == nil && set == nil {
		t.Fatal("expected at least one moon event")
	}
	for _, ev := range []*time.Time{rise, set} {
		if ev == nil {
			continue
		}
		alt, _ := moonAltAzGeo(52.52, 13.405, *ev)
		if math.Abs(alt-moonStandardAltDeg) > 0.5 {
			t.Errorf("altitude at moon event = %.3f°, want ≈ %.3f°", alt, moonStandardAltDeg)
		}
	}
}

// Illumination stays within [0,100] and the moon's distance is within the real
// perigee/apogee envelope for an arbitrary instant.
func TestMoonPhase_Bounds(t *testing.T) {
	instant := time.Date(2026, 7, 23, 18, 0, 0, 0, time.UTC)
	illum, angle, age, _, _ := MoonPhase(instant)
	if illum < 0 || illum > 100 {
		t.Errorf("illumination = %.2f%%, out of [0,100]", illum)
	}
	if angle < 0 || angle > 180 {
		t.Errorf("phase angle = %.2f°, out of [0,180]", angle)
	}
	if age < 0 || age > synodicMonthDays {
		t.Errorf("age = %.2f d, out of [0,%.2f]", age, synodicMonthDays)
	}
	_, _, dist := MoonPosition(52.52, 13.405, instant)
	if dist < 356000 || dist > 407000 {
		t.Errorf("distance = %.0f km, outside perigee/apogee envelope", dist)
	}
}
