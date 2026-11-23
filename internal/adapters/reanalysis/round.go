package reanalysis

import (
	"context"
	"time"

	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
)

// pendingRead is one model's series of one variable that may still be on
// its way.
type pendingRead struct {
	done chan struct{}
	vals []float32
	err  error
}

// startRound starts every read of round 1 at once — first[m] lists the
// variables of model m — and returns them as pending[m][vi] (nil where
// nothing was started).
func (a *Assembler) startRound(ctx context.Context, sels []selection, loc Location, vars []string, first [][]int, from, to time.Time) [][]*pendingRead {
	pending := make([][]*pendingRead, len(models))
	for m := range models {
		pending[m] = make([]*pendingRead, len(vars))
		for _, vi := range first[m] {
			p := &pendingRead{done: make(chan struct{})}
			pending[m][vi] = p
			go func() {
				defer close(p.done)
				p.vals, p.err = a.read(ctx, sels[m], vars[vi], from, to)
				if p.err == nil && (vars[vi] == VarTemperature || vars[vi] == VarDewPoint) {
					ra.Correct(p.vals, sels[m].cell.ModelElevation, loc.Elevation)
				}
			}()
		}
	}
	return pending
}

// collect takes the round-1 reads into perVar in priority order. It stops
// for a variable as soon as the higher-priority models leave no gap: a lower
// model is then not needed, so neither its answer nor its failure matters,
// and a query whose higher models are complete (often from the cache) never
// waits for — or fails on — a lower one.
func collect(ctx context.Context, vars []string, perVar [][][]float32, pending [][]*pendingRead) error {
	for vi := range vars {
		for m := range models {
			p := pending[m][vi]
			if p == nil {
				continue
			}
			if m > 0 && !hasGap(ra.Mix(perVar[vi][:m], true)) {
				break
			}
			select {
			case <-p.done:
			case <-ctx.Done():
				return ctx.Err()
			}
			if p.err != nil {
				return p.err
			}
			perVar[vi][m] = p.vals
		}
	}
	return nil
}
