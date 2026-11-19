package domain

import (
	"math"
	"time"
)

// Reanalysis variable names (the Open-Meteo names, used as keys of
// HourlySeries.Values).
const (
	VarTemperature = "temperature_2m"
	VarDewPoint    = "dew_point_2m"
	VarPrecip      = "precipitation"
	VarSnowfall    = "snowfall"
	VarWind        = "wind_speed_10m"
	VarCloud       = "cloud_cover"
	VarHumidity    = "relative_humidity_2m"
	VarWeatherCode = "weather_code"
)

// GeoPoint is a location with its height in metres above sea level.
type GeoPoint struct {
	Lat, Lon, Elevation float64
}

// PointOf returns the request's location.
func (r QueryRequest) PointOf() GeoPoint {
	return GeoPoint{Lat: r.Coordinate.Lat, Lon: r.Coordinate.Lon, Elevation: r.Elevation}
}

// ModelCell names the model grid point a series was read from. Elevation is
// the height the values refer to after selection (the target height when the
// model needed no correction).
type ModelCell struct {
	Model     string
	Lat, Lon  float64
	Elevation float64
}

// HourlySeries holds hourly values per variable from From on; NaN = missing.
// Values are unrounded: sums and extrema must not inherit display rounding.
type HourlySeries struct {
	From   time.Time
	Values map[string][]float64
	Cells  []ModelCell
}

// At returns the value of variable at the (hour-aligned) instant t.
func (h HourlySeries) At(variable string, t time.Time) (float64, bool) {
	s := h.Values[variable]
	i := int(t.Sub(h.From) / time.Hour)
	if i < 0 || i >= len(s) || math.IsNaN(s[i]) {
		return 0, false
	}
	return s[i], true
}

// DailySeries holds UTC-day aggregates from FirstDay on; NaN = missing.
// TMax/TMin are the extrema of the Hermite curve through the hourly values,
// TMean the mean of the 24 hours, PrecipSum the sum of hours 00–23.
type DailySeries struct {
	FirstDay                     time.Time
	TMax, TMin, TMean, PrecipSum []float64
	Cells                        []ModelCell
}

// Day returns the index of the UTC day containing t, or -1.
func (d DailySeries) Day(t time.Time) int {
	i := int(t.UTC().Truncate(24*time.Hour).Sub(d.FirstDay) / (24 * time.Hour))
	if i < 0 || i >= len(d.TMax) {
		return -1
	}
	return i
}

// ReanalysisLicense is the attribution every feature built from the
// reanalysis archive carries. The values come from ECMWF (IFS) and the
// Copernicus Climate Change Service (ERA5, ERA5-Land); Open-Meteo regridded
// and published them as an open dataset. All three require attribution.
func ReanalysisLicense() License {
	return License{
		Name: "CC-BY 4.0",
		URL:  "https://open-meteo.com/en/license",
		Attribution: "Weather data: ECMWF IFS (CC-BY 4.0) and Copernicus Climate Change Service " +
			"ERA5/ERA5-Land (contains modified Copernicus Climate Change Service information), " +
			"processed and distributed by Open-Meteo.com (CC-BY 4.0 open data)",
	}
}
