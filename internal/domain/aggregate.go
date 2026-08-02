package domain

import "time"

// DefaultGDDBaseCelsius is the default base temperature for growing-degree-day
// accumulation (a common all-purpose value for insects and warm-season crops).
const DefaultGDDBaseCelsius = 10.0

// AggregateSource is the attribution for the weather-aggregate feature.
const AggregateSource = "Zeitfenster-Aggregate (Niederschlag, Tages-Extrema, GDD) berechnet von tempus"

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
