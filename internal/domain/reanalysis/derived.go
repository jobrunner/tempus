package reanalysis

import "math"

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

// WindSpeedKmh from u/v components in m/s.
func WindSpeedKmh(u, v float32) float32 {
	return float32(sqrtf(float32(u*u)+float32(v*v)) * 3.6)
}

// SnowfallCm from snowfall water equivalent in mm.
func SnowfallCm(waterEquivalent float32) float32 {
	return float32(waterEquivalent * 0.7)
}
