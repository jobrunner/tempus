package astronomy

import (
	"math"
	"time"
)

// Output property-name keys, shared across the sun and moon features (and their
// tests) so the repeated literals live in one place.
const (
	keyElevationDeg = "elevationDeg"
	keyAzimuthDeg   = "azimuthDeg"
	keyZenithDeg    = "zenithDeg"
	keyDistanceKm   = "distanceKm"
	keyIllumination = "illuminationPct"
	keyPhaseAngle   = "phaseAngleDeg"
	keyAgeDays      = "ageDays"
	keyDawn         = "dawn"
	keyDusk         = "dusk"
)

// fmtTime renders an event time as an RFC3339 UTC string, or nil (JSON null)
// when the event does not occur (polar day/night, or no lunar crossing).
func fmtTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// round2 rounds to two decimal places.
func round2(v float64) float64 { return math.Round(v*100) / 100 }
