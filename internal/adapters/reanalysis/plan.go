package reanalysis

import (
	"context"
	"errors"
	"sync"
	"time"
)

// plan decides what round 1 reads. For every variable it takes each model,
// by priority, whose files cover some of the hours, up to and including the
// first that covers all of them: a model without files there would only
// yield NaN (IFS before 2017), and models after a complete one are needed
// only where its values have gaps. covered[m][vi] reports whether model m has
// any file for variable vi, for the gap rounds after round 1. The coverage
// comes from the bucket listings and costs no data read.
func (a *Assembler) plan(ctx context.Context, sels []selection, vars []string, from, to time.Time) (first [][]int, covered [][]bool, err error) {
	hours, err := a.coverage(ctx, sels, vars, from, to)
	if err != nil {
		return nil, nil, err
	}
	total := int(to.Sub(from) / time.Hour)
	first, covered = make([][]int, len(models)), make([][]bool, len(models))
	for m := range models {
		covered[m] = make([]bool, len(vars))
		for vi := range vars {
			covered[m][vi] = hours[m][vi] > 0
		}
	}
	for vi := range vars {
		for m := range models {
			if !covered[m][vi] {
				continue
			}
			first[m] = append(first[m], vi)
			if hours[m][vi] >= total {
				break
			}
		}
	}
	return first, covered, nil
}

// coverage asks the reader, for every model and the variables it provides,
// all at once, how many hours of [from, to) have a file.
func (a *Assembler) coverage(ctx context.Context, sels []selection, vars []string, from, to time.Time) ([][]int, error) {
	hours := make([][]int, len(models))
	errs := make([]error, len(models)*len(vars))
	var wg sync.WaitGroup
	for m, sel := range sels {
		hours[m] = make([]int, len(vars))
		for vi, v := range vars {
			if !sel.model.vars[v] {
				continue
			}
			wg.Go(func() {
				hours[m][vi], errs[m*len(vars)+vi] = a.reader.Coverage(ctx, sel.model.name, storedName(v), from, to)
			})
		}
	}
	wg.Wait()
	return hours, errors.Join(errs...)
}

// storedName is the archive variable an output variable is read from (the
// first one, for variables built from several).
func storedName(v string) string {
	switch v {
	case VarSnowfall:
		return varSnowfallWater
	case VarWind:
		return varWindU
	}
	return v
}

// ArchiveDirectories lists the bucket directories ("<domain>/<variable>")
// the models are read from, for keeping their listings warm.
func ArchiveDirectories() []string {
	var dirs []string
	for _, m := range models {
		for _, v := range rawVars {
			if !m.vars[v] {
				continue
			}
			names := []string{storedName(v)}
			if v == VarWind {
				names = append(names, varWindV)
			}
			for _, n := range names {
				dirs = append(dirs, m.name+"/"+n)
			}
		}
	}
	return dirs
}
