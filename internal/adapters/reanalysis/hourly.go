// Package reanalysis provides the time-dependent weather features from the
// Open-Meteo open-data archive of ECMWF IFS, ERA5 and ERA5-Land, read straight
// from the bucket through the SeriesReader port and combined the way the
// Open-Meteo archive's best_match does (internal/domain/reanalysis).
package reanalysis

import (
	"context"
	"errors"
	"sync"
	"time"

	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
	"github.com/jobrunner/tempus/internal/ports/output"
)

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
	sels, cells := selectCells(loc, elev)
	need := expand(want)
	vars := rawOrder(need)
	perVar, err := a.readModels(ctx, sels, loc, vars, from, to)
	if err != nil {
		return Hourly{}, err
	}
	n := int(to.Sub(from) / time.Hour)
	out := Hourly{From: from, Values: mixAll(vars, perVar, n), Cells: cells}
	derive(out.Values, need, sels[0].cell.ModelElevation, loc.Elevation, n)
	return out, nil
}

// selectCells picks every model's grid point for loc.
func selectCells(loc Location, elev map[string]*modelElevations) ([]selection, []SelectedCell) {
	sels := make([]selection, len(models))
	cells := make([]SelectedCell, len(models))
	for i, m := range models {
		c := m.grid.Select(loc.Lat, loc.Lon, loc.Elevation, elev[m.name])
		sels[i] = selection{model: m, cell: c}
		lat, lon := m.grid.Coordinates(c.Index)
		cells[i] = SelectedCell{Model: m.name, Lat: lat, Lon: lon, ModelElevation: c.ModelElevation}
	}
	return sels, cells
}

// rawOrder lists the needed raw variables in rawVars order.
func rawOrder(need map[string]bool) []string {
	var vars []string
	for _, v := range rawVars {
		if need[v] {
			vars = append(vars, v)
		}
	}
	return vars
}

// readModels reads vars from every model into perVar[variable][model].
// Round 1 reads, all models at once, what plan picked; further rounds read a
// lower-priority model only for variables whose mix of everything read so far
// still has a gap (sea hours of a land model, a partly written file). Mixing
// takes a lower model only where the higher ones have gaps, so reading it in
// round 1 changes nothing — it only saves a round-trip chain.
func (a *Assembler) readModels(ctx context.Context, sels []selection, loc Location, vars []string, from, to time.Time) ([][][]float32, error) {
	perVar := make([][][]float32, len(vars))
	for i := range perVar {
		perVar[i] = make([][]float32, len(models))
	}
	lookups := a.coverage(ctx, sels, vars, from, to)
	first, err := plan(ctx, lookups, vars, int(to.Sub(from)/time.Hour))
	if err != nil {
		return nil, err
	}
	errs := make([]error, len(models))
	var wg sync.WaitGroup
	for m := range models {
		wg.Go(func() { errs[m] = a.readRound(ctx, sels[m], loc, vars, first[m], perVar, m, from, to) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	for m := range models {
		need := gapped(ctx, sels[m].model, vars, perVar, lookups[m], m)
		if err := a.readRound(ctx, sels[m], loc, vars, need, perVar, m, from, to); err != nil {
			return nil, err
		}
	}
	return perVar, nil
}

// gapped returns the variables model m (provided by mdl) still has to read:
// those it has not read whose mix of everything read so far still has a gap
// and which it may cover. Its coverage answer is awaited only then.
func gapped(ctx context.Context, mdl model, vars []string, perVar [][][]float32, lookups []*lookup, m int) []int {
	var need []int
	for vi, v := range vars {
		if mdl.vars[v] && perVar[vi][m] == nil && hasGap(ra.Mix(perVar[vi], true)) && mayCover(ctx, lookups[vi]) {
			need = append(need, vi)
		}
	}
	return need
}
