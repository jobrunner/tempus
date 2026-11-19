// Package reanalysis provides the time-dependent weather features from the
// Open-Meteo open-data archive of ECMWF IFS, ERA5 and ERA5-Land, read straight
// from the bucket through the SeriesReader port and combined the way the
// Open-Meteo archive's best_match does (internal/domain/reanalysis).
package reanalysis

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
	"github.com/jobrunner/tempus/internal/ports/output"
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

// Location is a target point with its elevation in metres above sea level.
type Location struct {
	Lat, Lon, Elevation float32
}

// SelectedCell reports which grid point of a model a series came from.
type SelectedCell struct {
	Model          string
	Lat, Lon       float32
	ModelElevation float32
}

// Hourly is a mixed, corrected hourly series set starting at From.
type Hourly struct {
	From   time.Time
	Values map[string][]float32
	// Cells lists the selected grid points in priority order.
	Cells []SelectedCell
}

// Assembler reads and combines the model series for a location.
type Assembler struct {
	reader output.SeriesReader
}

// NewAssembler wires an Assembler onto a series reader.
func NewAssembler(r output.SeriesReader) *Assembler { return &Assembler{reader: r} }

type selection struct {
	model model
	cell  ra.Cell
}

// Hourly returns the hourly values of the wanted variables (and of the raw
// variables they derive from) for [from, to).
func (a *Assembler) Hourly(ctx context.Context, loc Location, from, to time.Time, want ...string) (Hourly, error) {
	elev, err := loadElevations()
	if err != nil {
		return Hourly{}, err
	}
	sels := make([]selection, len(models))
	cells := make([]SelectedCell, len(models))
	for i, m := range models {
		c := m.grid.Select(loc.Lat, loc.Lon, loc.Elevation, elev[m.name])
		sels[i] = selection{model: m, cell: c}
		lat, lon := m.grid.Coordinates(c.Index)
		cells[i] = SelectedCell{Model: m.name, Lat: lat, Lon: lon, ModelElevation: c.ModelElevation}
	}
	need := expand(want)
	var vars []string
	for _, v := range rawVars {
		if need[v] {
			vars = append(vars, v)
		}
	}
	perVar := make([][][]float32, len(vars))
	for i := range perVar {
		perVar[i] = make([][]float32, len(models))
	}
	// Round 1 reads the highest-priority model; later rounds only for the
	// variables that still have gaps, exactly where mixing would look.
	for m := range models {
		var need []int
		for vi, v := range vars {
			if !sels[m].model.vars[v] {
				continue
			}
			if m == 0 || hasGap(ra.Mix(perVar[vi][:m], true)) {
				need = append(need, vi)
			}
		}
		if err := a.readRound(ctx, sels[m], loc, vars, need, perVar, m, from, to); err != nil {
			return Hourly{}, err
		}
	}
	out := Hourly{From: from, Values: map[string][]float32{}, Cells: cells}
	for vi, v := range vars {
		mixed := ra.Mix(perVar[vi], true)
		if mixed == nil {
			mixed = nanSeries(int(to.Sub(from) / time.Hour))
		}
		out.Values[v] = mixed
	}
	if need[VarSnowfall] {
		ra.SnowRule(out.Values[VarSnowfall], out.Values[VarTemperature], sels[0].cell.ModelElevation, loc.Elevation)
	}
	n := int(to.Sub(from) / time.Hour)
	if need[VarHumidity] {
		t, td := out.Values[VarTemperature], out.Values[VarDewPoint]
		rh := make([]float32, n)
		for i := range rh {
			rh[i] = ra.RelativeHumidity(t[i], td[i])
		}
		out.Values[VarHumidity] = rh
	}
	if need[VarWeatherCode] {
		wc := make([]float32, n)
		for i := range wc {
			wc[i] = ra.WeatherCode(out.Values[VarCloud][i], out.Values[VarPrecip][i], out.Values[VarSnowfall][i])
		}
		out.Values[VarWeatherCode] = wc
	}
	return out, nil
}

// readRound reads the variables need of one model in parallel.
func (a *Assembler) readRound(ctx context.Context, sel selection, loc Location, vars []string, need []int, perVar [][][]float32, m int, from, to time.Time) error {
	errs := make([]error, len(need))
	var wg sync.WaitGroup
	for k, vi := range need {
		wg.Go(func() {
			s, err := a.read(ctx, sel, vars[vi], from, to)
			if err != nil {
				errs[k] = err
				return
			}
			if vars[vi] == VarTemperature || vars[vi] == VarDewPoint {
				ra.Correct(s, sel.cell.ModelElevation, loc.Elevation)
			}
			perVar[vi][m] = s
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// read fetches one output variable of one model, deriving it from the stored
// variables where the archive keeps components or other units.
func (a *Assembler) read(ctx context.Context, sel selection, v string, from, to time.Time) ([]float32, error) {
	get := func(name string) ([]float32, error) {
		return a.reader.ReadSeries(ctx, sel.model.name, name, sel.cell.Coord, from, to)
	}
	switch v {
	case VarSnowfall:
		s, err := get("snowfall_water_equivalent")
		for i := range s {
			s[i] = ra.SnowfallCm(s[i])
		}
		return s, err
	case VarWind:
		u, err := get("wind_u_component_10m")
		if err != nil {
			return nil, err
		}
		w, err := get("wind_v_component_10m")
		if err != nil {
			return nil, err
		}
		for i := range u {
			u[i] = ra.WindSpeedKmh(u[i], w[i])
		}
		return u, nil
	}
	return get(v)
}

func hasGap(s []float32) bool {
	for _, v := range s {
		if v != v {
			return true
		}
	}
	return s == nil
}

func nanSeries(n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = nan32
	}
	return s
}
