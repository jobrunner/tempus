package domain

import "math"

// DewPointCelsius returns the dew point (°C) from air temperature (°C) and
// relative humidity (%), Magnus-Tetens (Sonntag-1990 coefficients, valid
// roughly -45..60 °C). ok is false for out-of-range humidity.
func DewPointCelsius(tempC, rhPct float64) (float64, bool) {
	if rhPct <= 0 || rhPct > 100 {
		return 0, false
	}
	const a, b = 17.62, 243.12
	gamma := math.Log(rhPct/100) + a*tempC/(b+tempC)
	return b * gamma / (a - gamma), true
}
