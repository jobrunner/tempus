package reanalysis

import "math"

// HoursPerDay is the length of a UTC day in hourly series.
const HoursPerDay = 24

// DailyMinMax returns the minimum and maximum of the day starting at hourly
// index start, taken over the cubic Hermite (Catmull-Rom) curve through the
// hourly values rather than the samples themselves: the day consists of the 24
// intervals (h, h+1] for h = start … start+23, so the series must reach from
// start−1 to start+25; missing neighbours repeat the interval's end value.
func DailyMinMax(series []float32, start int) (lo, hi float32) {
	at := func(i int) float64 {
		if i < 0 || i >= len(series) {
			return math.NaN()
		}
		return float64(series[i])
	}
	mn, mx := math.Inf(1), math.Inf(-1)
	for h := start; h < start+HoursPerDay; h++ {
		l, u, ok := intervalExtrema(at(h-1), at(h), at(h+1), at(h+2))
		if !ok {
			continue
		}
		mn, mx = math.Min(mn, l), math.Max(mx, u)
	}
	if math.IsInf(mn, 1) {
		return float32(math.NaN()), float32(math.NaN())
	}
	return float32(mn), float32(mx)
}

// DailyMean is the float32 mean of the day's 24 hours (NaN if any is missing).
func DailyMean(series []float32, start int) float32 {
	return DailySum(series, start) / HoursPerDay
}

// DailySum is the float32 sum of the day's 24 hours (NaN if any is missing).
func DailySum(series []float32, start int) float32 {
	var sum float32
	for _, v := range series[start : start+HoursPerDay] {
		sum += v
	}
	return sum
}

// Round rounds like the reference's JSON output: scale in float32, round half
// away from zero, scale back.
func Round(x float32, decimals int) float32 {
	if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
		return x
	}
	p := float32(math.Pow10(decimals))
	return float32(roundHalfAway(float32(x*p))) / p
}
