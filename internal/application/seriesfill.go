package application

import (
	"context"
	"fmt"
	"time"
)

// fill gets the missing months. Each month has one owner at a time: this
// call reads the months it claims (adjacent ones in one upstream read, on a
// context detached from the caller) and waits for the ones another call is
// already reading, so overlapping reads — a weather day and an aggregate
// season of the same cell, or nearby points of a batch — share the work.
func (c *CachingSeriesReader) fill(ctx context.Context, s series, months []time.Time, keys []string, missing []int, blocks [][]float32) error {
	if len(missing) == 0 {
		return nil
	}
	claimKeys := make([]string, len(missing))
	for j, i := range missing {
		claimKeys[j] = keys[i]
	}
	calls, owned := c.flight.claim(claimKeys)
	for _, r := range ownedRuns(missing, owned) {
		lo, hi := r[0], r[1]
		go c.fillRun(context.WithoutCancel(ctx), s, months[missing[lo]:missing[hi-1]+1], claimKeys[lo:hi], calls[lo:hi])
	}
	for j, i := range missing {
		vals, err := calls[j].wait(ctx)
		if err != nil {
			return err
		}
		blocks[i] = vals
	}
	return nil
}

// fillRun reads adjacent claimed months and hands each to its waiters.
func (c *CachingSeriesReader) fillRun(detached context.Context, s series, months []time.Time, keys []string, calls []*blockCall) {
	ctx, cancel := context.WithTimeout(detached, blockFlightTimeout)
	defer cancel()
	vals, err := c.readMonths(ctx, s, months, keys)
	off := 0
	for i, m := range months {
		var v []float32
		if err == nil {
			n := hoursBetween(m, nextMonth(m))
			v, off = vals[off:off+n], off+n
		}
		c.flight.finish(keys[i], calls[i], v, err)
	}
}

// readMonths reads the months upstream and caches each. It first re-checks
// the cache: a fill that finished just before this one claimed them has them.
func (c *CachingSeriesReader) readMonths(ctx context.Context, s series, months []time.Time, keys []string) ([]float32, error) {
	if vals, ok := c.loadAll(ctx, months, keys); ok {
		return vals, nil
	}
	start := c.clock.Now().UTC() // freshness is decided by what was final when the read began
	end := nextMonth(months[len(months)-1])
	vals, err := c.inner.ReadSeries(ctx, s.domain, s.variable, s.coord, months[0], end)
	if err != nil {
		return nil, err
	}
	if len(vals) != hoursBetween(months[0], end) {
		return nil, fmt.Errorf("series cache: %s/%s: read %d hours, want %d", s.domain, s.variable, len(vals), hoursBetween(months[0], end))
	}
	off := 0
	for i, m := range months {
		n := hoursBetween(m, nextMonth(m))
		if ttl := c.ttlFor(nextMonth(m), start); ttl > 0 {
			_ = c.cache.Set(ctx, keys[i], encodeFloats(vals[off:off+n]), ttl)
		}
		off += n
	}
	return vals, nil
}

// loadAll returns the months joined when every one of them is cached.
func (c *CachingSeriesReader) loadAll(ctx context.Context, months []time.Time, keys []string) ([]float32, bool) {
	var out []float32
	for i, m := range months {
		b := c.load(ctx, keys[i], m)
		if b == nil {
			return nil, false
		}
		out = append(out, b...)
	}
	return out, true
}

// ownedRuns groups the claimed missing months into runs [lo, hi) over
// missing whose months are adjacent, so each run is one upstream read.
func ownedRuns(missing []int, owned []bool) [][2]int {
	var runs [][2]int
	for lo := 0; lo < len(missing); lo++ {
		if !owned[lo] {
			continue
		}
		hi := lo + 1
		for hi < len(missing) && owned[hi] && missing[hi] == missing[hi-1]+1 {
			hi++
		}
		runs = append(runs, [2]int{lo, hi})
		lo = hi - 1
	}
	return runs
}
