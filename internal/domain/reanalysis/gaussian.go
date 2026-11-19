package reanalysis

import "math"

// GaussianGrid is ECMWF's octahedral reduced Gaussian grid O<L> as Open-Meteo
// stores it: 2L rows with equally spaced latitudes (dy = 180/(2L+0.5)), row y
// holding 20+4y points in the northern half (mirrored in the southern), rows
// ordered from north to south, longitudes starting at 0°.
type GaussianGrid struct {
	L int
}

// O1280 is the ECMWF IFS HRES 9 km grid.
var O1280 = GaussianGrid{L: 1280}

func (g GaussianGrid) count() int { return 4 * g.L * (g.L + 9) }

// latStep is dy in float32 exactly as the reference computes it.
func (g GaussianGrid) latStep() float32 { return float32(180) / float32(float64(2*g.L)+0.5) }

// rowLength is the number of points in row y.
func (g GaussianGrid) rowLength(y int) int {
	if y < g.L {
		return 20 + 4*y
	}
	return (2*g.L-y-1)*4 + 20
}

// rowStart is the linear index of the first point of row y.
func (g GaussianGrid) rowStart(y int) int {
	if y < g.L {
		return 2*y*y + 18*y
	}
	z := 2*g.L - y
	return g.count() - (2*z*z + 18*z)
}

func (g GaussianGrid) rowLatitude(y int) float32 {
	dy := g.latStep()
	return float32(float32(g.L-y-1)*dy) + dy/2
}

// Coordinates satisfies Grid.
func (g GaussianGrid) Coordinates(i int) (lat, lon float32) {
	var y int
	if i < g.count()/2 {
		y = int((float64(sqrtf(float32(2*float32(i))+81)) - 9) / 2)
	} else {
		y = 2*g.L - 1 - int((float64(sqrtf(float32(2*float32(g.count()-i-1))+81))-9)/2)
	}
	x := i - g.rowStart(y)
	dx := 360 / float32(g.rowLength(y))
	lon = float32(float32(x) * dx)
	if lon >= 180 {
		lon -= 360
	}
	return g.rowLatitude(y), lon
}

// neighbourhood returns the 3×3 points around lat/lon (three rows, three
// points each), their squared degree distances and the position of the nearest.
// Distances use the unwrapped longitude x·dx, and each row's three points are
// rotated so that index 0 is not always the westernmost, as in the reference;
// the first strict minimum wins ties.
func (g GaussianGrid) neighbourhood(lat, lon float32) (pts [9]int, dsq [9]float32, nearest int) {
	dy := g.latStep()
	cy := roundHalfAway(float32(g.L) - 1 - (lat-dy/2)/dy)
	cy = max(1, min(2*g.L-2, cy))
	k := 0
	for j := range 3 {
		y := cy + j - 1
		nx := g.rowLength(y)
		dx := 360 / float32(nx)
		xc := roundHalfAway(lon / dx)
		plat := g.rowLatitude(y)
		start := max(0, 1-xc)
		for i := range 3 {
			x := xc + (i+start)%3 - 1
			pts[k] = g.rowStart(y) + ((x+2*nx)%nx+nx)%nx
			plon := float32(float32(x) * dx)
			dlat, dlon := plat-lat, plon-lon
			dsq[k] = float32(dlat*dlat) + float32(dlon*dlon)
			k++
		}
	}
	best := float32(math.MaxFloat32)
	for i, d := range dsq {
		if d < best {
			best, nearest = d, i
		}
	}
	return pts, dsq, nearest
}

// Select satisfies Grid. Unlike regular grids, a nearest cell within the
// shortcut distance is taken without any correction (its model elevation is
// treated as the target's), and the search starts from an unbounded cost.
func (g GaussianGrid) Select(lat, lon, elevation float32, e Elevations) Cell {
	pts, dsq, n := g.neighbourhood(lat, lon)
	nearestElev := e.At(pts[n])
	if absf(nearestElev-elevation) <= shortcutMetres {
		return g.cell(pts[n], elevation)
	}
	best := float32(math.MaxFloat32)
	bestIdx, bestElev := -1, float32(math.NaN())
	for k, i := range pts {
		h := e.At(i)
		if math.IsNaN(float64(h)) || h <= SeaMarker {
			continue
		}
		km := distanceKm(dsq[k])
		if c := absf(h-elevation) + float32(km*costPerKm); c < best && km < maxSearchKm {
			best, bestIdx, bestElev = c, i, h
		}
	}
	if bestIdx < 0 || best > maxCost {
		return g.cell(pts[n], numericElevation(nearestElev))
	}
	return g.cell(bestIdx, numericElevation(bestElev))
}

func (g GaussianGrid) cell(i int, h float32) Cell {
	return Cell{Index: i, Coord: []uint64{0, fileIndex(i)}, ModelElevation: h}
}
