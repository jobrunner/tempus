package reanalysis

import (
	"math"
	"testing"
)

func TestRegularGrid_NearestClampsOneStepPastTheEdges(t *testing.T) {
	tests := []struct {
		lat, lon float32
		x, y     int
	}{
		{-90.2, -180.2, 0, 0},                   // −1 in both directions
		{-89.75, -179.75, 1, 1},                 // 1 is a real index, not clamped
		{90.2, 180.2, era5.Nx - 1, era5.Ny - 1}, // Nx+1 and Ny
		{90, 179.75, era5.Nx - 1, era5.Ny - 1},  // the last real indices
		{0, 0, 720, 360},
	}
	for _, tt := range tests {
		if x, y := era5.nearest(tt.lat, tt.lon); x != tt.x || y != tt.y {
			t.Errorf("nearest(%v, %v) = %d/%d, want %d/%d", tt.lat, tt.lon, x, y, tt.x, tt.y)
		}
	}
	if lat, lon := era5.Coordinates(2*era5.Nx + 3); lat != -89.5 || lon != -179.25 {
		t.Fatalf("Coordinates: %v/%v", lat, lon)
	}
}

func TestShortcutBoundaryIsInclusive(t *testing.T) {
	lat, lon := float32(47.5), float32(11.0)
	x, y := era5.nearest(lat, lon)
	center := y*era5.Nx + x
	// Exactly 100 m off: still the centre, with its own height.
	if c := era5.Select(lat, lon, 1100, flat{h: 1000, over: map[int]float32{center + 1: 1100}}); c.Index != center || c.ModelElevation != 1000 {
		t.Fatalf("regular: %+v", c)
	}
	g := O1280
	pts, _, n := g.neighbourhood(lat, lon)
	if c := g.Select(lat, lon, 1100, flat{h: 1000, over: map[int]float32{pts[(n+1)%9]: 1100}}); c.Index != pts[n] || c.ModelElevation != 1100 {
		t.Fatalf("gaussian: %+v", c)
	}
	snow := []float32{1}
	SnowRule(snow, []float32{1}, 1000, 1100)
	if snow[0] != 1 {
		t.Fatal("snow rule: 100 m is within the shortcut")
	}
}

func TestGaussianGrid_RowLayout(t *testing.T) {
	g := O1280
	// ECMWF's O1280 has 6 599 680 points.
	if g.count() != 6599680 {
		t.Fatalf("count %d", g.count())
	}
	sum := 0
	for y := range 2 * g.L {
		if g.rowStart(y) != sum {
			t.Fatalf("row %d starts at %d, want %d", y, g.rowStart(y), sum)
		}
		sum += g.rowLength(y)
	}
	if sum != g.count() {
		t.Fatalf("rows hold %d points, want %d", sum, g.count())
	}
	if g.rowLength(0) != 20 || g.rowLength(2*g.L-1) != 20 || g.rowLength(g.L-1) != g.rowLength(g.L) {
		t.Fatal("rows must be symmetric with 20 points at the poles")
	}
	// dy = 180/(2L + 0.5); row 0 lies at (L − 0.5)·dy.
	dy := 180 / (2*float64(g.L) + 0.5)
	if lat, _ := g.Coordinates(0); math.Abs(float64(lat)-(float64(g.L)-0.5)*dy) > 1e-4 {
		t.Fatalf("first row latitude %v", lat)
	}
	if lat, _ := g.Coordinates(g.count() - 1); math.Abs(float64(lat)+(float64(g.L)-0.5)*dy) > 1e-4 {
		t.Fatalf("last row latitude %v", lat)
	}
}

func TestGaussianGrid_NeighbourhoodRoundTrip(t *testing.T) {
	g := O1280
	// Every point must be its own nearest neighbour, including the southern
	// half, the rows next to the poles and both sides of the date line.
	var idx []int
	for i := 0; i < g.count(); i += 104729 {
		idx = append(idx, i)
	}
	for _, y := range []int{1, 2, g.L - 1, g.L, 2*g.L - 3, 2*g.L - 2} {
		s, n := g.rowStart(y), g.rowLength(y)
		idx = append(idx, s, s+1, s+n/2-1, s+n/2, s+n/2+1, s+n-1)
	}
	for _, i := range idx {
		lat, lon := g.Coordinates(i)
		pts, dsq, n := g.neighbourhood(lat, lon)
		if pts[n] != i || dsq[n] > 1e-9 {
			t.Errorf("point %d at %v/%v: nearest is %d (dsq %v)", i, lat, lon, pts[n], dsq[n])
		}
	}
	// Longitudes run from 0° and wrap to negative values from 180° on.
	s, n := g.rowStart(g.L), g.rowLength(g.L)
	if _, lon := g.Coordinates(s + n/2); lon != -180 {
		t.Fatalf("half a row from 0° is −180°: %v", lon)
	}
	if _, lon := g.Coordinates(s + n/2 - 1); lon <= 179 || lon >= 180 {
		t.Fatalf("just before the date line: %v", lon)
	}
}
