package reanalysis

import (
	"context"
	"time"
)

// lookup is one coverage answer that may still be on its way.
type lookup struct {
	done  chan struct{}
	hours int
	err   error
}

// wait returns the answer, or ctx's error if ctx ends first.
func (l *lookup) wait(ctx context.Context) (int, error) {
	select {
	case <-l.done:
		return l.hours, l.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// coverage starts, for every model and the variables it provides, all at
// once, the question how many hours of [from, to) it can serve at the
// model's cell. lookups[m][vi] is nil where model m does not provide vi.
// Callers wait only for the answers they need: one stuck on an unneeded
// model (a bucket outage, with the needed ones cached) never holds a query.
func (a *Assembler) coverage(ctx context.Context, sels []selection, vars []string, from, to time.Time) [][]*lookup {
	lookups := make([][]*lookup, len(models))
	for m, sel := range sels {
		lookups[m] = make([]*lookup, len(vars))
		for vi, v := range vars {
			if !sel.model.vars[v] {
				continue
			}
			l := &lookup{done: make(chan struct{})}
			lookups[m][vi] = l
			go func() {
				l.hours, l.err = a.reader.Coverage(ctx, sel.model.name, storedName(v), sel.cell.Coord, from, to)
				close(l.done)
			}()
		}
	}
	return lookups
}

// mayCover reports whether a gap round should try the model: it has data
// for the period, or the lookup failed — then the read reports the trouble.
func mayCover(ctx context.Context, l *lookup) bool {
	if l == nil {
		return false
	}
	hours, err := l.wait(ctx)
	return err != nil || hours > 0
}
