package reanalysis

import "math"

// LapseRate is the temperature change per metre of height (K/m).
const LapseRate float32 = 0.0065

// Correct shifts temperatures from the cell's model elevation to the target
// elevation in place: T += (model − target) · 0.0065.
func Correct(series []float32, modelElevation, targetElevation float32) {
	if modelElevation == targetElevation {
		return
	}
	c := float32(float32(modelElevation-targetElevation) * LapseRate)
	for i, v := range series {
		series[i] = v + c
	}
}

// smoothWidth is the number of hours before a gap of the higher-priority
// series that are blended with the lower-priority one.
const smoothWidth = 3

// Mix combines series ordered from highest to lowest priority: the first
// series wins, lower ones only fill its NaN hours. Mixing stops as soon as no
// NaN is left. With smooth set (all but categorical variables), the hours just
// before a filled gap fade linearly from the higher- to the lower-priority
// value so the series has no jump at the gap. The inputs are not modified.
func Mix(byPriority [][]float32, smooth bool) []float32 {
	var out []float32
	for _, s := range byPriority {
		if s == nil {
			continue
		}
		if out == nil {
			out = append([]float32(nil), s...)
		} else {
			fill(out, s, smooth)
		}
		if !hasNaN(out) {
			break
		}
	}
	return out
}

// fill writes lower into the NaN hours of out, walking backwards in time so
// the distance to the next filled gap is known.
func fill(out, lower []float32, smooth bool) {
	since := smoothWidth
	for x := len(lower) - 1; x >= 0; x-- {
		since++
		if isNaN(lower[x]) {
			continue
		}
		if isNaN(out[x]) {
			out[x] = lower[x]
			since = 0
			continue
		}
		if !smooth || since > smoothWidth {
			continue
		}
		a := float32(lower[x] * float32(smoothWidth+1-since))
		b := float32(out[x] * float32(since))
		out[x] = (a + b) / float32(smoothWidth+1)
	}
}

func hasNaN(s []float32) bool {
	for _, v := range s {
		if isNaN(v) {
			return true
		}
	}
	return false
}

func isNaN(v float32) bool { return v != v }

// SnowRule removes snowfall where the corrected temperature is at or above
// 0 °C. The reference applies it to the mixed series when the IFS cell's
// elevation differs from the target by more than the shortcut distance (the
// snow then fell at another height than the one reported).
func SnowRule(snowfall, temperature []float32, ifsModelElevation, target float32) {
	if absf(ifsModelElevation-target) <= shortcutMetres {
		return
	}
	for i, t := range temperature {
		if t >= 0 {
			snowfall[i] = 0
		}
	}
}

// RelativeHumidity from temperature and dew point (°C), Magnus formula with
// the reference's coefficients, clamped to [0, 100].
func RelativeHumidity(t, td float32) float32 {
	const beta, lambda float32 = 17.625, 243.04
	num := float32(100 * expf(float32(beta*td)/(lambda+td)))
	rh := num / expf(float32(beta*t)/(lambda+t))
	return max(min(rh, 100), 0)
}

func expf(x float32) float32 { return float32(math.Exp(float64(x))) }

// WeatherCode derives the WMO code from cloud cover (%), precipitation (mm)
// and snowfall (cm) as the archive does; NaN if an input is missing.
func WeatherCode(cloud, precip, snow float32) float32 {
	if isNaN(cloud) || isNaN(precip) || isNaN(snow) {
		return float32(math.NaN())
	}
	switch {
	case snow >= 0.8:
		return 75
	case snow >= 0.2:
		return 73
	case snow >= 0.01:
		return 71
	case precip >= 7.6:
		return 65
	case precip >= 2.5:
		return 63
	case precip >= 1.3:
		return 61
	case precip >= 1.0:
		return 55
	case precip >= 0.5:
		return 53
	case precip >= 0.01:
		return 51
	case cloud >= 80:
		return 3
	case cloud >= 50:
		return 2
	case cloud >= 20:
		return 1
	case cloud >= 0:
		return 0
	}
	return float32(math.NaN())
}

// WindSpeedKmh from u/v components in m/s.
func WindSpeedKmh(u, v float32) float32 {
	return float32(sqrtf(float32(u*u)+float32(v*v)) * 3.6)
}

// SnowfallCm from snowfall water equivalent in mm.
func SnowfallCm(waterEquivalent float32) float32 {
	return float32(waterEquivalent * 0.7)
}
