package domain

import (
	"math"
	"time"
)

// Standard altitude thresholds (degrees) for solar events. Sunrise/sunset use
// −0.833° (atmospheric refraction + the sun's apparent radius); the three
// twilight phases are the conventional −6/−12/−18° depression angles.
const (
	altSunrise    = -0.833
	altCivil      = -6.0
	altNautical   = -12.0
	altAstronomic = -18.0
)

// SolarPositionSource cites the method for the computed day/night value.
const SolarPositionSource = "Tag/Nacht aus Sonnenstand berechnet (NOAA-Näherung), tempus"

// SunSource is the attribution for the sun feature.
const SunSource = "Sonnenstand berechnet (NOAA-Näherung), tempus"

// solarGeom returns the sun's declination (radians) and the equation of time
// (minutes) for the instant, via the NOAA/Spencer Fourier approximation.
func solarGeom(t time.Time) (declRad, eqTimeMin float64) {
	t = t.UTC()
	doy := float64(t.YearDay())
	frac := hoursUTC(t)
	gamma := 2 * math.Pi / 365.0 * (doy - 1 + (frac-12)/24)
	eqTimeMin = 229.18 * (0.000075 + 0.001868*math.Cos(gamma) - 0.032077*math.Sin(gamma) -
		0.014615*math.Cos(2*gamma) - 0.040849*math.Sin(2*gamma))
	declRad = 0.006918 - 0.399912*math.Cos(gamma) + 0.070257*math.Sin(gamma) -
		0.006758*math.Cos(2*gamma) + 0.000907*math.Sin(2*gamma) -
		0.002697*math.Cos(3*gamma) + 0.00148*math.Sin(3*gamma)
	return declRad, eqTimeMin
}

// hoursUTC returns the fractional UTC hour of t.
func hoursUTC(t time.Time) float64 {
	t = t.UTC()
	return float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600
}

// SolarElevationDeg returns the sun's elevation angle (degrees above the
// horizon) for a WGS84 coordinate at the given instant, via the NOAA
// approximation. lonDeg is east-positive.
func SolarElevationDeg(latDeg, lonDeg float64, t time.Time) float64 {
	elev, _ := SunAltAz(latDeg, lonDeg, t)
	return elev
}

// SunAltAz returns the sun's elevation (degrees above the horizon) and azimuth
// (degrees clockwise from true north) for a WGS84 coordinate at the instant.
func SunAltAz(latDeg, lonDeg float64, t time.Time) (elevDeg, azDeg float64) {
	decl, eqTime := solarGeom(t)
	frac := hoursUTC(t)
	tst := frac*60 + eqTime + 4*lonDeg // true solar time, minutes
	ha := (tst/4 - 180) * math.Pi / 180
	latRad := latDeg * math.Pi / 180

	cosZ := math.Sin(latRad)*math.Sin(decl) + math.Cos(latRad)*math.Cos(decl)*math.Cos(ha)
	cosZ = clampUnit(cosZ)
	zenith := math.Acos(cosZ)
	elevDeg = 90 - zenith*180/math.Pi

	// NOAA azimuth (clockwise from north).
	denom := math.Cos(latRad) * math.Sin(zenith)
	if denom == 0 {
		return elevDeg, 0
	}
	cosAz := clampUnit((math.Sin(latRad)*cosZ - math.Sin(decl)) / denom)
	az := math.Acos(cosAz) * 180 / math.Pi
	if ha > 0 {
		az = math.Mod(az+180, 360)
	} else {
		az = math.Mod(540-az, 360)
	}
	return elevDeg, az
}

// IsDaylight reports whether the sun is above the horizon (standard
// sunrise/sunset threshold −0.833°, accounting for refraction and the sun's
// radius) at the coordinate and instant.
func IsDaylight(latDeg, lonDeg float64, t time.Time) bool {
	return SolarElevationDeg(latDeg, lonDeg, t) > altSunrise
}

// SunTimes holds the computed solar event times (UTC) for the calendar day of
// the reference instant. Any field is nil when the event does not occur that
// day (polar day/night at the given altitude threshold).
type SunTimes struct {
	Sunrise               *time.Time
	Sunset                *time.Time
	SolarNoon             *time.Time
	SolarNoonElevationDeg float64
	DayLengthMinutes      float64
	CivilDawn             *time.Time
	CivilDusk             *time.Time
	NauticalDawn          *time.Time
	NauticalDusk          *time.Time
	AstronomicalDawn      *time.Time
	AstronomicalDusk      *time.Time
}

// SolarEvents computes sunrise, sunset, solar noon and the three twilight
// phases for the UTC calendar day of t, at the given coordinate.
func SolarEvents(latDeg, lonDeg float64, t time.Time) SunTimes {
	day := t.UTC().Truncate(24 * time.Hour)
	// Use declination/eqTime at local solar noon for the day.
	noonProbe := day.Add(12 * time.Hour)
	decl, eqTime := solarGeom(noonProbe)

	noonMin := 720 - 4*lonDeg - eqTime
	noon := day.Add(time.Duration(noonMin * float64(time.Minute)))
	noonElev, _ := SunAltAz(latDeg, lonDeg, noon)

	st := SunTimes{
		SolarNoon:             &noon,
		SolarNoonElevationDeg: round2(noonElev),
	}

	rise, set := sunEvent(latDeg, day, decl, noonMin, altSunrise)
	st.Sunrise, st.Sunset = rise, set
	if rise != nil && set != nil {
		st.DayLengthMinutes = round2(set.Sub(*rise).Minutes())
	}
	st.CivilDawn, st.CivilDusk = sunEvent(latDeg, day, decl, noonMin, altCivil)
	st.NauticalDawn, st.NauticalDusk = sunEvent(latDeg, day, decl, noonMin, altNautical)
	st.AstronomicalDawn, st.AstronomicalDusk = sunEvent(latDeg, day, decl, noonMin, altAstronomic)
	return st
}

// sunEvent returns the rising and setting instants (UTC) at which the sun
// crosses altDeg on the given day, or nil pointers when the sun stays entirely
// above or below that altitude (polar day/night).
func sunEvent(latDeg float64, day time.Time, declRad, noonMin, altDeg float64) (rise, set *time.Time) {
	latRad := latDeg * math.Pi / 180
	h0 := altDeg * math.Pi / 180
	cosH := (math.Sin(h0) - math.Sin(latRad)*math.Sin(declRad)) /
		(math.Cos(latRad) * math.Cos(declRad))
	if cosH > 1 || cosH < -1 {
		return nil, nil // no crossing: sun stays below (polar night) or above (polar day)
	}
	hDeg := math.Acos(cosH) * 180 / math.Pi
	r := day.Add(time.Duration((noonMin - 4*hDeg) * float64(time.Minute)))
	s := day.Add(time.Duration((noonMin + 4*hDeg) * float64(time.Minute)))
	return &r, &s
}

// SunLightPhase returns the bilingual light-regime label for a solar elevation:
// day, then civil / nautical / astronomical twilight, then night.
func SunLightPhase(elevDeg float64) (de, en string) {
	switch {
	case elevDeg > altSunrise:
		return "Tag", "day"
	case elevDeg > altCivil:
		return "bürgerliche Dämmerung", "civil twilight"
	case elevDeg > altNautical:
		return "nautische Dämmerung", "nautical twilight"
	case elevDeg > altAstronomic:
		return "astronomische Dämmerung", "astronomical twilight"
	default:
		return "Nacht", "night"
	}
}

// clampUnit clamps v to [-1, 1] (guards acos/asin against float rounding).
func clampUnit(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
