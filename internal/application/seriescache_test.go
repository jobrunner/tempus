package application

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

// hourValue is the fake archive: every hour has a distinct, recognisable value.
func hourValue(t time.Time) float32 { return float32(t.Unix()/3600%100000) / 10 }

type readCall struct{ from, to time.Time }

type fakeSeries struct {
	calls []readCall
	err   error
}

func (f *fakeSeries) ReadSeries(_ context.Context, _, _ string, _ []uint64, from, to time.Time) ([]float32, error) {
	f.calls = append(f.calls, readCall{from, to})
	if f.err != nil {
		return nil, f.err
	}
	out := make([]float32, int(to.Sub(from)/time.Hour))
	for i := range out {
		out[i] = hourValue(from.Add(time.Duration(i) * time.Hour))
	}
	return out, nil
}

var seriesNow = time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)

func seriesOpts() SeriesCacheOptions {
	return SeriesCacheOptions{Version: "1", Maturity: 72 * time.Hour,
		MatureTTL: 365 * 24 * time.Hour, ImmatureTTL: time.Hour, MaxSpan: 400 * 24 * time.Hour}
}

func checkSeries(t *testing.T, got []float32, from, to time.Time) {
	t.Helper()
	if want := int(to.Sub(from) / time.Hour); len(got) != want {
		t.Fatalf("len = %d, want %d", len(got), want)
	}
	for i, v := range got {
		if want := hourValue(from.Add(time.Duration(i) * time.Hour)); v != want {
			t.Fatalf("value[%d] = %v, want %v", i, v, want)
		}
	}
}

func TestSeriesCache_SecondReadHitsCache(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	for range 2 {
		got, err := r.ReadSeries(context.Background(), "era5", "temperature_2m", []uint64{1, 2}, from, to)
		if err != nil {
			t.Fatal(err)
		}
		checkSeries(t, got, from, to)
	}
	if len(inner.calls) != 1 {
		t.Fatalf("inner reads = %d, want 1", len(inner.calls))
	}
	// The miss reads the whole UTC month, so every other day of it hits.
	month := readCall{time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC)}
	if inner.calls[0] != month {
		t.Errorf("inner read %v, want the month %v", inner.calls[0], month)
	}
	if cache.setTTL != 365*24*time.Hour {
		t.Errorf("TTL = %v, want a year for a mature month", cache.setTTL)
	}
	other := from.AddDate(0, 0, 20)
	if _, err := r.ReadSeries(context.Background(), "era5", "temperature_2m", []uint64{1, 2}, other, other.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(inner.calls) != 1 {
		t.Errorf("another day of a cached month went upstream")
	}
}

func TestSeriesCache_KeysSeparateCellsVariablesAndDomains(t *testing.T) {
	inner := &fakeSeries{}
	r := NewCachingSeriesReader(inner, newFakeCache(), fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	ctx := context.Background()
	_, _ = r.ReadSeries(ctx, "era5", "temperature_2m", []uint64{1, 2}, from, from.Add(time.Hour))
	_, _ = r.ReadSeries(ctx, "era5", "temperature_2m", []uint64{1, 3}, from, from.Add(time.Hour))
	_, _ = r.ReadSeries(ctx, "era5", "precipitation", []uint64{1, 2}, from, from.Add(time.Hour))
	_, _ = r.ReadSeries(ctx, "era5_land", "temperature_2m", []uint64{1, 2}, from, from.Add(time.Hour))
	_, _ = r.ReadSeries(ctx, "era5", "temperature_2m", []uint64{12}, from, from.Add(time.Hour))
	if len(inner.calls) != 5 {
		t.Errorf("inner reads = %d, want 5 (no key collisions)", len(inner.calls))
	}
}

func TestSeriesCache_MergesMissingMonthsIntoOneRead(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	ctx := context.Background()
	coord := []uint64{7}
	// Warm May only.
	may := time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC)
	_, _ = r.ReadSeries(ctx, "era5", "t", coord, may, may.Add(time.Hour))

	from := time.Date(2020, 2, 15, 6, 0, 0, 0, time.UTC)
	to := time.Date(2020, 7, 3, 18, 0, 0, 0, time.UTC)
	got, err := r.ReadSeries(ctx, "era5", "t", coord, from, to)
	if err != nil {
		t.Fatal(err)
	}
	checkSeries(t, got, from, to)
	want := []readCall{
		{time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)},
		{time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)},
		{time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 8, 1, 0, 0, 0, 0, time.UTC)},
	}
	if len(inner.calls) != len(want) {
		t.Fatalf("inner reads = %v, want %v", inner.calls, want)
	}
	for i := range want {
		if inner.calls[i] != want[i] {
			t.Errorf("read %d = %v, want %v", i, inner.calls[i], want[i])
		}
	}
}

func TestSeriesCache_ImmatureMonthGetsShortTTL(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	from := seriesNow.Add(-48 * time.Hour)
	if _, err := r.ReadSeries(context.Background(), "ecmwf_ifs", "t", []uint64{1}, from, from.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if cache.setTTL != time.Hour {
		t.Errorf("TTL = %v, want 1h for a month still inside the maturity window", cache.setTTL)
	}
}

func TestSeriesCache_LongReadsBypassTheCache(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	from := time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, to)
	if err != nil {
		t.Fatal(err)
	}
	checkSeries(t, got, from, to)
	if cache.sets != 0 || len(inner.calls) != 1 || inner.calls[0] != (readCall{from, to}) {
		t.Errorf("sets=%d reads=%v, want a plain pass-through", cache.sets, inner.calls)
	}
}

func TestSeriesCache_ErrorsAreNotCached(t *testing.T) {
	boom := errors.New("bucket down")
	inner, cache := &fakeSeries{err: boom}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	if _, err := r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, from.Add(time.Hour)); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if cache.sets != 0 {
		t.Errorf("an error was cached")
	}
}

func TestSeriesCache_CorruptEntryIsAMiss(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	r := NewCachingSeriesReader(inner, cache, fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	_, _ = r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, from.Add(time.Hour))
	for k := range cache.store {
		cache.store[k] = []byte{1, 2, 3}
	}
	got, err := r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, from.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	checkSeries(t, got, from, from.Add(time.Hour))
	if len(inner.calls) != 2 {
		t.Errorf("inner reads = %d, want 2 (corrupt entry re-read)", len(inner.calls))
	}
}

func TestSeriesCache_KeepsNaN(t *testing.T) {
	cache := newFakeCache()
	nan := float32(math.NaN())
	r := NewCachingSeriesReader(nanSeries{nan}, cache, fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	for range 2 {
		got, _ := r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, from.Add(time.Hour))
		if len(got) != 1 || got[0] == got[0] {
			t.Fatalf("got %v, want [NaN]", got)
		}
	}
}

type nanSeries struct{ v float32 }

func (n nanSeries) ReadSeries(_ context.Context, _, _ string, _ []uint64, from, to time.Time) ([]float32, error) {
	out := make([]float32, int(to.Sub(from)/time.Hour))
	for i := range out {
		out[i] = n.v
	}
	return out, nil
}

// TestSeriesCache_ImmatureBlockExpiresWhenItsNextHourTurnsFinal: a block read
// at 12:20 holds hours that turn final at 13:00; it must not outlive that, or
// a year-long result could be computed from a pre-final value.
func TestSeriesCache_ImmatureBlockExpiresWhenItsNextHourTurnsFinal(t *testing.T) {
	inner, cache := &fakeSeries{}, newFakeCache()
	now := seriesNow.Add(20 * time.Minute)
	r := NewCachingSeriesReader(inner, cache, fixedClock{now}, seriesOpts())
	from := seriesNow.Add(-48 * time.Hour)
	if _, err := r.ReadSeries(context.Background(), "ecmwf_ifs", "t", []uint64{1}, from, from.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if cache.setTTL != 40*time.Minute {
		t.Errorf("TTL = %v, want 40m (until 13:00, when the next hour turns final)", cache.setTTL)
	}
}

// blockingSeries holds every read until release is closed.
type blockingSeries struct {
	fakeSeries
	mu      sync.Mutex
	release chan struct{}
}

func (b *blockingSeries) ReadSeries(ctx context.Context, d, v string, c []uint64, from, to time.Time) ([]float32, error) {
	<-b.release
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.fakeSeries.ReadSeries(ctx, d, v, c, from, to)
}

func TestSeriesCache_ConcurrentMissesShareOneRead(t *testing.T) {
	inner := &blockingSeries{release: make(chan struct{})}
	r := NewCachingSeriesReader(inner, newFakeLockedCache(), fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			got, err := r.ReadSeries(context.Background(), "era5", "t", []uint64{1}, from, from.Add(24*time.Hour))
			if err != nil {
				t.Error(err)
				return
			}
			checkSeries(t, got, from, from.Add(24*time.Hour))
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(inner.release)
	wg.Wait()
	if n := len(inner.calls); n != 1 {
		t.Errorf("inner reads = %d, want 1 (concurrent misses coalesced)", n)
	}
}

func TestSeriesCache_WaiterHonoursItsOwnContext(t *testing.T) {
	inner := &blockingSeries{release: make(chan struct{})}
	defer close(inner.release)
	r := NewCachingSeriesReader(inner, newFakeLockedCache(), fixedClock{seriesNow}, seriesOpts())
	from := time.Date(2020, 3, 5, 0, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.ReadSeries(ctx, "era5", "t", []uint64{1}, from, from.Add(time.Hour)); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// lockedCache is fakeCache safe for concurrent use.
type lockedCache struct {
	mu sync.Mutex
	*fakeCache
}

func newFakeLockedCache() *lockedCache { return &lockedCache{fakeCache: newFakeCache()} }

func (c *lockedCache) Get(ctx context.Context, k string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fakeCache.Get(ctx, k)
}

func (c *lockedCache) Set(ctx context.Context, k string, v []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fakeCache.Set(ctx, k, v, ttl)
}
