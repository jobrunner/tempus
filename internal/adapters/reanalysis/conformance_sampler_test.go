package reanalysis

import (
	"reflect"
	"testing"
	"time"
)

// The sampler and the windows of the live sweep run in every test pass, so a
// change to them shows without network access.

func TestConformancePoints_DeterministicAndInsideTheRegion(t *testing.T) {
	a, b := conformancePoints(50, conformanceSeed), conformancePoints(50, conformanceSeed)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed, different points")
	}
	if reflect.DeepEqual(a, conformancePoints(50, conformanceSeed+1)) {
		t.Fatal("the seed does not matter")
	}
	if len(a) != 50 {
		t.Fatalf("%d points", len(a))
	}
	for _, p := range a {
		if p.Lat < westPalearctic.south || p.Lat >= westPalearctic.north ||
			p.Lon < westPalearctic.west || p.Lon >= westPalearctic.east {
			t.Errorf("outside the West Palearctic box: %+v", p)
		}
		if p.Elevation < 0 || p.Elevation >= maxSampleElevation || p.Elevation != float64(int(p.Elevation)) {
			t.Errorf("elevation not whole metres in range: %+v", p)
		}
	}
	// A prefix stays the same when more points are drawn, so a larger
	// TEMPUS_OM_POINTS extends a run instead of replacing it.
	if !reflect.DeepEqual(conformancePoints(5, conformanceSeed), a[:5]) {
		t.Error("the first points depend on the count")
	}
}

func TestConformancePointCount(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want int
		ok   bool
	}{
		{"", defaultConformancePoints, true},
		{"3", 3, true},
		{"0", 0, false},
		{"-1", 0, false},
		{"many", 0, false},
	} {
		got, err := conformancePointCount(tc.env)
		if (err == nil) != tc.ok || (tc.ok && got != tc.want) {
			t.Errorf("%q: %d, %v", tc.env, got, err)
		}
	}
}

func TestConformanceWindows(t *testing.T) {
	today := time.Date(2026, 11, 22, 15, 4, 0, 0, time.UTC)
	ws := conformanceWindows(today)
	want := []conformanceWindow{
		{"2005", date(2005, 7, 12), date(2005, 7, 12)},
		{"ifs-switch-2017", date(2016, 12, 31), date(2017, 1, 1)},
		{"last-year", date(2025, 11, 22), date(2025, 11, 22)},
		{"recent", date(2026, 11, 10), date(2026, 11, 19)},
	}
	if !reflect.DeepEqual(ws, want) {
		t.Fatalf("windows\n got %+v\nwant %+v", ws, want)
	}
}
