// Package reanalysis turns raw model series into point values the way the
// Open-Meteo archive does for its best_match: cell selection on each model grid
// with terrain awareness, lapse-rate correction to the target elevation,
// priority mixing of ERA5, ERA5-Land and ECMWF IFS, and the derived variables.
//
// The rules are implemented from the behavioural description in the project's
// plan (verified against the live Open-Meteo API as an oracle), not from
// Open-Meteo source code. Arithmetic is float32 throughout, as in the
// reference; products that feed a sum are converted explicitly so the compiler
// cannot fuse them into FMA instructions, which would change the last bit.
package reanalysis

import "math"

// Elevations gives the model surface height of grid point i in metres; values
// at or below SeaMarker mark sea points.
type Elevations interface {
	At(i int) float32
}

const (
	// SeaMarker marks sea points in model elevation data.
	SeaMarker = -999
	// shortcutMetres: a nearest cell this close to the target height is taken.
	shortcutMetres = 100
	// costPerKm weighs horizontal distance against height difference (m per km).
	costPerKm = 30
	// maxSearchKm bounds the neighbour search radius.
	maxSearchKm = 50
	// maxCost: a best neighbour this expensive falls back to the nearest cell.
	maxCost     = 1500
	kmPerDegree = 111
	// noHeight marks grid points whose height should be ignored in the cost.
	noHeight = 9999
)

// Cell is the grid point selected for a target location.
type Cell struct {
	// Index is the linear grid index; Coord the leading file coordinate.
	Index int
	Coord []uint64
	// ModelElevation is the height the series refers to after selection: the
	// cell's surface height (0 for sea) or, when the reference skips the
	// correction, the target height itself.
	ModelElevation float32
}

// Grid selects cells on one model grid.
type Grid interface {
	// Select picks the cell for lat/lon at the target elevation (metres).
	Select(lat, lon, elevation float32, e Elevations) Cell
	// Coordinates returns the latitude and longitude of grid point i.
	Coordinates(i int) (lat, lon float32)
}

// numericElevation maps the sea marker to 0 m, which is what the correction uses.
func numericElevation(h float32) float32 {
	if h <= SeaMarker {
		return 0
	}
	return h
}

// fileIndex converts a grid index into a file coordinate.
func fileIndex(i int) uint64 {
	if i < 0 {
		return 0
	}
	return uint64(i)
}

func absf(x float32) float32 { return float32(math.Abs(float64(x))) }

func sqrtf(x float32) float32 { return float32(math.Sqrt(float64(x))) }

// roundHalfAway rounds like C roundf: halves away from zero.
func roundHalfAway(x float32) int {
	return int(math.Copysign(math.Floor(math.Abs(float64(x))+0.5), float64(x)))
}

// distanceKm is the reference's flat-earth distance: degrees unscaled by
// latitude, times 111 km.
func distanceKm(dsq float32) float32 { return float32(sqrtf(dsq) * kmPerDegree) }

// cost of a neighbour with height h for a target at elevation.
func cost(h, elevation, km float32) float32 {
	var dh float32
	if h < noHeight {
		dh = absf(h - elevation)
	}
	return dh + float32(km*costPerKm)
}
