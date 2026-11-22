package domain

import (
	"math"
	"testing"
	"time"
)

// Issue #102: events belong to the solar day the instant falls into at the
// coordinate, not to the UTC calendar day.

func TestSolarEvents_EastOfGreenwichLateUTC(t *testing.T) {
	// Thailand, 19 May 06:30 local = 18 May 23:30 UTC.
	q := time.Date(2025, 5, 18, 23, 30, 0, 0, time.UTC)
	ev := SolarEvents(15, 100, q)
	if ev.Sunrise == nil || ev.CivilDawn == nil || ev.CivilDusk == nil {
		t.Fatal("expected sunrise and civil twilight")
	}
	if y, m, d := ev.Sunrise.Date(); y != 2025 || m != 5 || d != 18 || ev.Sunrise.Hour() < 22 || !ev.Sunrise.Before(q) {
		t.Errorf("sunrise = %v, want 18 May late UTC, before the query (19 May morning local)", ev.Sunrise)
	}
	if q.Before(*ev.CivilDawn) || q.After(*ev.CivilDusk) {
		t.Errorf("query %v not between civil dawn %v and dusk %v", q, ev.CivilDawn, ev.CivilDusk)
	}
}

func TestSolarEvents_WestOfGreenwichEarlyUTC(t *testing.T) {
	// Mexico, summer evening 18:30 local (UTC-6) = 00:30 UTC the next day.
	q := time.Date(2025, 6, 21, 0, 30, 0, 0, time.UTC)
	ev := SolarEvents(25, -100, q)
	if ev.Sunset == nil {
		t.Fatal("expected a sunset")
	}
	if !ev.Sunset.After(q) {
		t.Errorf("sunset %v must follow the query %v", ev.Sunset, q)
	}
	if ev.Sunset.Sub(q) > 3*time.Hour {
		t.Errorf("sunset %v is not the evening of the query %v", ev.Sunset, q)
	}
}

func TestSolarEvents_CentralEuropeNoonUnchanged(t *testing.T) {
	q := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	ev := SolarEvents(50, 10, q)
	if ev.SolarNoon == nil || ev.SolarNoon.Day() != 21 {
		t.Fatalf("solar noon = %v, want 21 June", ev.SolarNoon)
	}
	if d := ev.SolarNoon.Sub(q); d < -time.Hour || d > 0 {
		t.Errorf("solar noon %v, want shortly before 12:00 UTC at lon 10", ev.SolarNoon)
	}
}

// TestSolarEvents_NoonWithinTwelveHours is the rule itself, over the whole
// longitude range and around the clock.
func TestSolarEvents_NoonWithinTwelveHours(t *testing.T) {
	base := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	for lon := -180.0; lon <= 180; lon += 15 {
		for h := range 48 {
			q := base.Add(time.Duration(h)*30*time.Minute + 7*time.Minute)
			ev := SolarEvents(40, lon, q)
			if d := math.Abs(ev.SolarNoon.Sub(q).Hours()); d > 12 {
				t.Fatalf("lon %v at %v: solar noon %v is %.2f h away", lon, q, ev.SolarNoon, d)
			}
		}
	}
}

func TestSolarEvents_PolarNightHasNoEvents(t *testing.T) {
	ev := SolarEvents(80, 120, time.Date(2025, 12, 21, 23, 0, 0, 0, time.UTC))
	if ev.Sunrise != nil || ev.Sunset != nil || ev.SolarNoon == nil {
		t.Errorf("polar night: sunrise %v sunset %v noon %v", ev.Sunrise, ev.Sunset, ev.SolarNoon)
	}
}

func TestMoonTimes_SearchTheLocalDay(t *testing.T) {
	cases := []struct {
		lat, lon float64
		q        time.Time
	}{
		{15, 100, time.Date(2025, 5, 18, 23, 30, 0, 0, time.UTC)},
		{25, -100, time.Date(2025, 6, 21, 0, 30, 0, 0, time.UTC)},
		{50, 10, time.Date(2025, 6, 11, 12, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		noon := *SolarEvents(c.lat, c.lon, c.q).SolarNoon
		from, to := noon.Add(-12*time.Hour), noon.Add(12*time.Hour)
		rise, set := MoonTimes(c.lat, c.lon, c.q)
		if rise == nil && set == nil {
			t.Errorf("lon %v: no moon event", c.lon)
		}
		for _, ev := range []*time.Time{rise, set} {
			if ev != nil && (ev.Before(from) || !ev.Before(to)) {
				t.Errorf("lon %v: moon event %v outside the local day [%v, %v)", c.lon, ev, from, to)
			}
		}
	}
}
