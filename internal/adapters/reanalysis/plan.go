package reanalysis

import (
	"context"
)

// plan decides what round 1 reads. For every variable it takes each model,
// by priority, whose files cover some of the hours, up to and including the
// first that covers all of them: a model without files there would only
// yield NaN (IFS before 2017), and models after a complete one are needed
// only where its values have gaps. It waits for a model's answer only when it
// gets to that model. A failed lookup schedules the read anyway: whether its
// failure matters is decided by collect, once it is clear whether the higher
// models leave anything for this one.
func plan(ctx context.Context, lookups [][]*lookup, vars []string, total int) [][]int {
	first := make([][]int, len(models))
	for vi := range vars {
		for m := range models {
			l := lookups[m][vi]
			if l == nil {
				continue
			}
			hours, err := l.wait(ctx)
			if err == nil && hours == 0 {
				continue
			}
			first[m] = append(first[m], vi)
			if err == nil && hours >= total {
				break
			}
		}
	}
	return first
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
