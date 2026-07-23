package domain

import (
	"math"
	"testing"
	"time"
)

// At solar noon in the northern hemisphere the sun is due south (~180°).
func TestSunAltAz_NoonAzimuthSouth(t *testing.T) {
	// Berlin, summer solstice, solar noon is ~11:00 UTC (lon 13.4°E, eqTime small).
	events := SolarEvents(52.52, 13.405, time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC))
	if events.SolarNoon == nil {
		t.Fatal("expected a solar noon")
	}
	elev, az := SunAltAz(52.52, 13.405, *events.SolarNoon)
	if math.Abs(az-180) > 3 {
		t.Errorf("noon azimuth = %.2f°, want ~180°", az)
	}
	// Max elevation at noon ≈ 90 - |lat - decl| ≈ 90 - (52.52 - 23.44) ≈ 60.9°.
	if elev < 58 || elev > 63 {
		t.Errorf("noon elevation = %.2f°, want ~61°", elev)
	}
}

// Azimuth grows through the day: morning sun in the east (<180°), afternoon in
// the west (>180°) for a northern-hemisphere site.
func TestSunAltAz_MorningEastAfternoonWest(t *testing.T) {
	_, azMorning := SunAltAz(52.52, 13.405, time.Date(2025, 6, 21, 6, 0, 0, 0, time.UTC))
	_, azAfternoon := SunAltAz(52.52, 13.405, time.Date(2025, 6, 21, 16, 0, 0, 0, time.UTC))
	if azMorning >= 180 {
		t.Errorf("morning azimuth = %.1f°, want < 180° (east)", azMorning)
	}
	if azAfternoon <= 180 {
		t.Errorf("afternoon azimuth = %.1f°, want > 180° (west)", azAfternoon)
	}
}

// The rise/set solver is self-consistent: the sun's elevation at the computed
// sunrise/sunset equals the −0.833° threshold.
func TestSolarEvents_CrossingSelfConsistent(t *testing.T) {
	events := SolarEvents(52.52, 13.405, time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC))
	if events.Sunrise == nil || events.Sunset == nil {
		t.Fatal("expected sunrise and sunset")
	}
	for _, ev := range []*time.Time{events.Sunrise, events.Sunset} {
		if elev := SolarElevationDeg(52.52, 13.405, *ev); math.Abs(elev-altSunrise) > 0.5 {
			t.Errorf("elevation at event = %.3f°, want ≈ %.3f°", elev, altSunrise)
		}
	}
	// Berlin midsummer day length is ~16h40m.
	if events.DayLengthMinutes < 960 || events.DayLengthMinutes > 1020 {
		t.Errorf("day length = %.0f min, want ~1000 (16h40m)", events.DayLengthMinutes)
	}
}

// Twilight phases are ordered: astronomical dawn precedes nautical, which
// precedes civil, which precedes sunrise.
func TestSolarEvents_TwilightOrdering(t *testing.T) {
	events := SolarEvents(52.52, 13.405, time.Date(2025, 3, 20, 12, 0, 0, 0, time.UTC))
	seq := []*time.Time{events.AstronomicalDawn, events.NauticalDawn, events.CivilDawn, events.Sunrise}
	for _, e := range seq {
		if e == nil {
			t.Fatal("expected all dawn events at equinox in Berlin")
		}
	}
	for i := 1; i < len(seq); i++ {
		if !seq[i].After(*seq[i-1]) {
			t.Errorf("dawn events out of order at index %d", i)
		}
	}
}

// Near an equinox at the equator, day length is ~12h and sunrise ~06:00 UTC.
func TestSolarEvents_EquatorEquinox(t *testing.T) {
	events := SolarEvents(0, 0, time.Date(2025, 3, 20, 12, 0, 0, 0, time.UTC))
	if events.Sunrise == nil || events.Sunset == nil {
		t.Fatal("expected sunrise and sunset")
	}
	if math.Abs(events.DayLengthMinutes-720) > 20 {
		t.Errorf("day length = %.0f min, want ~720 (12h)", events.DayLengthMinutes)
	}
	if h := events.Sunrise.UTC().Hour(); h != 5 && h != 6 {
		t.Errorf("sunrise hour = %d UTC, want ~06:00", h)
	}
}

// Polar day: no sunset above the Arctic Circle at summer solstice.
func TestSolarEvents_PolarDay(t *testing.T) {
	events := SolarEvents(78.22, 15.63, time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)) // Svalbard
	if events.Sunrise != nil || events.Sunset != nil {
		t.Errorf("expected no sunrise/sunset during polar day, got rise=%v set=%v",
			events.Sunrise, events.Sunset)
	}
}

func TestSunLightPhase(t *testing.T) {
	cases := []struct {
		elev float64
		de   string
	}{
		{10, "Tag"},
		{-3, "bürgerliche Dämmerung"},
		{-9, "nautische Dämmerung"},
		{-15, "astronomische Dämmerung"},
		{-30, "Nacht"},
	}
	for _, tc := range cases {
		if de, _ := SunLightPhase(tc.elev); de != tc.de {
			t.Errorf("SunLightPhase(%.0f) = %q, want %q", tc.elev, de, tc.de)
		}
	}
}
