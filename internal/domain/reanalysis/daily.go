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

// intervalExtrema bounds the Hermite curve between b and c on t ∈ [0, 1],
// with tangents (c−a)/2 and (d−b)/2. The cubic's derivative is solved with
// the numerically stable quadratic formula.
func intervalExtrema(a, b, c, d float64) (lo, hi float64, ok bool) {
	if math.IsNaN(b) || math.IsNaN(c) {
		return 0, 0, false
	}
	if math.IsNaN(a) {
		a = b
	}
	if math.IsNaN(d) {
		d = c
	}
	// float64(…) around every product keeps the compiler from fusing it with
	// the following addition (FMA), which would change the last bit.
	m0, m1 := (c-a)/2, (d-b)/2
	k3 := float64(2*(b-c)) + m0 + m1
	k2 := float64(3*(c-b)) - float64(2*m0) - m1
	val := func(t float64) float64 {
		v := float64(k3*t) + k2
		v = float64(v*t) + m0
		return float64(v*t) + b
	}
	// The end points come from the polynomial too, not from b and c: the
	// rounding of val(1) is part of the reference result.
	lo, hi = math.Min(val(0), val(1)), math.Max(val(0), val(1))
	inside := func(t float64) {
		if t > 0 && t < 1 {
			v := val(t)
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	qa, qb, qc := 3*k3, 2*k2, m0
	switch {
	case qa == 0 && qb != 0:
		inside(-qc / qb)
	case qa != 0:
		if disc := float64(qb*qb) - float64(4*qa*qc); disc >= 0 {
			sign := 1.0
			if qb < 0 {
				sign = -1
			}
			q := -0.5 * (qb + float64(sign*math.Sqrt(disc)))
			inside(q / qa)
			if q != 0 {
				inside(qc / q)
			}
		}
	}
	return lo, hi, true
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
