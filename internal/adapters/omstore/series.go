package omstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// ReadSeries returns the hourly values of variable in domain at the leading
// grid coordinate coord for the UTC hours [from, to). Hours no file covers are
// NaN. from and to are truncated to whole hours.
func (s *Store) ReadSeries(ctx context.Context, domain, variable string, coord []uint64, from, to time.Time) ([]float32, error) {
	h0, h1 := from.Unix()/3600, to.Unix()/3600
	if h1 <= h0 {
		return nil, fmt.Errorf("omstore: empty range %s … %s", from, to)
	}
	out := make([]float32, h1-h0)
	for i := range out {
		out[i] = float32(math.NaN())
	}
	cat, err := s.catalog(ctx, domain, variable)
	if err != nil {
		return nil, err
	}
	segs := cat.resolve(h0, h1)
	errs := make([]error, len(segs))
	var wg sync.WaitGroup
	for i, seg := range segs {
		wg.Go(func() {
			vals, err := s.readSegment(ctx, seg, coord)
			if err != nil {
				errs[i] = err
				return
			}
			copy(out[seg.out:], vals)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

// Coverage reports how many of the hours [from, to) of variable in domain
// lie in a file of the bucket. It costs a listing at most, never a data read.
func (s *Store) Coverage(ctx context.Context, domain, variable string, from, to time.Time) (int, error) {
	cat, err := s.catalog(ctx, domain, variable)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, seg := range cat.resolve(from.Unix()/3600, to.Unix()/3600) {
		hours := seg.t1 - seg.t0
		if hours > math.MaxInt32 { // a segment spans one file, a year at most
			return 0, fmt.Errorf("omstore: %s: segment of %d hours", seg.obj.Key, hours)
		}
		n += int(hours)
	}
	return n, nil
}

func (s *Store) readSegment(ctx context.Context, seg segment, coord []uint64) ([]float32, error) {
	a, err := s.array(ctx, seg.obj)
	if err != nil {
		return nil, err
	}
	p, err := a.PlanSeries(coord, seg.t0, seg.t1)
	if err != nil {
		return nil, fmt.Errorf("omstore: %s: %w", seg.obj.Key, err)
	}
	lut, err := s.lut(ctx, seg.obj, p)
	if err != nil {
		return nil, err
	}
	offs, err := p.Offsets(lut)
	if err != nil {
		return nil, fmt.Errorf("omstore: %s: %w", seg.obj.Key, err)
	}
	data, err := s.get(ctx, seg.obj, offs[0], offs[len(offs)-1]-offs[0])
	if err != nil {
		return nil, err
	}
	vals, err := p.Series(offs, data)
	if err != nil {
		return nil, fmt.Errorf("omstore: %s: %w", seg.obj.Key, err)
	}
	return vals, nil
}
