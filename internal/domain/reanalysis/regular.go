package reanalysis

import "math"

// RegularGrid is a latitude/longitude grid with equal spacing, row-major from
// the south-west corner (ERA5, ERA5-Land).
type RegularGrid struct {
	Nx, Ny         int
	LatMin, LonMin float32
	D              float32
}

// Coordinates satisfies Grid.
func (g RegularGrid) Coordinates(i int) (lat, lon float32) {
	y, x := i/g.Nx, i%g.Nx
	return g.LatMin + float32(float32(y)*g.D), g.LonMin + float32(float32(x)*g.D)
}

// nearest returns the grid position closest to lat/lon. Indices one past
// either end are clamped onto the grid (the reference's edge handling).
func (g RegularGrid) nearest(lat, lon float32) (x, y int) {
	x = roundHalfAway((lon - g.LonMin) / g.D)
	y = roundHalfAway((lat - g.LatMin) / g.D)
	switch x {
	case -1:
		x = 0
	case g.Nx, g.Nx + 1:
		x = g.Nx - 1
	}
	switch y {
	case -1:
		y = 0
	case g.Ny:
		y = g.Ny - 1
	}
	return x, y
}

// Select satisfies Grid. The 3×3 neighbourhood is clamped at the grid edges
// and its middle element serves as the "centre" (not the true nearest point at
// an edge). Regular grids always correct to the target height, even when the
// centre is within the shortcut distance.
func (g RegularGrid) Select(lat, lon, elevation float32, e Elevations) Cell {
	x, y := g.nearest(lat, lon)
	center := y*g.Nx + x
	idx := g.window(x, y)
	centerElev := e.At(idx[len(idx)/2])
	best := absf(centerElev - elevation)
	if best <= shortcutMetres {
		return g.cell(center, centerElev)
	}
	bestIdx, bestElev := -1, float32(math.NaN())
	for _, i := range idx {
		h := e.At(i)
		if math.IsNaN(float64(h)) || h <= SeaMarker {
			continue
		}
		clat, clon := g.Coordinates(i)
		dlat, dlon := clat-lat, clon-lon
		km := distanceKm(float32(dlat*dlat) + float32(dlon*dlon))
		if c := cost(h, elevation, km); c < best && km < maxSearchKm {
			best, bestIdx, bestElev = c, i, h
		}
	}
	if bestIdx < 0 || best > maxCost {
		return g.cell(center, centerElev)
	}
	return g.cell(bestIdx, bestElev)
}

// window returns the up to 3×3 grid indices around x/y, clamped at the edges.
// A fixed array, not an append loop: the window can never grow past nine.
func (g RegularGrid) window(x, y int) []int {
	var buf [9]int
	n := 0
	for yy := max(0, y-1); yy < min(g.Ny, y+2); yy++ {
		for xx := max(0, x-1); xx < min(g.Nx, x+2); xx++ {
			buf[n] = yy*g.Nx + xx
			n++
		}
	}
	return buf[:n]
}

func (g RegularGrid) cell(i int, h float32) Cell {
	return Cell{Index: i, Coord: []uint64{fileIndex(i / g.Nx), fileIndex(i % g.Nx)}, ModelElevation: numericElevation(h)}
}
