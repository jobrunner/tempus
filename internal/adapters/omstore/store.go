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
	"errors"
	"fmt"
	"net/http"
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
	// Meta, when set, keeps file metadata (root variables, LUT blocks) across
	// restarts; nil keeps it in memory only.
	Meta output.Cache
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
	meta   output.Cache

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
		meta:     opts.Meta,
	}
}

// Stats exposes the traffic counters.
func (s *Store) Stats() *Stats { return &s.stats }
