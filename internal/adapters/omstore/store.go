// Package omstore reads hourly series of single grid cells from the Open-Meteo
// open-data bucket (s3://openmeteo, served over plain HTTPS). It resolves hours
// onto year and chunk files, fetches only the byte ranges a series needs and
// decodes them with omfile.
//
// Per file and series it issues at most three range requests — tail (trailer
// and variable), LUT blocks, data — and caches the first two, so a warm read is
// one request per file. All files of a series are read in parallel.
package omstore

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/omfile"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const (
	// tailBytes is read from the end of a file; it holds the trailer and the
	// root variable (88 bytes for the arrays tempus reads).
	tailBytes = 4096

	defaultListTTL     = time.Hour
	defaultMaxParallel = 16
	defaultCacheItems  = 4096
	// maxBodyBytes caps any single response; a year of one cell is ~40 KiB.
	maxBodyBytes = 16 << 20
)

// ErrUpstream wraps a failure of the bucket (network, 5xx, unexpected status).
// It always arrives inside a transient output.ProviderError.
var ErrUpstream = errors.New("omstore: open-data bucket request failed")

// upstreamRetry is the retry hint for bucket failures.
const upstreamRetry = 30 * time.Second

func upstream(format string, args ...any) error {
	return output.NewTransientError(fmt.Errorf("%w: "+format, append([]any{ErrUpstream}, args...)...), upstreamRetry)
}

// Options configures a Store.
type Options struct {
	// BaseURL is the bucket root, e.g. https://openmeteo.s3.amazonaws.com.
	BaseURL     string
	HTTPClient  *http.Client
	MaxParallel int
	// ListTTL bounds how long a directory listing (and with it the ETags that
	// pin cached metadata) is trusted. Recent chunk files are rewritten daily.
	ListTTL time.Duration
	Now     func() time.Time
}

// Store reads series from the bucket. It is safe for concurrent use.
type Store struct {
	base    string
	client  *http.Client
	sem     chan struct{}
	listTTL time.Duration
	now     func() time.Time

	mu       sync.Mutex
	catalogs map[string]*catalog

	flight flightGroup
	arrays *lru[string, omfile.Array]
	luts   *lru[string, []byte]

	stats Stats
}

// Stats counts upstream traffic; used by tests and metrics.
type Stats struct {
	mu       sync.Mutex
	Requests int
	Bytes    int64
}

// Snapshot returns a copy of the counters.
func (s *Stats) Snapshot() (requests int, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Requests, s.Bytes
}

func (s *Stats) add(n int) {
	s.mu.Lock()
	s.Requests++
	s.Bytes += int64(n)
	s.mu.Unlock()
}

// New builds a Store.
func New(opts Options) *Store {
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	parallel := opts.MaxParallel
	if parallel <= 0 {
		parallel = defaultMaxParallel
	}
	ttl := opts.ListTTL
	if ttl <= 0 {
		ttl = defaultListTTL
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		base:     opts.BaseURL,
		client:   client,
		sem:      make(chan struct{}, parallel),
		listTTL:  ttl,
		now:      now,
		catalogs: map[string]*catalog{},
		arrays:   newLRU[string, omfile.Array](defaultCacheItems),
		luts:     newLRU[string, []byte](defaultCacheItems),
	}
}

// Stats exposes the traffic counters.
func (s *Store) Stats() *Stats { return &s.stats }

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

// array returns the parsed root variable, cached per key and ETag.
func (s *Store) array(ctx context.Context, obj object) (omfile.Array, error) {
	key := obj.Key + "@" + obj.ETag
	if a, ok := s.arrays.get(key); ok {
		return a, nil
	}
	v, err := s.flight.do(key, func() (any, error) {
		n := min(obj.Size, tailBytes)
		tail, err := s.get(ctx, obj, obj.Size-n, n)
		if err != nil {
			return nil, err
		}
		off, size, err := omfile.ParseTrailer(tail)
		if err != nil {
			return nil, fmt.Errorf("omstore: %s: %w", obj.Key, err)
		}
		start := obj.Size - n
		if off < start || off+size > obj.Size {
			// The variable lies outside the tail; fetch it on its own.
			if tail, err = s.get(ctx, obj, off, size); err != nil {
				return nil, err
			}
			start = off
		}
		a, err := omfile.ParseArray(tail[off-start : off-start+size])
		if err != nil {
			return nil, fmt.Errorf("omstore: %s: %w", obj.Key, err)
		}
		s.arrays.put(key, a)
		return a, nil
	})
	if err != nil {
		return omfile.Array{}, err
	}
	return v.(omfile.Array), nil
}

// lut returns the plan's LUT blocks, cached per key, ETag and block range.
func (s *Store) lut(ctx context.Context, obj object, p omfile.Plan) ([]byte, error) {
	off, n := p.LUTRange()
	key := obj.Key + "@" + obj.ETag + "#" + strconv.FormatUint(off, 10) + "+" + strconv.FormatUint(n, 10)
	if b, ok := s.luts.get(key); ok {
		return b, nil
	}
	v, err := s.flight.do(key, func() (any, error) {
		b, err := s.get(ctx, obj, off, n)
		if err != nil {
			return nil, err
		}
		s.luts.put(key, b)
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// get fetches [off, off+n) of obj. If-Match pins the ETag the cached metadata
// belongs to; a 412 means the file was rewritten, which invalidates the
// listing so the next read sees the new version.
func (s *Store) get(ctx context.Context, obj object, off, n uint64) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/"+obj.Key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	if obj.ETag != "" {
		req.Header.Set("If-Match", obj.ETag)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, upstream("%s: %w", obj.Key, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusPreconditionFailed, http.StatusNotFound:
		s.invalidate(obj.Key)
		return nil, upstream("%s changed or vanished (status %d)", obj.Key, resp.StatusCode)
	default:
		return nil, upstream("%s: status %d", obj.Key, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, upstream("%s: %w", obj.Key, err)
	}
	if uint64(len(b)) != n {
		return nil, upstream("%s: got %d of %d bytes", obj.Key, len(b), n)
	}
	s.stats.add(len(b))
	return b, nil
}

// invalidate drops the listing that contains key.
func (s *Store) invalidate(key string) {
	dir := key[:max(0, lastSlash(key))]
	s.mu.Lock()
	delete(s.catalogs, dir)
	s.mu.Unlock()
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// catalog returns the (cached) listing of data/<domain>/<variable>/.
func (s *Store) catalog(ctx context.Context, domain, variable string) (*catalog, error) {
	dir := "data/" + domain + "/" + variable
	s.mu.Lock()
	c, ok := s.catalogs[dir]
	s.mu.Unlock()
	if ok && s.now().Sub(c.fetched) < s.listTTL {
		return c, nil
	}
	v, err := s.flight.do("list:"+dir, func() (any, error) {
		objs, err := s.list(ctx, dir+"/")
		if err != nil {
			return nil, err
		}
		c := newCatalog(objs, s.now())
		s.mu.Lock()
		s.catalogs[dir] = c
		s.mu.Unlock()
		return c, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*catalog), nil
}

type listResult struct {
	Contents []struct {
		Key  string `xml:"Key"`
		ETag string `xml:"ETag"`
		Size uint64 `xml:"Size"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// list runs S3 ListObjectsV2 over prefix, following continuation tokens.
func (s *Store) list(ctx context.Context, prefix string) ([]object, error) {
	var objs []object
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, upstream("list %s: %w", prefix, err)
		}
		var res listResult
		err = func() error {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return upstream("list %s: status %d", prefix, resp.StatusCode)
			}
			return xml.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&res)
		}()
		if err != nil {
			return nil, err
		}
		for _, c := range res.Contents {
			objs = append(objs, object{Key: c.Key, ETag: c.ETag, Size: c.Size})
		}
		if !res.IsTruncated || res.NextContinuationToken == "" {
			return objs, nil
		}
		token = res.NextContinuationToken
	}
}
