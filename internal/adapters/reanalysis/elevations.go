package reanalysis

import (
	"embed"
	"fmt"
	"sync"

	"github.com/jobrunner/tempus/internal/adapters/omfile"
)

// The model surface heights of the three grids, as published by Open-Meteo
// (static/HSURF.om, see data/MANIFEST and scripts/fetch-model-elevations.sh).
// They stay in OM format — smaller than any re-encoding — and are decoded one
// chunk at a time on demand.
//
//go:embed data/*.om
var elevationFiles embed.FS

// modelElevations serves grid point heights from one embedded HSURF file.
type modelElevations struct {
	file  []byte
	array omfile.Array
}

var (
	elevationsOnce sync.Once
	elevations     map[string]*modelElevations
	elevationsErr  error
)

// loadElevations parses the embedded files once.
func loadElevations() (map[string]*modelElevations, error) {
	elevationsOnce.Do(func() {
		elevations = map[string]*modelElevations{}
		for _, name := range []string{domainERA5, domainERA5Land, domainIFS} {
			raw, err := elevationFiles.ReadFile("data/" + name + ".om")
			if err != nil {
				elevationsErr = err
				return
			}
			off, size, err := omfile.ParseTrailer(raw)
			if err != nil {
				elevationsErr = fmt.Errorf("model elevations %s: %w", name, err)
				return
			}
			a, err := omfile.ParseArray(raw[off : off+size])
			if err != nil {
				elevationsErr = fmt.Errorf("model elevations %s: %w", name, err)
				return
			}
			elevations[name] = &modelElevations{file: raw, array: a}
		}
	})
	return elevations, elevationsErr
}

// At satisfies reanalysis.Elevations: the height of linear grid index i
// (NaN if the index is outside the grid or the data cannot be decoded).
func (m *modelElevations) At(i int) float32 {
	last := len(m.array.Dims) - 1
	n := m.array.Dims[last]
	if i < 0 {
		return nan32
	}
	idx := uint64(i)
	coord := make([]uint64, last)
	rest := idx / n
	for d := last - 1; d >= 0; d-- {
		coord[d] = rest % m.array.Dims[d]
		rest /= m.array.Dims[d]
	}
	if rest != 0 {
		return nan32
	}
	p, err := m.array.PlanSeries(coord, idx%n, idx%n+1)
	if err != nil {
		return nan32
	}
	off, size := p.LUTRange()
	offs, err := p.Offsets(m.file[off : off+size])
	if err != nil {
		return nan32
	}
	vals, err := p.Series(offs, m.file[offs[0]:offs[len(offs)-1]])
	if err != nil || len(vals) != 1 {
		return nan32
	}
	return vals[0]
}
