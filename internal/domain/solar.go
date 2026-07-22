package domain

import (
	"math"
	"time"
)

// SolarElevationDeg returns the sun's elevation angle (degrees above the
// horizon) for a WGS84 coordinate at the given instant, via the NOAA
// approximation. lonDeg is east-positive.
func SolarElevationDeg(latDeg, lonDeg float64, t time.Time) float64 {
	t = t.UTC()
	doy := float64(t.YearDay())
	frac := float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600 // UTC hours
	gamma := 2 * math.Pi / 365.0 * (doy - 1 + (frac-12)/24)
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(gamma) - 0.032077*math.Sin(gamma) -
		0.014615*math.Cos(2*gamma) - 0.040849*math.Sin(2*gamma))
	decl := 0.006918 - 0.399912*math.Cos(gamma) + 0.070257*math.Sin(gamma) -
		0.006758*math.Cos(2*gamma) + 0.000907*math.Sin(2*gamma) -
		0.002697*math.Cos(3*gamma) + 0.00148*math.Sin(3*gamma)
	tst := frac*60 + eqTime + 4*lonDeg // true solar time, minutes
	ha := (tst/4 - 180) * math.Pi / 180
	latRad := latDeg * math.Pi / 180
	cosZ := math.Sin(latRad)*math.Sin(decl) + math.Cos(latRad)*math.Cos(decl)*math.Cos(ha)
	cosZ = math.Max(-1, math.Min(1, cosZ))
	return 90 - math.Acos(cosZ)*180/math.Pi
}

// IsDaylight reports whether the sun is above the horizon (standard
// sunrise/sunset threshold −0.833°, accounting for refraction and the sun's
// radius) at the coordinate and instant.
func IsDaylight(latDeg, lonDeg float64, t time.Time) bool {
	return SolarElevationDeg(latDeg, lonDeg, t) > -0.833
}

// SolarPositionSource cites the method for the computed day/night value.
const SolarPositionSource = "Tag/Nacht aus Sonnenstand berechnet (NOAA-Näherung), tempus"
