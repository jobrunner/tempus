package domain

import (
	"math"
	"time"
)

// MoonSource is the attribution for the moon feature.
const MoonSource = "Mondstand berechnet nach Meeus (geozentrische Näherung), tempus"

// moonStandardAltDeg is the geocentric altitude at which the moon is considered
// to rise or set: Meeus's standard value combining mean horizontal parallax,
// atmospheric refraction and the lunar semidiameter.
const moonStandardAltDeg = 0.125

const synodicMonthDays = 29.530588853

// Phase names that recur across the mapper and its tests.
const (
	phaseNewMoonDE  = "Neumond"
	phaseFullMoonDE = "Vollmond"
)

// lunarTerm is one row of Meeus table 47.A (longitude Σl and distance Σr).
type lunarTerm struct {
	d, m, mp, f int
	sl, sr      float64
}

// latTerm is one row of Meeus table 47.B (latitude Σb).
type latTerm struct {
	d, m, mp, f int
	sb          float64
}

// table47A are the periodic terms for lunar longitude (1e-6 deg) and distance
// (1e-3 km); table47B for latitude (1e-6 deg). From Meeus, Astronomical
// Algorithms, 2nd ed., ch. 47.
var table47A = []lunarTerm{
	{0, 0, 1, 0, 6288774, -20905355}, {2, 0, -1, 0, 1274027, -3699111},
	{2, 0, 0, 0, 658314, -2955968}, {0, 0, 2, 0, 213618, -569925},
	{0, 1, 0, 0, -185116, 48888}, {0, 0, 0, 2, -114332, -3149},
	{2, 0, -2, 0, 58793, 246158}, {2, -1, -1, 0, 57066, -152138},
	{2, 0, 1, 0, 53322, -170733}, {2, -1, 0, 0, 45758, -204586},
	{0, 1, -1, 0, -40923, -129620}, {1, 0, 0, 0, -34720, 108743},
	{0, 1, 1, 0, -30383, 104755}, {2, 0, 0, -2, 15327, 10321},
	{0, 0, 1, 2, -12528, 0}, {0, 0, 1, -2, 10980, 79661},
	{4, 0, -1, 0, 10675, -34782}, {0, 0, 3, 0, 10034, -23210},
	{4, 0, -2, 0, 8548, -21636}, {2, 1, -1, 0, -7888, 24208},
	{2, 1, 0, 0, -6766, 30824}, {1, 0, -1, 0, -5163, -8379},
	{1, 1, 0, 0, 4987, -16675}, {2, -1, 1, 0, 4036, -12831},
	{2, 0, 2, 0, 3994, -10445}, {4, 0, 0, 0, 3861, -11650},
	{2, 0, -3, 0, 3665, 14403}, {0, 1, -2, 0, -2689, -7003},
	{2, 0, -1, 2, -2602, 0}, {2, -1, -2, 0, 2390, 10056},
	{1, 0, 1, 0, -2348, 6322}, {2, -2, 0, 0, 2236, -9884},
	{0, 1, 2, 0, -2120, 5751}, {0, 2, 0, 0, -2069, 0},
	{2, -2, -1, 0, 2048, -4950}, {2, 0, 1, -2, -1773, 4130},
	{2, 0, 0, 2, -1595, 0}, {4, -1, -1, 0, 1215, -3958},
	{0, 0, 2, 2, -1110, 0}, {3, 0, -1, 0, -892, 3258},
	{2, 1, 1, 0, -810, 2616}, {4, -1, -2, 0, 759, -1897},
	{0, 2, -1, 0, -713, -2117}, {2, 2, -1, 0, -700, 2354},
	{2, 1, -2, 0, 691, 0}, {2, -1, 0, -2, 596, 0},
	{4, 0, 1, 0, 549, -1423}, {0, 0, 4, 0, 537, -1117},
	{4, -1, 0, 0, 520, -1571}, {1, 0, -2, 0, -487, -1739},
	{2, 1, 0, -2, -399, 0}, {0, 0, 2, -2, -381, -4421},
	{1, 1, 1, 0, 351, 0}, {3, 0, -2, 0, -340, 0},
	{4, 0, -3, 0, 330, 0}, {2, -1, 2, 0, 327, 0},
	{0, 2, 1, 0, -323, 1165}, {1, 1, -1, 0, 299, 0},
	{2, 0, 3, 0, 294, 0}, {2, 0, -1, -2, 0, 8752},
}

var table47B = []latTerm{
	{0, 0, 0, 1, 5128122}, {0, 0, 1, 1, 280602}, {0, 0, 1, -1, 277693},
	{2, 0, 0, -1, 173237}, {2, 0, -1, 1, 55413}, {2, 0, -1, -1, 46271},
	{2, 0, 0, 1, 32573}, {0, 0, 2, 1, 17198}, {2, 0, 1, -1, 9266},
	{0, 0, 2, -1, 8822}, {2, -1, 0, -1, 8216}, {2, 0, -2, -1, 4324},
	{2, 0, 1, 1, 4200}, {2, 1, 0, -1, -3359}, {2, -1, -1, 1, 2463},
	{2, -1, 0, 1, 2211}, {2, -1, -1, -1, 2065}, {0, 1, -1, -1, -1870},
	{4, 0, -1, -1, 1828}, {0, 1, 0, 1, -1794}, {0, 0, 0, 3, -1749},
	{0, 1, -1, 1, -1565}, {1, 0, 0, 1, -1491}, {0, 1, 1, 1, -1475},
	{0, 1, 1, -1, -1410}, {0, 1, 0, -1, -1344}, {1, 0, 0, -1, -1335},
	{0, 0, 3, 1, 1107}, {4, 0, 0, -1, 1021}, {4, 0, -1, 1, 833},
	{0, 0, 1, -3, 777}, {4, 0, -2, 1, 671}, {2, 0, 0, -3, 607},
	{2, 0, 2, -1, 596}, {2, -1, 1, -1, 491}, {2, 0, -2, 1, -451},
	{0, 0, 3, -1, 439}, {2, 0, 2, 1, 422}, {2, 0, -3, -1, 421},
	{2, 1, -1, 1, -366}, {2, 1, 0, 1, -351}, {4, 0, 0, 1, 331},
	{2, -1, 1, 1, 315}, {2, -2, 0, -1, 302}, {0, 0, 1, 3, -283},
	{2, 1, 1, -1, -229}, {1, 1, 0, -1, 223}, {1, 1, 0, 1, 223},
	{0, 1, -2, -1, -220}, {2, 1, -1, -1, -220}, {1, 0, 1, 1, -185},
	{2, -1, -2, -1, 181}, {0, 1, 2, 1, -177}, {4, 0, -2, -1, 176},
	{4, -1, -1, -1, 166}, {1, 0, 1, -1, -164}, {4, 0, 1, -1, 132},
	{1, 0, -1, -1, -119}, {4, -1, 0, -1, 115}, {2, -2, 0, 1, 107},
}

// julianDay returns the Julian Day (UT) for the instant. ΔT (TD−UT) is ignored;
// at the arcminute precision of this model it is negligible.
func julianDay(t time.Time) float64 {
	return float64(t.UTC().UnixNano())/8.64e13 + 2440587.5
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }

// eFactor scales a term by Meeus's E (or E²) when it contains the sun's mean
// anomaly, correcting for the eccentricity of the earth's orbit.
func eFactor(e float64, m int) float64 {
	switch m {
	case 1, -1:
		return e
	case 2, -2:
		return e * e
	default:
		return 1
	}
}

// moonEcliptic returns the moon's geocentric ecliptic longitude λ and latitude
// β (degrees) and distance Δ (km) via Meeus ch. 47.
func moonEcliptic(t time.Time) (lambda, beta, distKm float64) {
	jde := julianDay(t)
	tc := (jde - 2451545.0) / 36525.0

	lp := 218.3164477 + 481267.88123421*tc - 0.0015786*tc*tc + tc*tc*tc/538841 - tc*tc*tc*tc/65194000
	d := 297.8501921 + 445267.1114034*tc - 0.0018819*tc*tc + tc*tc*tc/545868 - tc*tc*tc*tc/113065000
	m := 357.5291092 + 35999.0502909*tc - 0.0001536*tc*tc + tc*tc*tc/24490000
	mp := 134.9633964 + 477198.8675055*tc + 0.0087414*tc*tc + tc*tc*tc/69699 - tc*tc*tc*tc/14712000
	f := 93.2720950 + 483202.0175233*tc - 0.0036539*tc*tc - tc*tc*tc/3526000 + tc*tc*tc*tc/863310000
	a1 := 119.75 + 131.849*tc
	a2 := 53.09 + 479264.290*tc
	a3 := 313.45 + 481266.484*tc
	e := 1 - 0.002516*tc - 0.0000074*tc*tc

	var sumL, sumR float64
	for _, tm := range table47A {
		arg := deg2rad(float64(tm.d)*d + float64(tm.m)*m + float64(tm.mp)*mp + float64(tm.f)*f)
		ef := eFactor(e, tm.m)
		sumL += tm.sl * ef * math.Sin(arg)
		sumR += tm.sr * ef * math.Cos(arg)
	}
	var sumB float64
	for _, tm := range table47B {
		arg := deg2rad(float64(tm.d)*d + float64(tm.m)*m + float64(tm.mp)*mp + float64(tm.f)*f)
		sumB += tm.sb * eFactor(e, tm.m) * math.Sin(arg)
	}

	sumL += 3958*math.Sin(deg2rad(a1)) + 1962*math.Sin(deg2rad(lp-f)) + 318*math.Sin(deg2rad(a2))
	sumB += -2235*math.Sin(deg2rad(lp)) + 382*math.Sin(deg2rad(a3)) +
		175*math.Sin(deg2rad(a1-f)) + 175*math.Sin(deg2rad(a1+f)) +
		127*math.Sin(deg2rad(lp-mp)) - 115*math.Sin(deg2rad(lp+mp))

	lambda = math.Mod(lp+sumL/1_000_000, 360)
	if lambda < 0 {
		lambda += 360
	}
	beta = sumB / 1_000_000
	distKm = 385000.56 + sumR/1000
	return lambda, beta, distKm
}

// obliquityRad returns the mean obliquity of the ecliptic (radians).
func obliquityRad(jde float64) float64 {
	tc := (jde - 2451545.0) / 36525.0
	eps := 23.4392911 - (46.8150*tc+0.00059*tc*tc-0.001813*tc*tc*tc)/3600
	return deg2rad(eps)
}

// gmstDeg returns Greenwich mean sidereal time (degrees) for the instant.
func gmstDeg(jde float64) float64 {
	tc := (jde - 2451545.0) / 36525.0
	g := 280.46061837 + 360.98564736629*(jde-2451545.0) + 0.000387933*tc*tc - tc*tc*tc/38710000
	g = math.Mod(g, 360)
	if g < 0 {
		g += 360
	}
	return g
}

// eclipticToEquatorial converts ecliptic (λ, β in degrees) to right ascension
// and declination (radians) at obliquity eps (radians).
func eclipticToEquatorial(lambda, beta, eps float64) (raRad, decRad float64) {
	l := deg2rad(lambda)
	b := deg2rad(beta)
	raRad = math.Atan2(math.Sin(l)*math.Cos(eps)-math.Tan(b)*math.Sin(eps), math.Cos(l))
	decRad = math.Asin(math.Sin(b)*math.Cos(eps) + math.Cos(b)*math.Sin(eps)*math.Sin(l))
	return raRad, decRad
}

// moonAltAzGeo returns the moon's geocentric altitude and azimuth (degrees,
// azimuth clockwise from north) at the coordinate and instant.
func moonAltAzGeo(latDeg, lonDeg float64, t time.Time) (altDeg, azDeg float64) {
	jde := julianDay(t)
	lambda, beta, _ := moonEcliptic(t)
	ra, dec := eclipticToEquatorial(lambda, beta, obliquityRad(jde))

	lstDeg := gmstDeg(jde) + lonDeg
	haRad := deg2rad(lstDeg) - ra
	latRad := deg2rad(latDeg)

	sinAlt := math.Sin(latRad)*math.Sin(dec) + math.Cos(latRad)*math.Cos(dec)*math.Cos(haRad)
	altDeg = math.Asin(clampUnit(sinAlt)) * 180 / math.Pi

	az := math.Atan2(math.Sin(haRad), math.Cos(haRad)*math.Sin(latRad)-math.Tan(dec)*math.Cos(latRad))
	azDeg = math.Mod(az*180/math.Pi+180, 360)
	if azDeg < 0 {
		azDeg += 360
	}
	return altDeg, azDeg
}

// MoonPosition returns the moon's elevation and azimuth (degrees, azimuth
// clockwise from north) and geocentric distance (km) at the coordinate/instant.
func MoonPosition(latDeg, lonDeg float64, t time.Time) (elevDeg, azDeg, distKm float64) {
	_, _, distKm = moonEcliptic(t)
	elevDeg, azDeg = moonAltAzGeo(latDeg, lonDeg, t)
	return elevDeg, azDeg, distKm
}

// sunApparentLongitude returns the sun's apparent geocentric ecliptic longitude
// (degrees) via the low-precision series of Meeus ch. 25.
func sunApparentLongitude(t time.Time) float64 {
	tc := (julianDay(t) - 2451545.0) / 36525.0
	l0 := 280.46646 + 36000.76983*tc + 0.0003032*tc*tc
	m := deg2rad(357.52911 + 35999.05029*tc - 0.0001537*tc*tc)
	c := (1.914602-0.004817*tc-0.000014*tc*tc)*math.Sin(m) +
		(0.019993-0.000101*tc)*math.Sin(2*m) + 0.000289*math.Sin(3*m)
	lambda := math.Mod(l0+c, 360)
	if lambda < 0 {
		lambda += 360
	}
	return lambda
}

// MoonPhase returns the illuminated fraction (percent), the phase angle
// (degrees; 0 = full, 180 = new), the moon's age (days since new moon), and the
// bilingual phase name for the instant.
func MoonPhase(t time.Time) (illumPct, phaseAngleDeg, ageDays float64, de, en string) {
	lambdaMoon, betaMoon, distMoon := moonEcliptic(t)
	lambdaSun := sunApparentLongitude(t)

	// Geocentric elongation ψ and phase angle i (Meeus ch. 48).
	cosPsi := math.Cos(deg2rad(betaMoon)) * math.Cos(deg2rad(lambdaMoon-lambdaSun))
	psi := math.Acos(clampUnit(cosPsi))
	const sunDistKm = 149_597_870.7
	i := math.Atan2(sunDistKm*math.Sin(psi), distMoon-sunDistKm*math.Cos(psi))
	illumPct = (1 + math.Cos(i)) / 2 * 100
	phaseAngleDeg = i * 180 / math.Pi

	// Elongation in ecliptic longitude drives the named phase and age.
	elong := math.Mod(lambdaMoon-lambdaSun, 360)
	if elong < 0 {
		elong += 360
	}
	ageDays = elong / 360 * synodicMonthDays
	de, en = moonPhaseName(elong)
	return round2(illumPct), round2(phaseAngleDeg), round2(ageDays), de, en
}

// moonPhaseName maps the moon–sun ecliptic elongation (degrees) to the eight
// conventional phase names.
func moonPhaseName(elongDeg float64) (de, en string) {
	switch bin := int(math.Mod(elongDeg+22.5, 360) / 45); bin {
	case 0:
		return phaseNewMoonDE, "New Moon"
	case 1:
		return "zunehmende Sichel", "Waxing Crescent"
	case 2:
		return "erstes Viertel", "First Quarter"
	case 3:
		return "zunehmender Mond", "Waxing Gibbous"
	case 4:
		return phaseFullMoonDE, "Full Moon"
	case 5:
		return "abnehmender Mond", "Waning Gibbous"
	case 6:
		return "letztes Viertel", "Last Quarter"
	default:
		return "abnehmende Sichel", "Waning Crescent"
	}
}

// MoonTimes computes moonrise and moonset (UTC) for the calendar day of t by
// scanning the moon's altitude for crossings of the standard altitude. Either
// value is nil when no such crossing occurs that day.
func MoonTimes(latDeg, lonDeg float64, t time.Time) (rise, set *time.Time) {
	day := t.UTC().Truncate(24 * time.Hour)
	const step = 10 * time.Minute
	steps := int(24 * time.Hour / step)

	prevT := day
	prevAlt, _ := moonAltAzGeo(latDeg, lonDeg, prevT)
	for n := 1; n <= steps; n++ {
		curT := day.Add(time.Duration(n) * step)
		curAlt, _ := moonAltAzGeo(latDeg, lonDeg, curT)
		if prevAlt < moonStandardAltDeg && curAlt >= moonStandardAltDeg && rise == nil {
			c := interpCrossing(prevT, prevAlt, curT, curAlt)
			rise = &c
		}
		if prevAlt >= moonStandardAltDeg && curAlt < moonStandardAltDeg && set == nil {
			c := interpCrossing(prevT, prevAlt, curT, curAlt)
			set = &c
		}
		prevT, prevAlt = curT, curAlt
	}
	return rise, set
}

// interpCrossing linearly interpolates the instant at which altitude equals the
// standard altitude between two bracketing samples.
func interpCrossing(t0 time.Time, a0 float64, t1 time.Time, a1 float64) time.Time {
	frac := (moonStandardAltDeg - a0) / (a1 - a0)
	return t0.Add(time.Duration(frac * float64(t1.Sub(t0))))
}
