package application_test

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// countingFeatures records Query calls and returns an envelope echoing the
// request so tests can assert per-point echoes after dedup fan-out.
type countingFeatures struct {
	mu       sync.Mutex
	calls    int
	inflight atomic.Int32
	maxInfl  atomic.Int32
	block    chan struct{} // nil ⇒ no blocking
	sawBatch atomic.Bool
}

func (f *countingFeatures) Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error) {
	if output.IsBatchOrigin(ctx) {
		f.sawBatch.Store(true)
	}
	cur := f.inflight.Add(1)
	for {
		m := f.maxInfl.Load()
		if cur <= m || f.maxInfl.CompareAndSwap(m, cur) {
			break
		}
	}
	defer f.inflight.Add(-1)
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return domain.QueryResult{}, ctx.Err()
		}
	}
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return domain.QueryResult{
		Query:     domain.QueryEcho{Coordinate: req.Coordinate, Datetime: req.Instant.UTC().Format(time.RFC3339)},
		Features:  []domain.Feature{},
		Providers: []domain.ProviderStatus{{ID: "fake", Kind: "fake", Status: domain.StatusOK}},
	}, nil
}

func point(id string, lat, lon float64) input.BatchPoint {
	return input.BatchPoint{ID: id, Req: &domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: lat, Lon: lon},
		Instant:    time.Date(2025, 6, 3, 14, 0, 0, 0, time.UTC),
	}}
}

func collect(t *testing.T, svc input.BatchService, points []input.BatchPoint) []input.BatchItem {
	t.Helper()
	var items []input.BatchItem
	err := svc.QueryBatch(context.Background(), points, func(it input.BatchItem) error {
		items = append(items, it)
		return nil
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	return items
}

func TestBatchService_DedupsAndPreservesOrder(t *testing.T) {
	f := &countingFeatures{}
	svc := application.NewBatchService(f, 2, 2)
	// p1 and p3 land in the same 0.01°-rounded cell → one upstream Query.
	items := collect(t, svc, []input.BatchPoint{
		point("a", 49.791, 9.951),
		point("b", 47.420, 10.980),
		point("c", 49.793, 9.952),
	})
	if f.calls != 2 {
		t.Errorf("Query calls = %d, want 2 (dedup)", f.calls)
	}
	for i, want := range []string{"a", "b", "c"} {
		if items[i].ID != want {
			t.Errorf("items[%d].ID = %q, want %q (input order)", i, items[i].ID, want)
		}
	}
	// Fan-out must echo each point's own coordinate, not the representative's.
	if items[2].Query.Coordinate.Lat != 49.793 {
		t.Errorf("deduped item echoes %v, want the point's own coordinate", items[2].Query.Coordinate)
	}
}

func TestBatchService_ParseErrorsBecomeErrorItems(t *testing.T) {
	f := &countingFeatures{}
	svc := application.NewBatchService(f, 2, 2)
	items := collect(t, svc, []input.BatchPoint{
		point("ok", 1, 2),
		{ID: "bad", ParseError: "invalid lat: must be a number in [-90,90]"},
	})
	if items[1].Error == nil || items[1].Error.Message == "" || items[1].QueryResult != nil {
		t.Fatalf("items[1] = %+v, want pure error item", items[1])
	}
	if f.calls != 1 {
		t.Errorf("Query calls = %d, want 1 (bad point never reaches the query path)", f.calls)
	}
}

func TestBatchService_BoundsConcurrencyAndMarksBatchOrigin(t *testing.T) {
	f := &countingFeatures{block: make(chan struct{})}
	svc := application.NewBatchService(f, 2, 2)
	done := make(chan struct{})
	var pts []input.BatchPoint
	for i := 0; i < 6; i++ {
		pts = append(pts, point(strconv.Itoa(i), float64(i), float64(i)))
	}
	go func() {
		defer close(done)
		_ = svc.QueryBatch(context.Background(), pts, func(input.BatchItem) error { return nil })
	}()
	time.Sleep(50 * time.Millisecond)
	close(f.block)
	<-done
	if m := f.maxInfl.Load(); m > 2 {
		t.Errorf("max in-flight = %d, want <= 2", m)
	}
	if !f.sawBatch.Load() {
		t.Error("Query contexts must be batch-origin-marked")
	}
}

func TestBatchService_EmitErrorAborts(t *testing.T) {
	f := &countingFeatures{}
	svc := application.NewBatchService(f, 1, 2)
	wantErr := context.Canceled // any sentinel error works here
	err := svc.QueryBatch(context.Background(),
		[]input.BatchPoint{point("a", 1, 2), point("b", 3, 4)},
		func(input.BatchItem) error { return wantErr })
	if err != wantErr {
		t.Fatalf("err = %v, want emit error propagated", err)
	}
}
