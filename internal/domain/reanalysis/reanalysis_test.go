package reanalysis

import (
	"math"
	"testing"
)

var nan = float32(math.NaN())

func TestCorrect(t *testing.T) {
	s := []float32{10, nan}
	Correct(s, 1500, 1000)
	if want := float32(10) + float32(500*LapseRate); s[0] != want {
		t.Fatalf("got %v want %v", s[0], want)
	}
	if !isNaN(s[1]) {
		t.Fatal("NaN must stay NaN")
	}
	s = []float32{3}
	Correct(s, 700, 700)
	if s[0] != 3 {
		t.Fatal("equal elevations must not change values")
	}
}

func TestMix_PriorityAndEarlyStop(t *testing.T) {
	high := []float32{1, nan, 3}
	low := []float32{10, 20, 30}
	got := Mix([][]float32{high, nil, low}, false)
	if got[0] != 1 || got[1] != 20 || got[2] != 3 {
		t.Fatalf("got %v", got)
	}
	if !isNaN(high[1]) {
		t.Fatal("inputs must not be modified")
	}
	full := Mix([][]float32{{1, 2}, {nan, nan}}, true)
	if full[0] != 1 || full[1] != 2 {
		t.Fatalf("a complete high-priority series must win: %v", full)
	}
}

func TestMix_SmoothsTheHoursBeforeAGap(t *testing.T) {
	high := []float32{0, 0, 0, 0, 0, nan, 0}
	low := []float32{8, 8, 8, 8, 8, 8, 8}
	got := Mix([][]float32{high, low}, true)
	// Walking back from the filled gap at index 5: since = 1, 2, 3 at
	// indices 4, 3, 2 → weights of the lower series 3/4, 2/4, 1/4.
	want := []float32{0, 0, 2, 4, 6, 8, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %v want %v (all %v)", i, got[i], want[i], got)
		}
	}
	plain := Mix([][]float32{high, low}, false)
	if plain[4] != 0 || plain[5] != 8 {
		t.Fatalf("categorical variables are not smoothed: %v", plain)
	}
}

func TestSnowRule(t *testing.T) {
	snow := []float32{1, 1}
	SnowRule(snow, []float32{-1, 0.5}, 1000, 1050)
	if snow[1] != 1 {
		t.Fatal("within 100 m the rule must not fire")
	}
	SnowRule(snow, []float32{-1, 0}, 1000, 1200)
	if snow[0] != 1 || snow[1] != 0 {
		t.Fatalf("got %v", snow)
	}
}

func TestRelativeHumidity(t *testing.T) {
	if got := RelativeHumidity(20, 20); got != 100 {
		t.Fatalf("saturated: %v", got)
	}
	if got := RelativeHumidity(20, 10); math.Abs(float64(got)-52.6) > 0.2 {
		t.Fatalf("20/10 °C: %v", got)
	}
	if got := RelativeHumidity(10, 12); got != 100 {
		t.Fatalf("clamped: %v", got)
	}
}

func TestWeatherCode(t *testing.T) {
	tests := []struct {
		cloud, precip, snow float32
		want                float32
	}{
		{10, 0, 0, 0}, {20, 0, 0, 1}, {50, 0, 0, 2}, {80, 0, 0, 3},
		{0, 0.01, 0, 51}, {0, 0.5, 0, 53}, {0, 1.0, 0, 55}, {0, 1.3, 0, 61},
		{0, 2.5, 0, 63}, {0, 7.6, 0, 65}, {0, 9, 0.01, 71}, {0, 0, 0.2, 73}, {0, 0, 0.8, 75},
	}
	for _, tt := range tests {
		if got := WeatherCode(tt.cloud, tt.precip, tt.snow); got != tt.want {
			t.Errorf("WeatherCode(%v, %v, %v) = %v, want %v", tt.cloud, tt.precip, tt.snow, got, tt.want)
		}
	}
	// 1.3 mm compares in float32: the float32 nearest to 1.3 is a 61.
	if got := WeatherCode(0, float32(1.3), 0); got != 61 {
		t.Errorf("float32 1.3: %v", got)
	}
	if !isNaN(WeatherCode(nan, 0, 0)) || !isNaN(WeatherCode(-1, 0, 0)) {
		t.Error("missing or negative cloud cover must give NaN")
	}
}

func TestDerivedUnits(t *testing.T) {
	if got := WindSpeedKmh(3, 4); got != 18 {
		t.Fatalf("wind: %v", got)
	}
	if got := SnowfallCm(1); got != float32(0.7) {
		t.Fatalf("snow: %v", got)
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		x    float32
		d    int
		want float32
	}{
		{0.05, 1, 0.1}, {-0.25, 1, -0.3}, {12.34, 1, 12.3}, {99.5, 0, 100}, {0.125, 2, 0.13},
	}
	for _, tt := range tests {
		if got := Round(tt.x, tt.d); got != tt.want {
			t.Errorf("Round(%v, %d) = %v, want %v", tt.x, tt.d, got, tt.want)
		}
	}
}

func TestDailyMinMax(t *testing.T) {
	series := make([]float32, 26)
	for i := range series {
		series[i] = 5
	}
	lo, hi := DailyMinMax(series, 1)
	if lo != 5 || hi != 5 {
		t.Fatalf("constant: %v %v", lo, hi)
	}
	// A single peak: the Hermite curve through …, 0, 10, 0, … peaks at the
	// sample, but its neighbours undershoot below zero.
	series[12] = 0
	series[13] = 10
	for i := range series {
		if i != 13 {
			series[i] = 0
		}
	}
	lo, hi = DailyMinMax(series, 1)
	if hi != 10 || lo >= 0 {
		t.Fatalf("peak: lo %v hi %v", lo, hi)
	}
	empty := []float32{nan, nan, nan}
	if lo, _ := DailyMinMax(empty, 0); !isNaN(lo) {
		t.Fatal("no data must give NaN")
	}
}

func TestDailyMeanAndSum(t *testing.T) {
	series := make([]float32, 24)
	for i := range series {
		series[i] = float32(i)
	}
	if got := DailySum(series, 0); got != 276 {
		t.Fatalf("sum %v", got)
	}
	if got := DailyMean(series, 0); got != 11.5 {
		t.Fatalf("mean %v", got)
	}
}

// flat returns the same height everywhere except where overridden.
type flat struct {
	h    float32
	over map[int]float32
}

func (f flat) At(i int) float32 {
	if v, ok := f.over[i]; ok {
		return v
	}
	return f.h
}

var era5 = RegularGrid{Nx: 1440, Ny: 721, LatMin: -90, LonMin: -180, D: 0.25}

func TestRegularGrid_Select(t *testing.T) {
	lat, lon := float32(47.5), float32(11.0)
	x, y := era5.nearest(lat, lon)
	center := y*era5.Nx + x
	if c := era5.Select(lat, lon, 1000, flat{h: 950}); c.Index != center || c.ModelElevation != 950 {
		t.Fatalf("shortcut: %+v", c)
	}
	// Centre 1000 m too low; the northern neighbour (27.75 km → 832 m of cost)
	// still fits better. Regular grids start the search at the centre's cost.
	north := center + era5.Nx
	c := era5.Select(lat, lon, 2000, flat{h: 1000, over: map[int]float32{north: 1990}})
	if c.Index != north || c.ModelElevation != 1990 || c.Coord[0] != fileIndex(y+1) {
		t.Fatalf("search: %+v", c)
	}
	// Everything is sea: fall back to the centre, sea counts as 0 m.
	if c := era5.Select(lat, lon, 1600, flat{h: SeaMarker}); c.Index != center || c.ModelElevation != 0 {
		t.Fatalf("sea fallback: %+v", c)
	}
	if x, y := era5.nearest(90, 180); x != era5.Nx-1 || y != era5.Ny-1 {
		t.Fatalf("edge clamp: %d %d", x, y)
	}
	if x, _ := era5.nearest(0, -180.1); x != 0 {
		t.Fatalf("west clamp: %d", x)
	}
}

func TestGaussianGrid_RowsAndCoordinates(t *testing.T) {
	g := O1280
	if g.rowStart(2*g.L) != g.count() {
		t.Fatalf("rows must cover all %d points", g.count())
	}
	for _, i := range []int{0, 19, 20, 1234567, g.count()/2 - 1, g.count() / 2, g.count() - 1} {
		lat, lon := g.Coordinates(i)
		pts, _, n := g.neighbourhood(lat, lon)
		if pts[n] != i {
			t.Errorf("point %d at %v/%v: nearest is %d", i, lat, lon, pts[n])
		}
	}
	lat, _ := g.Coordinates(0)
	if lat < 89.9 || lat > 90 {
		t.Fatalf("first row latitude %v", lat)
	}
}

func TestGaussianGrid_Select(t *testing.T) {
	g := O1280
	lat, lon := float32(47.5), float32(11.0)
	pts, _, n := g.neighbourhood(lat, lon)
	if c := g.Select(lat, lon, 1000, flat{h: 950}); c.Index != pts[n] || c.ModelElevation != 1000 {
		t.Fatalf("shortcut takes the target height (no correction): %+v", c)
	}
	other := pts[(n+1)%9]
	c := g.Select(lat, lon, 1600, flat{h: 1000, over: map[int]float32{other: 1590}})
	if c.Index != other || c.ModelElevation != 1590 || c.Coord[1] != fileIndex(other) {
		t.Fatalf("search: %+v", c)
	}
	if c := g.Select(lat, lon, 1600, flat{h: SeaMarker}); c.Index != pts[n] || c.ModelElevation != 0 {
		t.Fatalf("sea fallback: %+v", c)
	}
}
