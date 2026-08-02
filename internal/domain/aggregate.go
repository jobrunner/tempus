package domain

import (
	"math"
	"time"
)

// GDDBase5Celsius and GDDBase10Celsius are the two base temperatures always
// computed for growing-degree-days: 5 °C (cool-temperate plants/insects) and
// 10 °C (warm-season, general insect development).
const (
	GDDBase5Celsius  = 5.0
	GDDBase10Celsius = 10.0
)

// AggregateSource is the attribution for the weather-aggregate feature.
const AggregateSource = "Zeitfenster-Aggregate (Niederschlag, Tages-Extrema, GDD, Arrhenius-Zeit) berechnet von tempus"

// Arrhenius thermal-time constants. The activation energy is the Metabolic
// Theory of Ecology value (Brown et al. 2004; Gillooly et al. 2001); the
// reference temperature only rescales the whole index by a constant, so values
// at different references are interconvertible.
const (
	ArrheniusActivationEnergyEv = 0.65
	ArrheniusReferenceTempC     = 20.0
	boltzmannEvPerK             = 8.617333262e-5
)

// arrheniusRate is the Boltzmann-Arrhenius rate exp(−E/(k·T)) at temperature tC
// (°C), normalised to 1.0 at ArrheniusReferenceTempC:
// w(T) = exp[(E/k)·(1/Tref − 1/T)] with temperatures in Kelvin.
func arrheniusRate(tC float64) float64 {
	const eOverK = ArrheniusActivationEnergyEv / boltzmannEvPerK
	tK := tC + 273.15
	if tK <= 0 { // at/below absolute zero (only bad data) the rate is zero
		return 0
	}
	refK := ArrheniusReferenceTempC + 273.15
	r := math.Exp(eOverK * (1/refK - 1/tK))
	if math.IsInf(r, 0) || math.IsNaN(r) { // guard non-finite so JSON encoding never breaks
		return 0
	}
	return r
}

// ArrheniusThermalTime accumulates a species-agnostic, base-free thermal-time
// index over the paired daily minimum/maximum temperatures. Each day
// contributes the mean of the Boltzmann-Arrhenius rate at Tmin and Tmax (a
// 2-point average that captures the exponential non-linearity better than the
// daily mean). The result is in reference-temperature-equivalent days: a day at
// ArrheniusReferenceTempC contributes 1.0. Unlike a GDD base, the reference is
// only a normalising constant and never changes what is counted.
func ArrheniusThermalTime(dailyTmin, dailyTmax []float64) (value float64, days int) {
	n := len(dailyTmin)
	if len(dailyTmax) < n {
		n = len(dailyTmax)
	}
	for i := 0; i < n; i++ {
		value += (arrheniusRate(dailyTmin[i]) + arrheniusRate(dailyTmax[i])) / 2
	}
	return value, n
}

// GrowingDegreeDays accumulates growing-degree-days over the paired daily
// minimum/maximum temperatures using the simple average method with a lower
// floor at zero: GDD_i = max(0, (Tmax_i + Tmin_i)/2 − base). No upper cutoff is
// applied. It returns the accumulated value and the number of days summed
// (len of the shorter input slice).
func GrowingDegreeDays(dailyTmin, dailyTmax []float64, baseC float64) (gdd float64, days int) {
	n := len(dailyTmin)
	if len(dailyTmax) < n {
		n = len(dailyTmax)
	}
	for i := 0; i < n; i++ {
		if d := (dailyTmax[i]+dailyTmin[i])/2 - baseC; d > 0 {
			gdd += d
		}
	}
	return gdd, n
}

// GDDStartDate returns the start of the growing-degree-day accumulation window
// for an instant: 1 January of its year in the northern hemisphere, or 1 July
// of the current growing year in the southern hemisphere (so the season does
// not begin in the middle of the southern summer).
func GDDStartDate(instant time.Time, latDeg float64) time.Time {
	t := instant.UTC()
	if latDeg < 0 {
		start := time.Date(t.Year(), time.July, 1, 0, 0, 0, 0, time.UTC)
		if t.Before(start) {
			start = time.Date(t.Year()-1, time.July, 1, 0, 0, 0, 0, time.UTC)
		}
		return start
	}
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
}

// MinMaxMean returns the minimum, maximum and arithmetic mean of vals. ok is
// false for an empty slice.
func MinMaxMean(vals []float64) (mn, mx, mean float64, ok bool) {
	if len(vals) == 0 {
		return 0, 0, 0, false
	}
	mn, mx = vals[0], vals[0]
	var sum float64
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
		sum += v
	}
	return mn, mx, sum / float64(len(vals)), true
}
