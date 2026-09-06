package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// BatchService processes batch points through the single-query path with a
// bounded worker pool. Points sharing a rounded coordinate + instant + options
// are fetched once and fanned out; emission strictly follows input order, so
// NDJSON consumers can track progress by counting lines.
type BatchService struct {
	features    input.FeatureService
	concurrency int
	precision   int
}

// NewBatchService builds the service; latLonPrecision must match the cache
// decorator's rounding so dedup and cache agree on "same point".
func NewBatchService(features input.FeatureService, concurrency, latLonPrecision int) *BatchService {
	if concurrency < 1 {
		concurrency = 1
	}
	return &BatchService{features: features, concurrency: concurrency, precision: latLonPrecision}
}

// group is one deduplicated fetch: all points with the same key share it.
type group struct {
	req  domain.QueryRequest
	done chan struct{}
	res  domain.QueryResult
	err  error
}

// QueryBatch implements input.BatchService.
func (s *BatchService) QueryBatch(ctx context.Context, points []input.BatchPoint, emit func(input.BatchItem) error) error {
	ctx, cancel := context.WithCancel(output.WithBatchOrigin(ctx))
	defer cancel() // stops the dispatcher when emit aborts early

	assign, order := s.buildGroups(points)
	sem := make(chan struct{}, s.concurrency)
	go s.dispatch(ctx, order, sem)

	for i, p := range points {
		if p.Req == nil {
			if err := emit(input.BatchItem{ID: p.ID, Error: &input.BatchItemError{Message: p.ParseError}}); err != nil {
				return err
			}
			continue
		}
		if err := s.awaitAndEmit(ctx, p, assign[i], emit); err != nil {
			return err
		}
	}
	return nil
}

// buildGroups deduplicates points sharing the same fetch key: assign maps each
// input point index to its group, and order lists each distinct group exactly
// once, in first-seen order (the order dispatch fires them in).
func (s *BatchService) buildGroups(points []input.BatchPoint) (assign []*group, order []*group) {
	groups := map[string]*group{}
	assign = make([]*group, len(points))
	for i, p := range points {
		if p.Req == nil {
			continue
		}
		k := s.groupKey(*p.Req)
		g, ok := groups[k]
		if !ok {
			g = &group{req: *p.Req, done: make(chan struct{})}
			groups[k] = g
			order = append(order, g)
		}
		assign[i] = g
	}
	return assign, order
}

// dispatch fetches each group through the bounded worker pool, releasing its
// semaphore slot as soon as the fetch completes. It returns (via the closed
// done channel) rather than an error since results are collected by the
// caller reading g.res/g.err after g.done closes.
func (s *BatchService) dispatch(ctx context.Context, order []*group, sem chan struct{}) {
	for _, g := range order {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		go func(g *group) {
			defer func() { <-sem }()
			g.res, g.err = s.features.Query(ctx, g.req)
			close(g.done)
		}(g)
	}
}

// awaitAndEmit waits for g's fetch to complete and emits the point's result
// with its own echo (coordinate/datetime), preserving input order.
func (s *BatchService) awaitAndEmit(ctx context.Context, p input.BatchPoint, g *group, emit func(input.BatchItem) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.done:
	}
	if g.err != nil {
		return g.err // Query only errors on caller-canceled contexts
	}
	res := g.res // shallow copy: the echo is rewritten per point, features are shared read-only
	res.Query = domain.QueryEcho{
		Coordinate: p.Req.Coordinate,
		Datetime:   p.Req.Instant.UTC().Format(time.RFC3339),
	}
	return emit(input.BatchItem{ID: p.ID, QueryResult: &res})
}

// groupKey identifies "the same fetch": rounded coordinate, instant, and the
// per-request options that change provider output.
func (s *BatchService) groupKey(req domain.QueryRequest) string {
	gdd := ""
	if req.GDDBaseCelsius != nil {
		gdd = fmt.Sprintf("%.2f", *req.GDDBaseCelsius)
	}
	return fmt.Sprintf("%.*f|%.*f|%s|%s|%s|%s",
		s.precision, roundTo(req.Coordinate.Lat, s.precision),
		s.precision, roundTo(req.Coordinate.Lon, s.precision),
		req.Instant.UTC().Format(time.RFC3339),
		gdd, req.RefPeriod, strings.Join(req.Providers, ","))
}
