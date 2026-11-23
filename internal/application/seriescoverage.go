package application

import (
	"context"
	"time"
)

// Coverage implements output.SeriesReader. Cached months count with the
// hours they actually hold a value for; only the months missing from the
// cache are asked of the inner reader. A query whose months are all cached
// therefore plans without the bucket — it keeps working through a bucket
// outage and after a restart — and a month cached empty (a model without
// data for it) does not count as covered.
func (c *CachingSeriesReader) Coverage(ctx context.Context, domain, variable string, coord []uint64, from, to time.Time) (int, error) {
	from, to = from.UTC().Truncate(time.Hour), to.UTC().Truncate(time.Hour)
	if !to.After(from) || to.Sub(from) > c.opts.MaxSpan {
		return c.inner.Coverage(ctx, domain, variable, coord, from, to)
	}
	s := series{domain, variable, coord}
	n := 0
	var misses [][2]time.Time
	for _, m := range monthsCovering(from, to) {
		lo, hi := maxTime(from, m), minTime(to, nextMonth(m))
		b := c.load(ctx, c.key(s, m), m)
		if b == nil {
			if k := len(misses) - 1; k >= 0 && misses[k][1].Equal(lo) {
				misses[k][1] = hi
			} else {
				misses = append(misses, [2]time.Time{lo, hi})
			}
			continue
		}
		n += countValues(b[hoursBetween(m, lo):hoursBetween(m, hi)])
	}
	for _, r := range misses {
		k, err := c.inner.Coverage(ctx, domain, variable, coord, r[0], r[1])
		if err != nil {
			return 0, err
		}
		n += k
	}
	return n, nil
}

// countValues counts the hours that hold a value (not NaN).
func countValues(v []float32) int {
	n := 0
	for _, f := range v {
		if f == f {
			n++
		}
	}
	return n
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
