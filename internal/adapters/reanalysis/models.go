package reanalysis

import (
	"math"

	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
)

// Bucket directory names of the three models.
const (
	domainERA5     = "copernicus_era5"
	domainERA5Land = "copernicus_era5_land"
	domainIFS      = "ecmwf_ifs"
)

// Output variables (Open-Meteo names).
const (
	VarTemperature = domain.VarTemperature
	VarDewPoint    = domain.VarDewPoint
	VarPrecip      = domain.VarPrecip
	VarSnowfall    = domain.VarSnowfall
	VarWind        = domain.VarWind
	VarCloud       = domain.VarCloud
	VarHumidity    = domain.VarHumidity
	VarWeatherCode = domain.VarWeatherCode
)

// rawVars are the variables read from the archive, in a fixed order.
var rawVars = []string{VarTemperature, VarDewPoint, VarPrecip, VarSnowfall, VarWind, VarCloud}

// expand adds the raw inputs derived variables need: humidity needs both
// temperatures, the weather code cloud cover, precipitation and snowfall, and
// snowfall the temperature for the snow rule.
func expand(want []string) map[string]bool {
	need := map[string]bool{}
	for _, v := range want {
		need[v] = true
	}
	if need[VarHumidity] {
		need[VarTemperature], need[VarDewPoint] = true, true
	}
	if need[VarWeatherCode] {
		need[VarCloud], need[VarPrecip], need[VarSnowfall] = true, true, true
	}
	if need[VarSnowfall] {
		need[VarTemperature] = true
	}
	return need
}

var nan32 = float32(math.NaN())

// model is one archive grid in priority order (highest first).
type model struct {
	name string
	grid ra.Grid
	// vars lists the output variables this model provides.
	vars map[string]bool
}

var allVars = map[string]bool{VarTemperature: true, VarDewPoint: true, VarPrecip: true, VarSnowfall: true, VarWind: true, VarCloud: true}

// models in priority order: a later model only fills hours an earlier one
// leaves empty. ERA5-Land carries only temperature and dew point.
var models = []model{
	{name: domainIFS, grid: ra.O1280, vars: allVars},
	{name: domainERA5Land, grid: ra.RegularGrid{Nx: 3600, Ny: 1801, LatMin: -90, LonMin: -180, D: 0.1},
		vars: map[string]bool{VarTemperature: true, VarDewPoint: true}},
	{name: domainERA5, grid: ra.RegularGrid{Nx: 1440, Ny: 721, LatMin: -90, LonMin: -180, D: 0.25}, vars: allVars},
}
