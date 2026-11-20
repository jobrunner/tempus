package omstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testDomain = "testdom"
	varTemp    = "temperature_2m"
	varCloud   = "cloud_cover"
)

// fakeBucket serves testdata/bucket like the real bucket: ListObjectsV2,
// ranged GETs with ETag and If-Match. Tests can rewrite a file to change its
// ETag, and count requests.
type fakeBucket struct {
	mu       sync.Mutex
	files    map[string][]byte
	etags    map[string]string
	requests int
	lists    int
	pageSize int
}

func newFakeBucket(t *testing.T) *fakeBucket {
	t.Helper()
	b := &fakeBucket{files: map[string][]byte{}, etags: map[string]string{}, pageSize: 2}
	root := os.DirFS("testdata/bucket")
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		b.put(p, raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *fakeBucket) put(key string, raw []byte) {
	sum := sha256.Sum256(raw)
	b.mu.Lock()
	b.files[key] = raw
	b.etags[key] = `"` + hex.EncodeToString(sum[:8]) + `"`
	b.mu.Unlock()
}

func (b *fakeBucket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.URL.Path == "/" {
		b.lists++
		b.serveList(w, r)
		return
	}
	b.requests++
	key := strings.TrimPrefix(r.URL.Path, "/")
	raw, ok := b.files[key]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if m := r.Header.Get("If-Match"); m != "" && m != b.etags[key] {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	var lo, hi int
	if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &lo, &hi); err != nil || hi >= len(raw) || lo > hi {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	w.Header().Set("ETag", b.etags[key])
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(raw[lo : hi+1])
}

// serveList pages through keys under prefix, pageSize at a time, so the
// continuation-token loop is exercised.
func (b *fakeBucket) serveList(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	var keys []string
	for k := range b.files {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	slicesSort(keys)
	start, _ := strconv.Atoi(r.URL.Query().Get("continuation-token"))
	end := min(len(keys), start+b.pageSize)
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	for _, k := range keys[start:end] {
		fmt.Fprintf(&sb, "<Contents><Key>%s</Key><ETag>%s</ETag><Size>%d</Size></Contents>", k, b.etags[k], len(b.files[k]))
	}
	if end < len(keys) {
		fmt.Fprintf(&sb, "<IsTruncated>true</IsTruncated><NextContinuationToken>%d</NextContinuationToken>", end)
	} else {
		sb.WriteString("<IsTruncated>false</IsTruncated>")
	}
	sb.WriteString("</ListBucketResult>")
	_, _ = w.Write([]byte(sb.String()))
}

func slicesSort(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func newTestStore(t *testing.T, b *fakeBucket, now func() time.Time) *Store {
	t.Helper()
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	return New(Options{BaseURL: srv.URL, HTTPClient: srv.Client(), Now: now})
}

// expected recomputes the fixture value at epoch hour h for location loc.
func expected(h int64, loc int) float32 {
	return float32(h%1000+int64(7*loc)) / 20
}

func epochHour(t time.Time) int64 { return t.Unix() / 3600 }

func TestReadSeries_YearAndChunkFiles(t *testing.T) {
	s := newTestStore(t, newFakeBucket(t), time.Now)
	from := time.Date(2001, 12, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2002, 1, 20, 0, 0, 0, 0, time.UTC)
	got, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 2}, from, to)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got {
		if want := expected(epochHour(from)+int64(i), 2); v != want {
			t.Fatalf("hour %d: got %v want %v", i, v, want)
		}
	}
}

func TestReadSeries_GapsAndStaleChunksAreNaN(t *testing.T) {
	s := newTestStore(t, newFakeBucket(t), time.Now)
	// The stale chunk before year_2001 must be ignored; nothing else covers 2000.
	from := time.Date(2000, 12, 31, 0, 0, 0, 0, time.UTC)
	to := time.Date(2001, 1, 1, 2, 0, 0, 0, time.UTC)
	got, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 1}, from, to)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 24 {
		if !math.IsNaN(float64(got[i])) {
			t.Fatalf("hour %d before the first year file: got %v, want NaN", i, got[i])
		}
	}
	if got[24] != expected(epochHour(from)+24, 1) {
		t.Fatalf("first hour of 2001: got %v", got[24])
	}
	// Variables without year files are served from chunks only.
	got, err = s.ReadSeries(context.Background(), testDomain, varCloud, []uint64{0, 0}, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != expected(epochHour(from), 0) {
		t.Fatalf("chunk-only variable: got %v", got[0])
	}
}

func TestReadSeries_WarmReadsAreOneRequestPerFile(t *testing.T) {
	b := newFakeBucket(t)
	s := newTestStore(t, b, time.Now)
	from := time.Date(2001, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(48 * time.Hour)
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, to); err != nil {
		t.Fatal(err)
	}
	if b.requests != 3 {
		t.Fatalf("cold read: %d requests, want 3 (tail, LUT, data)", b.requests)
	}
	lists := b.lists
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 1}, from, to); err != nil {
		t.Fatal(err)
	}
	if b.requests != 4 || b.lists != lists {
		t.Fatalf("warm read: %d requests, %d list pages; want 4 and %d (listing cached)", b.requests, b.lists, lists)
	}
}

func TestReadSeries_RewrittenFileIsReloaded(t *testing.T) {
	b := newFakeBucket(t)
	now := time.Now()
	s := newTestStore(t, b, func() time.Time { return now })
	from := time.Date(2002, 1, 2, 0, 0, 0, 0, time.UTC)
	to := from.Add(3 * time.Hour)
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, to); err != nil {
		t.Fatal(err)
	}
	// Rewrite the chunk with other contents (here: another chunk's bytes).
	key := "data/testdom/temperature_2m/chunk_556.om"
	b.put(key, b.files["data/testdom/temperature_2m/chunk_558.om"])
	_, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, to)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("read against a rewritten file: err = %v, want ErrUpstream", err)
	}
	got, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, to)
	if err != nil {
		t.Fatalf("after invalidation: %v", err)
	}
	if want := expected(558*hoursPerChunkFile+epochHour(from)-556*hoursPerChunkFile, 0); got[0] != want {
		t.Fatalf("after rewrite: got %v want %v", got[0], want)
	}
}

func TestReadSeries_ListingExpires(t *testing.T) {
	b := newFakeBucket(t)
	now := time.Now()
	s := newTestStore(t, b, func() time.Time { return now })
	from := time.Date(2001, 3, 1, 0, 0, 0, 0, time.UTC)
	read := func() {
		if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, from.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	read()
	lists := b.lists
	now = now.Add(2 * defaultListTTL)
	read()
	if b.lists == lists {
		t.Fatal("listing was not refreshed after its TTL")
	}
}

func TestReadSeries_Errors(t *testing.T) {
	b := newFakeBucket(t)
	s := newTestStore(t, b, time.Now)
	from := time.Date(2001, 3, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, from); err == nil {
		t.Error("empty range: no error")
	}
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 9}, from, from.Add(time.Hour)); err == nil {
		t.Error("coordinate outside the grid: no error")
	}
	b.files["data/testdom/temperature_2m/year_2001.om"] = []byte("not an om file")
	if _, err := s.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, from.Add(time.Hour)); err == nil {
		t.Error("corrupt file: no error")
	}
	down := New(Options{BaseURL: "http://127.0.0.1:1", HTTPClient: &http.Client{Timeout: time.Second}})
	if _, err := down.ReadSeries(context.Background(), testDomain, varTemp, []uint64{0, 0}, from, from.Add(time.Hour)); !errors.Is(err, ErrUpstream) {
		t.Errorf("unreachable bucket: err = %v, want ErrUpstream", err)
	}
}

func TestFloorDiv(t *testing.T) {
	for _, c := range [][3]int64{{-505, 504, -2}, {-504, 504, -1}, {-1, 504, -1}, {0, 504, 0}, {505, 504, 1}} {
		if got := floorDiv(c[0], c[1]); got != c[2] {
			t.Errorf("floorDiv(%d, %d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestResolve_BeforeTheEpochUsesTheContainingChunk(t *testing.T) {
	c := newCatalog([]object{{Key: "d/v/chunk_-2.om"}, {Key: "d/v/chunk_-1.om"}}, time.Now())
	segs := c.resolve(-505, -503)
	if len(segs) != 2 || segs[0].obj.Key != "d/v/chunk_-2.om" || segs[0].t0 != 503 || segs[1].out != 1 {
		t.Fatalf("segments: %+v", segs)
	}
}

func TestFlightGroup_WaiterStopsOnItsOwnCancellation(t *testing.T) {
	var g flightGroup
	release := make(chan struct{})
	started := make(chan struct{})
	go func() {
		_, _ = g.do(context.Background(), "k", func() (any, error) {
			close(started)
			<-release
			return 1, nil
		})
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.do(ctx, "k", func() (any, error) { return 2, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter: err = %v", err)
	}
	close(release)
}
