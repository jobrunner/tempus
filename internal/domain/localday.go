package domain

import (
	"time"
)

// The local day of an instant at a coordinate: the solar day whose solar
// noon is nearest to it. Sun and moon events are reported for this day
// rather than for the UTC calendar day, which far from Greenwich is the
// neighbouring local day.

// localSolarDay picks the UTC day whose local day (see dayBounds) holds t
// and returns it with its solar noon in minutes after the day's 00:00 UTC and
// the solar declination. That is the day whose solar noon is nearest to t,
// at most about twelve hours away. The UTC day of t and its neighbours are
// the only candidates: solar noon lies within ±12 h 16 min of 12:00 UTC.
func localSolarDay(lonDeg float64, t time.Time) (day time.Time, noonMin, declRad float64) {
	day = t.UTC().Truncate(24 * time.Hour)
	for _, offset := range []int{-1, 1} {
		d := day.AddDate(0, 0, offset)
		if from, to := dayBounds(lonDeg, d); !t.Before(from) && t.Before(to) {
			day = d
		}
	}
	noonMin, declRad = solarNoonMin(lonDeg, day)
	return day, noonMin, declRad
}

// solarNoonMin returns the solar noon of the UTC day in minutes after its
// 00:00 UTC, with the declination; both use the declination and equation of
// time at 12:00 UTC of the day.
func solarNoonMin(lonDeg float64, day time.Time) (noonMin, declRad float64) {
	decl, eqTime := solarGeom(day.Add(12 * time.Hour))
	return 720 - 4*lonDeg - eqTime, decl
}

// solarNoonOf returns the solar noon of the UTC day as an instant.
func solarNoonOf(lonDeg float64, day time.Time) time.Time {
	nm, _ := solarNoonMin(lonDeg, day)
	return day.Add(time.Duration(nm * float64(time.Minute)))
}

// dayBounds returns the local day around the solar noon of the UTC day:
// from halfway after the previous solar noon to halfway before the next.
// Adjacent days share their bounds, so consecutive local days tile time
// without gaps or overlaps — solar noons are not exactly 24 h apart.
func dayBounds(lonDeg float64, day time.Time) (from, to time.Time) {
	prev := solarNoonOf(lonDeg, day.AddDate(0, 0, -1))
	noon := solarNoonOf(lonDeg, day)
	next := solarNoonOf(lonDeg, day.AddDate(0, 0, 1))
	return prev.Add(noon.Sub(prev) / 2), noon.Add(next.Sub(noon) / 2)
}

// localDayBounds returns the bounds of the local day t falls into.
func localDayBounds(lonDeg float64, t time.Time) (from, to time.Time) {
	day, _, _ := localSolarDay(lonDeg, t)
	return dayBounds(lonDeg, day)
}

// MoonTimes computes moonrise and moonset (UTC) for the local day t falls
// into at the coordinate — from halfway after the previous solar noon to
// halfway before the next, the day SolarEvents reports — by scanning the
// moon's altitude for crossings of the standard altitude. Either value is nil
// when no such crossing occurs that day.
func MoonTimes(latDeg, lonDeg float64, t time.Time) (rise, set *time.Time) {
	from, to := localDayBounds(lonDeg, t)
	const step = 10 * time.Minute

	prevT := from
	prevAlt, _ := moonAltAzGeo(latDeg, lonDeg, prevT)
	for prevT.Before(to) {
		curT := prevT.Add(step)
		if curT.After(to) {
			curT = to
		}
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
