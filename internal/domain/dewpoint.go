package domain

import "math"

// DewPointComfortSource cites the origin of the dew-point comfort scale.
const (
	DewPointComfortSource    = "Taupunkt-Komfortskala (gängige meteorologische Einteilung)"
	DewPointComfortSourceURL = "https://en.wikipedia.org/wiki/Dew_point#Relationship_to_human_comfort"
)

// DewPointComfort classifies a dew point (°C) into a human-comfort category,
// returning German and English labels.
func DewPointComfort(tdC float64) (de, en string) {
	switch {
	case tdC <= 5:
		return "sehr trocken", "very dry"
	case tdC <= 10:
		return "trocken", "dry"
	case tdC <= 13:
		return "sehr angenehm", "very comfortable"
	case tdC <= 16:
		return "angenehm", "comfortable"
	case tdC <= 18:
		return "leicht schwül", "slightly humid"
	case tdC <= 21:
		return "schwül", "humid"
	case tdC <= 24:
		return "sehr schwül", "very humid"
	default:
		return "drückend", "oppressive"
	}
}

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
