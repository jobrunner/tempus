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
