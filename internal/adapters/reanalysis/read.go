package reanalysis

import (
	"context"
	"errors"
	"sync"
	"time"

	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
)

// Archive variables read under another name than the output variable.
const (
	varSnowfallWater = "snowfall_water_equivalent"
	varWindU         = "wind_u_component_10m"
	varWindV         = "wind_v_component_10m"
)

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
		s, err := get(varSnowfallWater)
		for i := range s {
			s[i] = ra.SnowfallCm(s[i])
		}
		return s, err
	case VarWind:
		// Both components at once: one after the other costs a second
		// round-trip chain to the bucket.
		var u, w []float32
		var errU, errV error
		var wg sync.WaitGroup
		wg.Go(func() { u, errU = get(varWindU) })
		wg.Go(func() { w, errV = get(varWindV) })
		wg.Wait()
		if err := errors.Join(errU, errV); err != nil {
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
