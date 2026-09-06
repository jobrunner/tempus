# Batch-Query Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `POST /api/v1/query/batch` (sync + NDJSON-Streaming, ortus-konform) mit selbstdrosselndem Open-Meteo-Zugriff, aggregate-Caching und Frontend-Batch-Tab.

**Architecture:** Hexagonal. Ein gemeinsamer `http.RoundTripper` (Paket `omhttp`) drosselt, retryt und budgetiert alle Open-Meteo-Calls und wird als `*http.Client` in die drei bestehenden Adapter injiziert (kein Adapter→Adapter-Import). Ein `BatchService` in `application` dedupliziert Punkte, arbeitet sie über den bestehenden `FeatureService.Query` mit begrenzter Nebenläufigkeit ab und emittiert Items in Eingabereihenfolge. Der HTTP-Adapter rendert sync-Envelope oder NDJSON.

**Tech Stack:** Go, gorilla/mux, viper, `golang.org/x/time/rate` (neue direkte Dependency), Vanilla-JS-Frontend (go:embed).

**Spec:** `docs/superpowers/specs/2026-09-06-batch-query-design.md`

## Global Constraints

- `make verify` (fmt-check, vet, lint, test, arch, debt-guard) muss nach JEDEM Task grün sein; der pre-commit-Hook erzwingt fmt/build/debt-guard.
- depguard: Adapter importieren keine anderen Adapter, kein `application`, kein `app`. `application` importiert keine Adapter. Geteilte Typen gehen in `ports`/`domain`.
- Jede Route unter `/api/v1` muss in `internal/adapters/http/openapi.yaml` dokumentiert sein (`TestRoutesMatchOpenAPISpec`); der Spiegel `api/openapi/openapi.yaml` muss **byte-identisch** sein (`scripts/openapi-mirror-check.sh`) → nach jeder Spec-Änderung `cp internal/adapters/http/openapi.yaml api/openapi/openapi.yaml`.
- Jedes Schema mit `properties` braucht `type: object` (`TestObjectSchemasDeclareTheirType`); bei `allOf`-Komposition im tragenden Member.
- Conventional Commits (release-please). Commit-Messages enden mit der Claude-Session-Zeile, wie in den bisherigen Commits.
- Config: viper, Prefix `TEMPUS`, `.`→`_`. Neue Keys IMMER mit Default in `config.Defaults()`.
- Kommentare/Code englisch, im Stil der umgebenden Dateien (erklärende Doc-Kommentare, keine Redundanz).

---

### Task 1: Batch-Origin-Marker + `omhttp`-Transport (Limiter, Retry, Tagesbudget)

**Files:**
- Create: `internal/ports/output/batchorigin.go`
- Create: `internal/ports/output/batchorigin_test.go`
- Create: `internal/adapters/omhttp/omhttp.go`
- Create: `internal/adapters/omhttp/budget.go`
- Test: `internal/adapters/omhttp/omhttp_test.go`, `internal/adapters/omhttp/budget_test.go`

**Interfaces:**
- Consumes: `output.Clock` (`Now() time.Time`), `golang.org/x/time/rate`.
- Produces: `output.WithBatchOrigin(ctx) context.Context`, `output.IsBatchOrigin(ctx) bool`; `omhttp.NewBudget(limit int, clock output.Clock) *Budget` mit `TryReserve(weight int) bool`; `omhttp.NewTransport(opts omhttp.Options) *Transport` (implementiert `http.RoundTripper`); `omhttp.ErrBudgetExhausted`. `omhttp.Options{Limiter *rate.Limiter, Budget *Budget, Weight int, RetryAttempts int, Base http.RoundTripper}`.

- [ ] **Step 1: Dependency holen**

```bash
cd /Users/jbrunner/work/projects/tempus && go get golang.org/x/time@latest && go mod tidy
```

- [ ] **Step 2: Failing Tests für den Batch-Origin-Marker schreiben**

`internal/ports/output/batchorigin_test.go`:

```go
package output_test

import (
	"context"
	"testing"

	"github.com/jobrunner/tempus/internal/ports/output"
)

func TestBatchOrigin_RoundTrip(t *testing.T) {
	ctx := context.Background()
	if output.IsBatchOrigin(ctx) {
		t.Fatal("plain context must not be batch-origin")
	}
	if !output.IsBatchOrigin(output.WithBatchOrigin(ctx)) {
		t.Fatal("marked context must be batch-origin")
	}
}
```

- [ ] **Step 3: Test laufen lassen — muss fehlschlagen**

Run: `go test ./internal/ports/output/ -run TestBatchOrigin -v`
Expected: FAIL (compile error: `WithBatchOrigin` undefined)

- [ ] **Step 4: Marker implementieren**

`internal/ports/output/batchorigin.go`:

```go
package output

import "context"

// batchOriginKey marks a context as originating from the batch path. The
// Open-Meteo transport enforces the daily call budget only for batch-marked
// requests, so interactive single queries are never budget-limited.
type batchOriginKey struct{}

// WithBatchOrigin marks ctx as coming from the batch path.
func WithBatchOrigin(ctx context.Context) context.Context {
	return context.WithValue(ctx, batchOriginKey{}, true)
}

// IsBatchOrigin reports whether ctx was marked via WithBatchOrigin.
func IsBatchOrigin(ctx context.Context) bool {
	v, _ := ctx.Value(batchOriginKey{}).(bool)
	return v
}
```

- [ ] **Step 5: Test grün** — `go test ./internal/ports/output/ -run TestBatchOrigin -v` → PASS

- [ ] **Step 6: Failing Tests für das Budget schreiben**

`internal/adapters/omhttp/budget_test.go` (Fake-Clock lokal im Test):

```go
package omhttp

import (
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestBudget_ReserveUntilExhausted(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	b := NewBudget(10, clk)
	if !b.TryReserve(6) || !b.TryReserve(4) {
		t.Fatal("reservations within the limit must succeed")
	}
	if b.TryReserve(1) {
		t.Fatal("reservation beyond the limit must fail")
	}
}

func TestBudget_ResetsAtUTCDayRollover(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 23, 0, 0, 0, time.UTC)}
	b := NewBudget(5, clk)
	if !b.TryReserve(5) || b.TryReserve(1) {
		t.Fatal("day one: limit must be enforced")
	}
	clk.now = clk.now.Add(2 * time.Hour) // 2026-09-07 01:00 UTC
	if !b.TryReserve(5) {
		t.Fatal("new UTC day: budget must reset")
	}
}
```

- [ ] **Step 7: Test laufen lassen** — `go test ./internal/adapters/omhttp/ -v` → FAIL (package fehlt)

- [ ] **Step 8: Budget implementieren**

`internal/adapters/omhttp/budget.go`:

```go
// Package omhttp provides the shared HTTP transport for every Open-Meteo call:
// a token-bucket rate limit, transparent retry honoring Retry-After, and a
// weighted daily call budget enforced only for batch-originated requests.
// Adapters receive it as a plain *http.Client, so no adapter imports this
// package (composition happens in internal/app).
package omhttp

import (
	"sync"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// Budget is a weighted daily call counter, resetting at the UTC day boundary.
type Budget struct {
	mu    sync.Mutex
	clock output.Clock
	limit int
	day   string
	spent int
}

// NewBudget builds a Budget with the given daily limit in weighted calls.
func NewBudget(limit int, clock output.Clock) *Budget {
	return &Budget{clock: clock, limit: limit}
}

// TryReserve atomically reserves weight from today's budget; false when the
// remaining budget is insufficient. The counter rolls over at UTC midnight.
func (b *Budget) TryReserve(weight int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	today := b.clock.Now().UTC().Format("2006-01-02")
	if today != b.day {
		b.day, b.spent = today, 0
	}
	if b.spent+weight > b.limit {
		return false
	}
	b.spent += weight
	return true
}
```

- [ ] **Step 9: Budget-Tests grün** — `go test ./internal/adapters/omhttp/ -v` → PASS

- [ ] **Step 10: Failing Tests für den Transport schreiben**

`internal/adapters/omhttp/omhttp_test.go`:

```go
package omhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/ports/output"
)

func newClient(t *testing.T, opts Options) *http.Client {
	t.Helper()
	return &http.Client{Transport: NewTransport(opts), Timeout: 5 * time.Second}
}

func TestTransport_RetriesOn429ThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 3}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if calls.Load() != 3 {
		t.Fatalf("upstream calls = %d, want 3", calls.Load())
	}
}

func TestTransport_ExhaustedRetriesReturnLastResponse(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 2}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	if calls.Load() != 3 { // initial + 2 retries
		t.Fatalf("upstream calls = %d, want 3", calls.Load())
	}
}

func TestTransport_NoRetryOn400(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 3}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1 (4xx must not retry)", calls.Load())
	}
}

func TestTransport_BudgetRejectsBatchOnly(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	budget := NewBudget(1, clk)
	client := newClient(t, Options{Budget: budget, Weight: 1})

	// Non-batch requests ignore the budget entirely.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if resp, err := client.Do(req); err != nil {
		t.Fatalf("non-batch request: %v", err)
	} else {
		resp.Body.Close()
	}

	// First batch request consumes the budget, second is rejected locally.
	bctx := output.WithBatchOrigin(req.Context())
	breq, _ := http.NewRequestWithContext(bctx, http.MethodGet, srv.URL, nil)
	if resp, err := client.Do(breq); err != nil {
		t.Fatalf("first batch request: %v", err)
	} else {
		resp.Body.Close()
	}
	before := calls.Load()
	breq2, _ := http.NewRequestWithContext(bctx, http.MethodGet, srv.URL, nil)
	_, err := client.Do(breq2)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	if calls.Load() != before {
		t.Fatal("budget rejection must not hit the upstream server")
	}
}

func TestTransport_RateLimiterPacesRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 1 token immediately (burst), then one per 50 ms.
	limiter := rate.NewLimiter(rate.Every(50*time.Millisecond), 1)
	client := newClient(t, Options{Limiter: limiter})
	start := time.Now()
	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("3 requests took %v, want >= 100ms (limiter must pace)", elapsed)
	}
}
```

- [ ] **Step 11: Tests laufen lassen** — `go test ./internal/adapters/omhttp/ -v` → FAIL (`NewTransport` undefined)

- [ ] **Step 12: Transport implementieren**

`internal/adapters/omhttp/omhttp.go`:

```go
package omhttp

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// ErrBudgetExhausted is returned (without any upstream call) when a
// batch-originated request finds today's weighted call budget spent.
var ErrBudgetExhausted = errors.New("open-meteo daily budget exhausted; retry tomorrow")

// Options configures the transport. Limiter and Budget are shared across all
// Open-Meteo adapters; Weight is the per-call cost this client charges.
type Options struct {
	Limiter       *rate.Limiter
	Budget        *Budget
	Weight        int
	RetryAttempts int
	Base          http.RoundTripper
}

// Transport rate-limits, retries (429/5xx/network, honoring Retry-After), and
// budget-guards Open-Meteo GET requests. Retries assume idempotent, body-less
// requests — all Open-Meteo calls are plain GETs.
type Transport struct {
	limiter  *rate.Limiter
	budget   *Budget
	weight   int
	attempts int
	base     http.RoundTripper
}

// NewTransport builds the transport; a nil Base falls back to
// http.DefaultTransport.
func NewTransport(opts Options) *Transport {
	base := opts.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{
		limiter:  opts.Limiter,
		budget:   opts.Budget,
		weight:   opts.Weight,
		attempts: opts.RetryAttempts,
		base:     base,
	}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.budget != nil && output.IsBatchOrigin(req.Context()) && !t.budget.TryReserve(t.weight) {
		return nil, ErrBudgetExhausted
	}
	for attempt := 0; ; attempt++ {
		if t.limiter != nil {
			if err := t.limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
		}
		resp, err := t.base.RoundTrip(req)
		if !shouldRetry(resp, err) || attempt >= t.attempts || req.Body != nil {
			return resp, err
		}
		wait := retryDelay(resp, attempt)
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
		}
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(wait):
		}
	}
}

// shouldRetry: network errors and 429/5xx are worth retrying; 4xx are not.
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

// retryDelay prefers the server's Retry-After (seconds) and falls back to
// exponential backoff starting at 2s.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return 2 * time.Second << attempt
}
```

- [ ] **Step 13: Alle Paket-Tests grün** — `go test ./internal/adapters/omhttp/ ./internal/ports/output/ -v` → PASS

- [ ] **Step 14: Verify + Commit**

```bash
make verify
git add go.mod go.sum internal/ports/output/batchorigin.go internal/ports/output/batchorigin_test.go internal/adapters/omhttp/
git commit -m "feat(omhttp): add shared rate-limited, retrying, budget-guarded Open-Meteo transport"
```

---

### Task 2: Config-Erweiterung (Batch + Drosselung)

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (existiert die Datei nicht, anlegen)

**Interfaces:**
- Produces: `cfg.Query.Batch.MaxPoints/MaxSyncPoints/Concurrency int`; `cfg.Providers.OpenMeteo.RatePerMinute/RetryAttempts/DailyBudget int`; `cfg.Providers.OpenMeteo.Weights.Weather/Aggregate/Bioclim int`.

- [ ] **Step 1: Failing Test für die neuen Defaults schreiben**

In `internal/config/config_test.go` (Package `config_test`; falls die Datei existiert, Test ergänzen und Stil übernehmen):

```go
package config_test

import (
	"testing"

	"github.com/jobrunner/tempus/internal/config"
)

func TestLoad_BatchAndThrottleDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Query.Batch.MaxPoints; got != 10000 {
		t.Errorf("query.batch.max_points = %d, want 10000", got)
	}
	if got := cfg.Query.Batch.MaxSyncPoints; got != 1000 {
		t.Errorf("query.batch.max_sync_points = %d, want 1000", got)
	}
	if got := cfg.Query.Batch.Concurrency; got != 4 {
		t.Errorf("query.batch.concurrency = %d, want 4", got)
	}
	om := cfg.Providers.OpenMeteo
	if om.RatePerMinute != 500 || om.RetryAttempts != 3 || om.DailyBudget != 8000 {
		t.Errorf("openmeteo throttle defaults = %+v, want 500/3/8000", om)
	}
	if om.Weights.Weather != 1 || om.Weights.Aggregate != 2 || om.Weights.Bioclim != 30 {
		t.Errorf("openmeteo weights = %+v, want 1/2/30", om.Weights)
	}
}
```

- [ ] **Step 2: Test laufen lassen** — `go test ./internal/config/ -run TestLoad_BatchAndThrottleDefaults -v` → FAIL

- [ ] **Step 3: Config-Structs und Defaults ergänzen**

In `internal/config/config.go`:

```go
// QueryConfig wird zu:
type QueryConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
	Batch   BatchConfig   `mapstructure:"batch"`
}

// BatchConfig bounds the batch endpoint: MaxPoints is the hard request cap,
// MaxSyncPoints the synchronous-mode cap (larger batches must stream NDJSON),
// Concurrency the batch worker-pool size.
type BatchConfig struct {
	MaxPoints     int `mapstructure:"max_points"`
	MaxSyncPoints int `mapstructure:"max_sync_points"`
	Concurrency   int `mapstructure:"concurrency"`
}

// OpenMeteoConfig erhält zusätzlich:
//	RatePerMinute int           `mapstructure:"rate_per_minute"`
//	RetryAttempts int           `mapstructure:"retry_attempts"`
//	DailyBudget   int           `mapstructure:"daily_budget"`
//	Weights       CallWeights   `mapstructure:"weights"`

// CallWeights are the estimated Open-Meteo cost weights per call type; long
// archive ranges count as multiple calls upstream, so bioclim's 30-year daily
// series weighs far more than a single-day weather call.
type CallWeights struct {
	Weather   int `mapstructure:"weather"`
	Aggregate int `mapstructure:"aggregate"`
	Bioclim   int `mapstructure:"bioclim"`
}
```

In `Defaults()` ergänzen:

```go
viper.SetDefault("query.batch.max_points", 10000)
viper.SetDefault("query.batch.max_sync_points", 1000)
viper.SetDefault("query.batch.concurrency", 4)
viper.SetDefault("providers.openmeteo.rate_per_minute", 500)
viper.SetDefault("providers.openmeteo.retry_attempts", 3)
viper.SetDefault("providers.openmeteo.daily_budget", 8000)
viper.SetDefault("providers.openmeteo.weights.weather", 1)
viper.SetDefault("providers.openmeteo.weights.aggregate", 2)
viper.SetDefault("providers.openmeteo.weights.bioclim", 30)
```

- [ ] **Step 4: Test grün** — `go test ./internal/config/ -v` → PASS

- [ ] **Step 5: Verify + Commit**

```bash
make verify
git add internal/config/
git commit -m "feat(config): add query.batch and open-meteo throttle/budget keys"
```

---

### Task 3: Drossel-Clients in die drei Adapter verdrahten

**Files:**
- Modify: `internal/app/providers.go`
- Modify: `internal/app/app.go` (nur falls Signatur von `buildRegistry` sich ändert — sie ändert sich nicht: Clients werden in `providers.go` gebaut)
- Test: `internal/app/providers_test.go` (anlegen, falls nicht vorhanden)

**Interfaces:**
- Consumes: `omhttp.NewTransport`, `omhttp.NewBudget`, `rate.NewLimiter`; die `HTTPClient *http.Client`-Options-Felder aller drei Adapter (openmeteo, aggregate, bioclim — alle vorhanden).
- Produces: `buildOpenMeteoClients(cfg *config.Config, clk output.Clock) omClients` mit `omClients{weather, aggregate, bioclim *http.Client}`; `buildRegistry` nutzt sie intern (Signatur unverändert: `buildRegistry(cfg, cache, clk)`).

- [ ] **Step 1: Failing Test schreiben**

`internal/app/providers_test.go`:

```go
package app

import (
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/clock"
	"github.com/jobrunner/tempus/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestBuildOpenMeteoClients_SharedLimiterDistinctTimeouts(t *testing.T) {
	cfg := testConfig(t)
	c := buildOpenMeteoClients(cfg, clock.System{})
	if c.weather == nil || c.aggregate == nil || c.bioclim == nil {
		t.Fatal("all three clients must be built")
	}
	if c.weather.Timeout != cfg.Providers.OpenMeteo.Timeout {
		t.Errorf("weather timeout = %v, want %v", c.weather.Timeout, cfg.Providers.OpenMeteo.Timeout)
	}
	if c.bioclim.Timeout != 60*time.Second {
		t.Errorf("bioclim timeout = %v, want 60s (large 30-year fetch)", c.bioclim.Timeout)
	}
	if c.weather.Transport == c.aggregate.Transport {
		t.Error("each client needs its own transport (distinct weights)")
	}
}
```

- [ ] **Step 2: Test laufen lassen** — `go test ./internal/app/ -run TestBuildOpenMeteoClients -v` → FAIL

- [ ] **Step 3: Clients bauen und injizieren**

In `internal/app/providers.go` (Imports ergänzen: `net/http`, `golang.org/x/time/rate`, `github.com/jobrunner/tempus/internal/adapters/omhttp`):

```go
// omClients are the per-adapter HTTP clients sharing one rate limiter and one
// daily budget. Each client carries its own cost weight and timeout.
type omClients struct {
	weather   *http.Client
	aggregate *http.Client
	bioclim   *http.Client
}

// buildOpenMeteoClients wires the shared throttling transport. The limiter and
// budget are shared across all Open-Meteo traffic; weights differ per call
// type because Open-Meteo weighs long archive ranges as multiple calls.
func buildOpenMeteoClients(cfg *config.Config, clk output.Clock) omClients {
	om := cfg.Providers.OpenMeteo
	// Burst 10 keeps a single interactive query (≤4 calls) latency-free while
	// the sustained rate stays under the free-tier minute limit.
	limiter := rate.NewLimiter(rate.Limit(float64(om.RatePerMinute)/60.0), 10)
	budget := omhttp.NewBudget(om.DailyBudget, clk)
	client := func(weight int, timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: omhttp.NewTransport(omhttp.Options{
				Limiter:       limiter,
				Budget:        budget,
				Weight:        weight,
				RetryAttempts: om.RetryAttempts,
			}),
		}
	}
	return omClients{
		weather:   client(om.Weights.Weather, om.Timeout),
		aggregate: client(om.Weights.Aggregate, om.Timeout),
		// The 30-year daily fetch is large; allow more time than a single-hour
		// call (matches the previous hardcoded bioclim timeout).
		bioclim: client(om.Weights.Bioclim, 60*time.Second),
	}
}
```

In `buildRegistry` die Clients bauen und in die drei `Options` einsetzen:

```go
func buildRegistry(cfg *config.Config, cache output.Cache, clk output.Clock) *application.Registry {
	registry := application.NewRegistry()
	clients := buildOpenMeteoClients(cfg, clk)

	if cfg.Providers.OpenMeteo.Enabled {
		registry.Register(cachedWeatherProvider(cfg, cache, clk, clients.weather))
	}
	// … astronomy unverändert …
	// aggregate: Options um HTTPClient: clients.aggregate ergänzen
	// bioclim:   Options um HTTPClient: clients.bioclim ergänzen (Timeout-Feld
	//            bleibt, wird durch den gesetzten Client aber nicht mehr genutzt)
	…
}

// cachedWeatherProvider erhält den Client als Parameter:
func cachedWeatherProvider(cfg *config.Config, cache output.Cache, clk output.Clock, client *http.Client) output.FeatureProvider {
	om := openmeteo.New(openmeteo.Options{ …wie bisher…, HTTPClient: client })
	…Decorator unverändert…
}
```

- [ ] **Step 4: Tests grün** — `go test ./internal/app/ -v` → PASS

- [ ] **Step 5: Verify + Commit**

```bash
make verify
git add internal/app/
git commit -m "feat(app): route all Open-Meteo adapters through the shared throttling transport"
```

---

### Task 4: aggregate cachbar machen (`KeyParams` im Cache-Key)

**Files:**
- Modify: `internal/application/caching.go` (`CachingOptions` + `CacheKey`)
- Modify: `internal/app/providers.go` (aggregate mit Decorator umwickeln)
- Test: `internal/application/caching_test.go` (ergänzen; existiert vermutlich — Stil übernehmen)

**Interfaces:**
- Consumes: `application.CachingOptions`, `application.CacheKey` (bestehende Aufrufer anpassen).
- Produces: `CachingOptions.KeyParams func(domain.QueryRequest) string` (nil ⇒ ""); `CacheKey(providerID, version string, req domain.QueryRequest, precision int, params string) string` — bei `params == ""` byte-identisch zum bisherigen Key (kein Cache-Invalidieren beim Deploy).

- [ ] **Step 1: Failing Golden-Key-Test schreiben** (bestehende `CacheKey`-Aufrufer in Tests bekommen den neuen Parameter `""`)

In `internal/application/caching_test.go` ergänzen:

```go
func TestCacheKey_EmptyParamsKeepLegacyFormat(t *testing.T) {
	req := domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 49.79345, Lon: 9.95341},
		Instant:    time.Date(2025, 6, 3, 14, 0, 0, 0, time.UTC),
	}
	// The legacy raw format, reproduced literally: params must not change it.
	raw := fmt.Sprintf("%s|%s|%.*f|%.*f|%s",
		"open-meteo", "1", 2, 49.79, 2, 9.95, req.Instant.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])
	if got := application.CacheKey("open-meteo", "1", req, 2, ""); got != want {
		t.Errorf("CacheKey with empty params = %s, want legacy %s", got, want)
	}
	if same := application.CacheKey("open-meteo", "1", req, 2, "gdd=7.00"); same == want {
		t.Error("non-empty params must produce a different key")
	}
}

func TestCachingProvider_KeyParamsSeparateEntries(t *testing.T) {
	// fakeProvider/fakeCache aus den bestehenden Tests wiederverwenden.
	// Zwei Fetches mit unterschiedlichem GDDBaseCelsius → 2 inner-Fetches;
	// dritter Fetch mit gleichem GDDBaseCelsius wie der erste → Cache-Hit
	// (inner bleibt bei 2).
	base7, base9 := 7.0, 9.0
	inner := &fakeProvider{}
	cp := application.NewCachingProvider(inner, newFakeCache(), fixedClock{}, application.CachingOptions{
		Version: "1", MatureTTL: time.Hour, ImmatureTTL: time.Hour, LatLonPrecision: 2,
		KeyParams: func(req domain.QueryRequest) string {
			if req.GDDBaseCelsius == nil {
				return ""
			}
			return fmt.Sprintf("gdd=%.2f", *req.GDDBaseCelsius)
		},
	})
	req := domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 1, Lon: 2}, Instant: time.Unix(0, 0).UTC()}
	req.GDDBaseCelsius = &base7
	_, _ = cp.Fetch(context.Background(), req)
	req.GDDBaseCelsius = &base9
	_, _ = cp.Fetch(context.Background(), req)
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2 (distinct gddBase must miss)", inner.calls)
	}
	req.GDDBaseCelsius = &base7
	res, _ := cp.Fetch(context.Background(), req)
	if inner.calls != 2 || !res.Cached {
		t.Fatalf("inner calls = %d, cached = %v; want 2, true", inner.calls, res.Cached)
	}
}
```

(Die Fakes `fakeProvider`/`newFakeCache`/`fixedClock` aus der bestehenden Testdatei nutzen; falls sie dort anders heißen, deren Namen übernehmen.)

- [ ] **Step 2: Tests laufen lassen** — `go test ./internal/application/ -v` → FAIL (Signatur)

- [ ] **Step 3: `CacheKey` + Decorator anpassen**

In `internal/application/caching.go`:

```go
// CachingOptions erhält:
//	// KeyParams optionally contributes provider-specific request parameters to
//	// the cache key (e.g. the aggregate provider's gddBase). Empty/nil keeps
//	// the legacy key format, so existing cache entries stay valid.
//	KeyParams func(domain.QueryRequest) string

// In Fetch:
	params := ""
	if c.opts.KeyParams != nil {
		params = c.opts.KeyParams(req)
	}
	key := CacheKey(c.inner.ID(), c.opts.Version, req, c.opts.LatLonPrecision, params)

// CacheKey:
func CacheKey(providerID, version string, req domain.QueryRequest, precision int, params string) string {
	lat := roundTo(req.Coordinate.Lat, precision)
	lon := roundTo(req.Coordinate.Lon, precision)
	raw := fmt.Sprintf("%s|%s|%.*f|%.*f|%s",
		providerID, version, precision, lat, precision, lon, req.Instant.UTC().Format(time.RFC3339))
	if params != "" {
		raw += "|" + params
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: aggregate in `providers.go` umwickeln** — den bisherigen „ohne Decorator, weil gddBase"-Kommentar ersetzen:

```go
	// Weather aggregates: cached like the weather provider; the per-request
	// gddBase override is part of the cache key via KeyParams.
	if cfg.Providers.Aggregate.Enabled && cfg.Providers.OpenMeteo.Enabled {
		agg := aggregate.New(aggregate.Options{ …wie bisher…, HTTPClient: clients.aggregate })
		registry.Register(application.NewCachingProvider(agg, cache, clk, application.CachingOptions{
			Version:         "1",
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			MatureTTL:       365 * 24 * time.Hour,
			ImmatureTTL:     time.Hour,
			LatLonPrecision: 2,
			KeyParams: func(req domain.QueryRequest) string {
				if req.GDDBaseCelsius == nil {
					return ""
				}
				return fmt.Sprintf("gdd=%.2f", *req.GDDBaseCelsius)
			},
		}))
	}
```

(Import `fmt` und `domain` in providers.go ergänzen.)

- [ ] **Step 5: Tests grün** — `go test ./internal/application/ ./internal/app/ -v` → PASS

- [ ] **Step 6: Verify + Commit**

```bash
make verify
git add internal/application/ internal/app/
git commit -m "feat(cache): include gddBase via KeyParams and cache the aggregate provider"
```

---

### Task 5: `BatchService` (Dedup, Worker-Pool, geordnete Emission)

**Files:**
- Modify: `internal/ports/input/ports.go` (Batch-Typen + Port)
- Create: `internal/application/batch.go`
- Test: `internal/application/batch_test.go`

**Interfaces:**
- Consumes: `input.FeatureService` (`Query(ctx, domain.QueryRequest) (domain.QueryResult, error)`), `output.WithBatchOrigin`.
- Produces (für Task 6):
  - `input.BatchPoint{ID string; Req *domain.QueryRequest; ParseError string}`
  - `input.BatchItem{ID string; *domain.QueryResult; Error *BatchItemError}` (JSON: `id` + eingebettete `query`/`features`/`providers`, oder `id`+`error`)
  - `input.BatchItemError{Message string}` (JSON: `message`)
  - `input.BatchService` Interface: `QueryBatch(ctx context.Context, points []BatchPoint, emit func(BatchItem) error) error`
  - `application.NewBatchService(features input.FeatureService, concurrency, latLonPrecision int) *BatchService`

- [ ] **Step 1: Port-Typen ergänzen** (kein eigener Test — reine Typdeklarationen, von den Service-Tests mitgeprüft)

In `internal/ports/input/ports.go`:

```go
// BatchPoint is one resolved batch input: either a valid QueryRequest or the
// parse error that keeps it from the query path. ID is the opaque echo id.
type BatchPoint struct {
	ID         string
	Req        *domain.QueryRequest
	ParseError string
}

// BatchItemError is the per-item error object for unprocessable points.
type BatchItemError struct {
	Message string `json:"message"`
}

// BatchItem is one batch result: the single-query envelope plus the echo id,
// or (for unparsable points) just id + error. The embedded nil pointer keeps
// query/features/providers absent on error items.
type BatchItem struct {
	ID string `json:"id"`
	*domain.QueryResult
	Error *BatchItemError `json:"error,omitempty"`
}

// BatchService processes batch points in input order; emit is called once per
// point. A non-nil emit error (e.g. a broken stream) aborts the batch.
type BatchService interface {
	QueryBatch(ctx context.Context, points []BatchPoint, emit func(BatchItem) error) error
}
```

- [ ] **Step 2: Failing Tests für den Service schreiben**

`internal/application/batch_test.go`:

```go
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
		max := f.maxInfl.Load()
		if cur <= max || f.maxInfl.CompareAndSwap(max, cur) {
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
	if max := f.maxInfl.Load(); max > 2 {
		t.Errorf("max in-flight = %d, want <= 2", max)
	}
	if !f.sawBatch.Load() {
		t.Error("Query contexts must be batch-origin-marked")
	}
}

func TestBatchService_EmitErrorAborts(t *testing.T) {
	f := &countingFeatures{}
	svc := application.NewBatchService(f, 1, 2)
	wantErr := context.Canceled // beliebiger Sentinel
	err := svc.QueryBatch(context.Background(),
		[]input.BatchPoint{point("a", 1, 2), point("b", 3, 4)},
		func(input.BatchItem) error { return wantErr })
	if err != wantErr {
		t.Fatalf("err = %v, want emit error propagated", err)
	}
}
```

- [ ] **Step 3: Tests laufen lassen** — `go test ./internal/application/ -run TestBatchService -v` → FAIL

- [ ] **Step 4: Service implementieren**

`internal/application/batch.go`:

```go
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

	groups := map[string]*group{}
	var order []*group
	assign := make([]*group, len(points))
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

	sem := make(chan struct{}, s.concurrency)
	go func() {
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
	}()

	for i, p := range points {
		if p.Req == nil {
			if err := emit(input.BatchItem{ID: p.ID, Error: &input.BatchItemError{Message: p.ParseError}}); err != nil {
				return err
			}
			continue
		}
		g := assign[i]
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
		if err := emit(input.BatchItem{ID: p.ID, QueryResult: &res}); err != nil {
			return err
		}
	}
	return nil
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
```

- [ ] **Step 5: Tests grün** — `go test ./internal/application/ -v` → PASS

- [ ] **Step 6: Verify + Commit**

```bash
make verify
git add internal/ports/input/ internal/application/
git commit -m "feat(application): add BatchService with dedup, bounded workers, ordered emission"
```

---

### Task 6: HTTP-Endpoint `POST /api/v1/query/batch` + OpenAPI (beide Specs)

Route, Handler, NDJSON-Rendering und die Spec-Einträge gehören in EINEN Task, weil `TestRoutesMatchOpenAPISpec` sonst zwischen den Commits rot wäre.

**Files:**
- Create: `internal/adapters/http/batch.go`
- Create: `internal/adapters/http/batch_render.go`
- Modify: `internal/adapters/http/server.go` (NewServer-Signatur, Route, Options)
- Modify: `internal/adapters/http/server_test.go` (Stub ergänzen), `internal/adapters/http/contract_test.go:234` (NewServer-Aufruf)
- Modify: `internal/app/app.go` (BatchService bauen + durchreichen)
- Modify: `internal/adapters/http/openapi.yaml` + `cp` nach `api/openapi/openapi.yaml`
- Test: `internal/adapters/http/batch_test.go`

**Interfaces:**
- Consumes: `input.BatchService.QueryBatch(ctx, []input.BatchPoint, emit func(input.BatchItem) error) error`; `domain.ParseQueryRequest(lat, lon, datetime, gddBase string, providers []string)`.
- Produces: `NewServer(addr string, features input.FeatureService, batch input.BatchService, providers input.ProviderLister, health input.HealthChecker, clock output.Clock, logger *slog.Logger, opts Options)`; `Options.Batch BatchLimits{MaxPoints, MaxSyncPoints int}` (0 ⇒ Defaults 10000/1000).

- [ ] **Step 1: Failing Handler-Tests schreiben**

`internal/adapters/http/batch_test.go` (Package `httpapi`, Stubs aus `server_test.go` wiederverwenden; `stubBatch` neu — er reicht Punkte an eine echte, kleine Fake-Emission durch):

```go
package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// echoBatch is a minimal input.BatchService: valid points become ok items
// echoing the request, invalid ones become error items.
type echoBatch struct{}

func (echoBatch) QueryBatch(_ context.Context, points []input.BatchPoint, emit func(input.BatchItem) error) error {
	for _, p := range points {
		if p.Req == nil {
			if err := emit(input.BatchItem{ID: p.ID, Error: &input.BatchItemError{Message: p.ParseError}}); err != nil {
				return err
			}
			continue
		}
		res := domain.QueryResult{
			Query:     domain.QueryEcho{Coordinate: p.Req.Coordinate, Datetime: p.Req.Instant.UTC().Format("2006-01-02T15:04:05Z07:00")},
			Features:  []domain.Feature{},
			Providers: []domain.ProviderStatus{},
		}
		if err := emit(input.BatchItem{ID: p.ID, QueryResult: &res}); err != nil {
			return err
		}
	}
	return nil
}
```

Helper und Testfälle (Stubs `stubFeatures{}`, `stubProviders{}`, `stubHealth{}`, `fixedClock{}` aus `server_test.go` wiederverwenden):

```go
func postBatch(t *testing.T, srv *Server, body string, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	return rr
}

func TestHandleQueryBatch_SyncEnvelope(t *testing.T) {
	// 2 Punkte (einer mit id, einer ohne) → 200, results in Reihenfolge,
	// fehlende id = Index als String, total == 2, processing_time_ms vorhanden.
	body := `{"points":[
		{"id":"x","lat":49.79,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"},
		{"lat":47.42,"lon":10.98,"datetime":"2025-06-03T14:00:00Z"}]}`
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), body, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var env struct {
		Results          []json.RawMessage `json:"results"`
		Total            int               `json:"total"`
		ProcessingTimeMS int64             `json:"processing_time_ms"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Total != 2 || len(env.Results) != 2 {
		t.Fatalf("total/results = %d/%d, want 2/2", env.Total, len(env.Results))
	}
	var first struct{ ID string `json:"id"` }
	_ = json.Unmarshal(env.Results[0], &first)
	var second struct{ ID string `json:"id"` }
	_ = json.Unmarshal(env.Results[1], &second)
	if first.ID != "x" || second.ID != "1" {
		t.Errorf("ids = %q,%q; want x,1 (index fallback)", first.ID, second.ID)
	}
}

// points builds a JSON body with n valid points.
func batchBody(n int) string {
	var b strings.Builder
	b.WriteString(`{"points":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"p%d","lat":49.79,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"}`, i)
	}
	b.WriteString(`]}`)
	return b.String()
}

func TestHandleQueryBatch_InvalidPointBecomesErrorItem(t *testing.T) {
	body := `{"points":[{"id":"bad","lat":999,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"}]}`
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), body, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (per-item errors never abort)", rr.Code)
	}
	var env struct {
		Results []struct {
			ID    string          `json:"id"`
			Error *struct{ Message string `json:"message"` } `json:"error"`
			Query json.RawMessage `json:"query"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	it := env.Results[0]
	if it.ID != "bad" || it.Error == nil || !strings.Contains(it.Error.Message, "lat") {
		t.Errorf("item = %+v, want id=bad with lat parse error", it)
	}
	if len(it.Query) != 0 {
		t.Error("error item must not carry a query echo")
	}
}

func TestHandleQueryBatch_EmptyPointsIs400(t *testing.T) {
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), `{"points":[]}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleQueryBatch_TooManyPointsIs400(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 5, MaxSyncPoints: 5})
	rr := postBatch(t, srv, batchBody(6), "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleQueryBatch_SyncCapIs413WithNDJSONHint(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 10, MaxSyncPoints: 2})
	rr := postBatch(t, srv, batchBody(3), "")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "application/x-ndjson") {
		t.Errorf("413 message must hint at NDJSON streaming, got %s", rr.Body.String())
	}
}

func TestHandleQueryBatch_NDJSONStreamsAllPoints(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 10, MaxSyncPoints: 2})
	rr := postBatch(t, srv, batchBody(3), "application/x-ndjson")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (stream mode bypasses the sync cap)", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", ct)
	}
	var ids []string
	sc := bufio.NewScanner(bytes.NewReader(rr.Body.Bytes()))
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var it struct{ ID string `json:"id"` }
		if err := json.Unmarshal(sc.Bytes(), &it); err != nil {
			t.Fatalf("line not parseable: %v", err)
		}
		ids = append(ids, it.ID)
	}
	if len(ids) != 3 || ids[0] != "p0" || ids[2] != "p2" {
		t.Fatalf("ids = %v, want [p0 p1 p2] in input order", ids)
	}
}

func TestHandleQueryBatch_BodyTooLargeIs413(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 5, MaxSyncPoints: 5})
	// Cap = 5*512 B + 64 KiB; ~80 KiB whitespace padding inside the JSON
	// blows it while staying syntactically pending.
	body := `{"points":[` + strings.Repeat(" ", 80*1024) + `]}`
	rr := postBatch(t, srv, body, "")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}
```

Helper `newBatchTestServer` in dieselbe Datei:

```go
func newBatchTestServer(t *testing.T, limits BatchLimits) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, echoBatch{}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{Batch: limits})
}
```

- [ ] **Step 2: Tests laufen lassen** — `go test ./internal/adapters/http/ -run TestHandleQueryBatch -v` → FAIL (compile)

- [ ] **Step 3: Request-Parsing + Handler implementieren**

`internal/adapters/http/batch.go`:

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// BatchLimits bounds the batch endpoint; zero values fall back to the
// defaults below (wired from config in the composition root).
type BatchLimits struct {
	MaxPoints     int
	MaxSyncPoints int
}

const (
	defaultBatchMaxPoints     = 10000
	defaultBatchMaxSyncPoints = 1000
	// batchBytesPerPoint sizes the request-body cap: points are small JSON
	// objects; 512 B each plus fixed headroom is generous.
	batchBytesPerPoint = 512
	batchBodyHeadroom  = 64 * 1024
)

func (l BatchLimits) maxPoints() int {
	if l.MaxPoints > 0 {
		return l.MaxPoints
	}
	return defaultBatchMaxPoints
}

func (l BatchLimits) maxSyncPoints() int {
	if l.MaxSyncPoints > 0 {
		return l.MaxSyncPoints
	}
	return defaultBatchMaxSyncPoints
}

// batchRequest mirrors the single-query parameters plus the points array.
type batchRequest struct {
	Providers []string     `json:"providers"`
	GDDBase   *float64     `json:"gddBase"`
	RefPeriod string       `json:"refPeriod"`
	Points    []batchPoint `json:"points"`
}

type batchPoint struct {
	ID       string   `json:"id"`
	Lat      *float64 `json:"lat"`
	Lon      *float64 `json:"lon"`
	Datetime string   `json:"datetime"`
}

func (s *Server) handleQueryBatch(w http.ResponseWriter, r *http.Request) {
	maxPts := s.batchLimits.maxPoints()
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxPts)*batchBytesPerPoint+batchBodyHeadroom)

	var breq batchRequest
	if err := json.NewDecoder(r.Body).Decode(&breq); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(breq.Points) == 0 {
		s.writeError(w, http.StatusBadRequest, "points must not be empty")
		return
	}
	if len(breq.Points) > maxPts {
		s.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("too many points: %d > %d", len(breq.Points), maxPts))
		return
	}
	stream := prefersNDJSON(r)
	if !stream && len(breq.Points) > s.batchLimits.maxSyncPoints() {
		s.writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf(
			"more than %d points require streaming; retry with Accept: application/x-ndjson",
			s.batchLimits.maxSyncPoints()))
		return
	}

	points := resolveBatchPoints(breq)
	if stream {
		s.streamBatchNDJSON(w, r, points)
		return
	}
	s.writeBatchSync(w, r, points)
}

// resolveBatchPoints validates every point through the exact single-query
// parser; invalid points become error items and never reach the query path.
func resolveBatchPoints(breq batchRequest) []input.BatchPoint {
	gdd := ""
	if breq.GDDBase != nil {
		gdd = strconv.FormatFloat(*breq.GDDBase, 'f', -1, 64)
	}
	out := make([]input.BatchPoint, 0, len(breq.Points))
	for i, p := range breq.Points {
		id := p.ID
		if id == "" {
			id = strconv.Itoa(i)
		}
		lat, lon := "", ""
		if p.Lat != nil {
			lat = strconv.FormatFloat(*p.Lat, 'f', -1, 64)
		}
		if p.Lon != nil {
			lon = strconv.FormatFloat(*p.Lon, 'f', -1, 64)
		}
		req, err := domain.ParseQueryRequest(lat, lon, p.Datetime, gdd, breq.Providers)
		if err != nil {
			out = append(out, input.BatchPoint{ID: id, ParseError: err.Error()})
			continue
		}
		req.RefPeriod = breq.RefPeriod
		out = append(out, input.BatchPoint{ID: id, Req: &req})
	}
	return out
}

// prefersNDJSON reports whether the client asked for the NDJSON stream.
func prefersNDJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/x-ndjson")
}
```

- [ ] **Step 4: Rendering implementieren**

`internal/adapters/http/batch_render.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jobrunner/tempus/internal/ports/input"
)

// batchEnvelope is the synchronous batch response (ortus-conformant shape).
type batchEnvelope struct {
	Results          []input.BatchItem `json:"results"`
	Total            int               `json:"total"`
	ProcessingTimeMS int64             `json:"processing_time_ms"`
}

func (s *Server) writeBatchSync(w http.ResponseWriter, r *http.Request, points []input.BatchPoint) {
	start := s.clock.Now()
	results := make([]input.BatchItem, 0, len(points))
	err := s.batch.QueryBatch(r.Context(), points, func(it input.BatchItem) error {
		results = append(results, it)
		return nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("batch query canceled by client")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "batch query failed")
		return
	}
	s.writeJSON(w, http.StatusOK, batchEnvelope{
		Results:          results,
		Total:            len(results),
		ProcessingTimeMS: s.clock.Now().Sub(start).Milliseconds(),
	})
}

// streamBatchNDJSON writes one result item per line in input order, flushing
// per line so clients can render progress. There is no trailing envelope; a
// client detects a broken stream by comparing line count to points sent.
func (s *Server) streamBatchNDJSON(w http.ResponseWriter, r *http.Request, points []input.BatchPoint) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	enc := json.NewEncoder(w)
	err := s.batch.QueryBatch(r.Context(), points, func(it input.BatchItem) error {
		if err := enc.Encode(it); err != nil { // Encode appends the newline
			return err
		}
		return rc.Flush()
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		// Headers are sent; all we can do is log and abort the stream.
		s.logger.Warn("batch stream aborted", "error", err)
	}
}
```

- [ ] **Step 5: Server verdrahten**

In `internal/adapters/http/server.go`:
- Feld `batch input.BatchService` + `batchLimits BatchLimits` im `Server`-Struct.
- `Options` um `Batch BatchLimits` erweitern.
- `NewServer(addr string, features input.FeatureService, batch input.BatchService, providers input.ProviderLister, health input.HealthChecker, clock output.Clock, logger *slog.Logger, opts Options)` — Zuweisung `batch: batch, batchLimits: opts.Batch`.
- Route registrieren: `api.HandleFunc("/query/batch", s.handleQueryBatch).Methods(http.MethodPost)`.

In `internal/adapters/http/server_test.go` und `contract_test.go:234`: `NewServer`-Aufrufe um einen Stub erweitern (`echoBatch{}` aus batch_test.go bzw. ein no-op `stubBatchService{}` in server_test.go — EIN gemeinsamer Stub in server_test.go genügt, batch_test.go nutzt ihn mit).

In `internal/app/app.go`:

```go
	batch := application.NewBatchService(features, cfg.Query.Batch.Concurrency, 2)
	a.server = httpapi.NewServer(addr, features, batch, registry, readyAlways{}, clk, logger, serverOpts)
```
und in `wireObservability`/Options-Bau: `serverOpts.Batch = httpapi.BatchLimits{MaxPoints: cfg.Query.Batch.MaxPoints, MaxSyncPoints: cfg.Query.Batch.MaxSyncPoints}` (dort setzen, wo `serverOpts` befüllt wird).

- [ ] **Step 6: Handler-Tests laufen lassen** — `go test ./internal/adapters/http/ -run TestHandleQueryBatch -v` → PASS; `TestRoutesMatchOpenAPISpec` → FAIL (Route undokumentiert) — genau das treibt Step 7.

- [ ] **Step 7: OpenAPI ergänzen** (`internal/adapters/http/openapi.yaml`)

Unter `paths` (Stil der Datei übernehmen, kompakte Flow-Syntax wo vorhanden):

```yaml
  /query/batch:
    post:
      summary: Batch feature query for many coordinate+time points
      description: >
        Processes up to query.batch.max_points points through the same
        provider pipeline as GET /query. Responses stream as NDJSON (one
        result item per line, input order) when the client sends
        Accept: application/x-ndjson; otherwise a synchronous JSON envelope
        is returned, capped at query.batch.max_sync_points points (413 above
        that, with a hint to retry streaming). Invalid points become per-item
        error objects and never abort the batch. Batch traffic is
        rate-limited and budget-guarded against the Open-Meteo free tier;
        exhausted budget surfaces as retryable per-provider errors.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/BatchQueryRequest"
      responses:
        "200":
          description: Batch results (sync envelope, or NDJSON stream of items)
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/BatchQueryResponse"
            application/x-ndjson:
              schema:
                $ref: "#/components/schemas/BatchQueryResultItem"
        "400":
          description: Invalid request (empty points, too many points, bad JSON)
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "413":
          description: Body too large, or too many points for synchronous mode
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
```

Unter `components/schemas` (JEDES Objekt mit `type: object`):

```yaml
    BatchQueryPoint:
      type: object
      required: [lat, lon, datetime]
      properties:
        id:
          type: string
          description: Opaque echo id; defaults to the 0-based input index as string.
        lat:
          type: number
        lon:
          type: number
        datetime:
          type: string
          description: Same formats as GET /query's datetime parameter.
    BatchQueryRequest:
      type: object
      required: [points]
      properties:
        providers:
          type: array
          items:
            type: string
          description: Optional provider-id filter applied to every point.
        gddBase:
          type: number
          description: Optional extra GDD base (°C, [-50,50]) applied to every point; base 5 and 10 are always included.
        refPeriod:
          type: string
          description: Optional bioclim reference period 'YYYY-YYYY' applied to every point.
        points:
          type: array
          items:
            $ref: "#/components/schemas/BatchQueryPoint"
    BatchQueryResultItem:
      description: >
        One result per input point, in input order: the single-query envelope
        plus id — or, for unparsable points, only id + error.
      allOf:
        - $ref: "#/components/schemas/QueryResult"
        - type: object
          required: [id]
          properties:
            id:
              type: string
            error:
              type: object
              properties:
                message:
                  type: string
    BatchQueryResponse:
      type: object
      required: [results, total, processing_time_ms]
      properties:
        results:
          type: array
          items:
            $ref: "#/components/schemas/BatchQueryResultItem"
        total:
          type: integer
        processing_time_ms:
          type: integer
```

(Voraussetzung: es existiert ein `QueryResult`-Schema — laut `/query`-Pfad ja. Falls dessen `required` `query`/`features`/`providers` erzwingt, stattdessen im `allOf` NICHT QueryResult referenzieren, sondern die drei Properties optional inline dokumentieren — prüfen und die Variante wählen, die Fehler-Items nicht invalide macht.)

Dann Spiegel aktualisieren:

```bash
cp internal/adapters/http/openapi.yaml api/openapi/openapi.yaml
```

- [ ] **Step 8: Alle HTTP-Tests grün** — `go test ./internal/adapters/http/ ./internal/app/ -v` → PASS (inkl. Contract- und Schema-Tests)

- [ ] **Step 9: Verify + Commit**

```bash
make verify
git add internal/adapters/http/ internal/app/ api/openapi/
git commit -m "feat(http): add POST /api/v1/query/batch with sync envelope and NDJSON streaming"
```

---

### Task 7: Frontend-Batch-Tab

**Files:**
- Modify: `internal/adapters/http/index.html`
- Test: `internal/adapters/http/frontend_test.go` (anlegen: Smoke-Assertions auf die ausgelieferte Seite)

**Interfaces:**
- Consumes: `POST /api/v1/query/batch` mit `Accept: application/x-ndjson` (Task 6).

- [ ] **Step 1: Failing Smoke-Test schreiben**

`internal/adapters/http/frontend_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFrontend_ContainsBatchTab pins the batch UI's load-bearing element ids;
// renaming them breaks the tab wiring silently, so the smoke test names them.
func TestFrontend_ContainsBatchTab(t *testing.T) {
	srv := newContractTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	body := rr.Body.String()
	for _, want := range []string{
		`id="tabSingle"`, `id="tabBatch"`, `id="batchPanel"`,
		`id="batchInput"`, `id="batchProgress"`, `id="batchRunBtn"`,
		`id="batchAbortBtn"`, `id="batchExportBtn"`, `id="batchTable"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("frontend page is missing %s", want)
		}
	}
}
```

- [ ] **Step 2: Test laufen lassen** — `go test ./internal/adapters/http/ -run TestFrontend_ContainsBatchTab -v` → FAIL

- [ ] **Step 3: Tab-Navigation + Batch-Panel in `index.html` einbauen**

Direkt nach `</header>` (vor der bestehenden Query-`<section>`) eine Tab-Leiste; die bestehende Einzelabfrage-Sektion + `#results` in einen Container `id="singlePanel"` wrappen:

```html
<nav class="tabs" role="tablist">
    <button type="button" class="tab active" id="tabSingle" role="tab" aria-selected="true" aria-controls="singlePanel">Einzelabfrage</button>
    <button type="button" class="tab" id="tabBatch" role="tab" aria-selected="false" aria-controls="batchPanel">Batch</button>
</nav>
```

Batch-Panel (nach `#singlePanel`, initial versteckt):

```html
<section id="batchPanel" role="tabpanel" hidden>
    <form id="batchForm">
        <label for="batchInput">Punkte — eine Zeile pro Punkt: <code>[id;] lat; lon; datetime</code>
            (Trennung durch <code>;</code>, <code>,</code> oder Tab; datetime RFC3339 oder
            <code>YYYY-MM-DDTHH:MM</code>, ohne Offset = UTC)</label>
        <textarea id="batchInput" rows="10" spellcheck="false"
            placeholder="fund-17; 49.79; 9.95; 2025-06-03T14:00&#10;47.42; 10.98; 2024-08-14T09:30"></textarea>
        <div class="form-row">
            <label for="batchProviders">Provider (kommagetrennt, leer = alle)</label>
            <input type="text" id="batchProviders" placeholder="open-meteo,aggregate,sun,moon,bioclim">
            <label for="batchGddBase">gddBase (optional, °C)</label>
            <input type="text" id="batchGddBase" inputmode="decimal" placeholder="z.B. 7">
            <label for="batchRefPeriod">refPeriod (optional)</label>
            <input type="text" id="batchRefPeriod" placeholder="z.B. 1991-2020">
        </div>
        <div class="form-actions">
            <button type="submit" class="btn" id="batchRunBtn">Batch starten</button>
            <button type="button" class="btn btn-secondary" id="batchAbortBtn" disabled>Abbrechen</button>
            <button type="button" class="btn btn-secondary" id="batchExportBtn" disabled>JSON exportieren</button>
        </div>
        <small>Große Batches können bei kaltem Cache mehrere Minuten laufen — Tab geöffnet lassen.
            Ein erneuter Start nach Abbruch ist dank Cache günstig.</small>
    </form>
    <div class="error-box" id="batchErrorBox" role="alert"></div>
    <div id="batchProgressWrap" hidden>
        <progress id="batchProgress" max="1" value="0"></progress>
        <span id="batchProgressText" aria-live="polite"></span>
    </div>
    <div class="table-wrap">
        <table id="batchTable" hidden>
            <thead><tr><th>id</th><th>lat</th><th>lon</th><th>datetime</th><th>Status</th></tr></thead>
            <tbody id="batchTableBody"></tbody>
        </table>
    </div>
</section>
```

CSS im bestehenden `<style>`-Block ergänzen (Stil/Variablen der Seite übernehmen): `.tabs` (Flex-Leiste), `.tab`/`.tab.active`, `#batchInput { width:100%; font-family:monospace; }`, `.table-wrap { overflow-x:auto; }`, Status-Badges `.st-ok`/`.st-err` (grün/rot analog vorhandener Farben).

- [ ] **Step 4: Batch-JS im bestehenden `<script>`-Block ergänzen**

```javascript
// --- Batch tab -------------------------------------------------------------
(function initBatch() {
    const tabSingle = document.getElementById('tabSingle');
    const tabBatch = document.getElementById('tabBatch');
    const singlePanel = document.getElementById('singlePanel');
    const batchPanel = document.getElementById('batchPanel');
    function selectTab(batch) {
        singlePanel.hidden = batch;
        batchPanel.hidden = !batch;
        tabSingle.classList.toggle('active', !batch);
        tabBatch.classList.toggle('active', batch);
        tabSingle.setAttribute('aria-selected', String(!batch));
        tabBatch.setAttribute('aria-selected', String(batch));
    }
    tabSingle.addEventListener('click', () => selectTab(false));
    tabBatch.addEventListener('click', () => selectTab(true));

    const form = document.getElementById('batchForm');
    const inputEl = document.getElementById('batchInput');
    const runBtn = document.getElementById('batchRunBtn');
    const abortBtn = document.getElementById('batchAbortBtn');
    const exportBtn = document.getElementById('batchExportBtn');
    const errorBox = document.getElementById('batchErrorBox');
    const progWrap = document.getElementById('batchProgressWrap');
    const prog = document.getElementById('batchProgress');
    const progText = document.getElementById('batchProgressText');
    const table = document.getElementById('batchTable');
    const tbody = document.getElementById('batchTableBody');

    let controller = null;
    let collected = [];
    let runStart = 0;

    // parseBatchLines: "[id sep] lat sep lon sep datetime", sep = ; , or tab.
    // Comma only splits when the line has no ; or tab (comma decimals then
    // cannot be used — the placeholder shows dot decimals).
    function parseBatchLines(text) {
        const points = [];
        const errors = [];
        text.split('\n').map(l => l.trim()).filter(l => l && !l.startsWith('#')).forEach((line, i) => {
            const sep = line.includes(';') ? ';' : (line.includes('\t') ? '\t' : ',');
            const parts = line.split(sep).map(s => s.trim()).filter(s => s !== '');
            let id = '';
            let rest = parts;
            if (parts.length === 4) { id = parts[0]; rest = parts.slice(1); }
            if (rest.length !== 3) { errors.push('Zeile ' + (i + 1) + ': erwartet [id' + sep + '] lat' + sep + ' lon' + sep + ' datetime'); return; }
            const lat = Number(rest[0].replace(',', '.'));
            const lon = Number(rest[1].replace(',', '.'));
            if (!isFinite(lat) || !isFinite(lon)) { errors.push('Zeile ' + (i + 1) + ': lat/lon keine Zahl'); return; }
            points.push({ id: id || String(points.length), lat: lat, lon: lon, datetime: rest[2] });
        });
        return { points: points, errors: errors };
    }

    function providerSummary(item) {
        if (item.error) return '<span class="st-err">' + esc(item.error.message) + '</span>';
        const parts = (item.providers || []).map(p =>
            p.status === 'ok'
                ? '<span class="st-ok">' + esc(p.id) + '</span>'
                : '<span class="st-err" title="' + esc(p.error || '') + '">' + esc(p.id) + ': ' + esc(p.status) + '</span>');
        return parts.join(' ');
    }

    function addRow(item) {
        const tr = document.createElement('tr');
        const q = item.query || {};
        const c = q.coordinate || {};
        tr.innerHTML = '<td>' + esc(item.id) + '</td><td>' + (c.lat ?? '') + '</td><td>' +
            (c.lon ?? '') + '</td><td>' + esc(q.datetime || '') + '</td><td>' + providerSummary(item) + '</td>';
        tbody.appendChild(tr);
    }

    function setRunning(running) {
        runBtn.disabled = running;
        abortBtn.disabled = !running;
        inputEl.disabled = running;
    }

    async function runBatch(points) {
        const body = { points: points };
        const provs = document.getElementById('batchProviders').value.trim();
        if (provs) body.providers = provs.split(',').map(s => s.trim()).filter(Boolean);
        const gdd = document.getElementById('batchGddBase').value.trim();
        if (gdd) body.gddBase = Number(gdd.replace(',', '.'));
        const ref = document.getElementById('batchRefPeriod').value.trim();
        if (ref) body.refPeriod = ref;

        controller = new AbortController();
        collected = [];
        tbody.innerHTML = '';
        table.hidden = false;
        progWrap.hidden = false;
        prog.max = points.length;
        prog.value = 0;
        progText.textContent = '0 / ' + points.length;
        exportBtn.disabled = true;
        setRunning(true);
        runStart = Date.now();
        try {
            const resp = await fetch('/api/v1/query/batch', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'Accept': 'application/x-ndjson' },
                body: JSON.stringify(body),
                signal: controller.signal
            });
            if (!resp.ok) {
                const err = await resp.json().catch(() => ({}));
                throw new Error(err.message || ('HTTP ' + resp.status));
            }
            const reader = resp.body.getReader();
            const decoder = new TextDecoder();
            let buf = '';
            for (;;) {
                const { done, value } = await reader.read();
                if (done) break;
                buf += decoder.decode(value, { stream: true });
                let nl;
                while ((nl = buf.indexOf('\n')) >= 0) {
                    const line = buf.slice(0, nl).trim();
                    buf = buf.slice(nl + 1);
                    if (!line) continue;
                    const item = JSON.parse(line);
                    collected.push(item);
                    addRow(item);
                    prog.value = collected.length;
                    progText.textContent = collected.length + ' / ' + points.length;
                }
            }
            if (collected.length < points.length) {
                showBatchError('Stream vorzeitig beendet: ' + collected.length + ' von ' + points.length + ' Punkten erhalten. Erneut starten ist dank Cache günstig.');
            }
        } catch (e) {
            if (e.name !== 'AbortError') showBatchError(e.message);
        } finally {
            setRunning(false);
            exportBtn.disabled = collected.length === 0;
            controller = null;
        }
    }

    function showBatchError(msg) {
        errorBox.textContent = msg;
        errorBox.style.display = 'block';
    }

    form.addEventListener('submit', function (e) {
        e.preventDefault();
        errorBox.style.display = 'none';
        const parsed = parseBatchLines(inputEl.value);
        if (parsed.errors.length) { showBatchError(parsed.errors.join(' — ')); return; }
        if (!parsed.points.length) { showBatchError('Keine Punkte eingegeben.'); return; }
        runBatch(parsed.points);
    });
    abortBtn.addEventListener('click', function () { if (controller) controller.abort(); });
    exportBtn.addEventListener('click', function () {
        const envelope = { results: collected, total: collected.length, processing_time_ms: Date.now() - runStart };
        const blob = new Blob([JSON.stringify(envelope, null, 2)], { type: 'application/json' });
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = 'tempus-batch.json';
        a.click();
        URL.revokeObjectURL(a.href);
    });
})();
```

(`esc()` existiert bereits im Frontend-Script und wird mitbenutzt. Das bestehende Einzel-Formular und seine IDs bleiben unangetastet; nur das Wrapper-`<div id="singlePanel">` kommt hinzu.)

- [ ] **Step 5: Smoke-Test grün** — `go test ./internal/adapters/http/ -run TestFrontend -v` → PASS

- [ ] **Step 6: Manueller Smoke-Test**

```bash
make run &   # bzw. das im Makefile dokumentierte Run-Target
# Browser: http://localhost:8080 → Tab „Batch“:
# 1) 3 Zeilen einfügen (eine davon absichtlich kaputt), Batch starten
# 2) Progress-Balken füllt sich, Tabelle wächst zeilenweise, kaputte Zeile rot
# 3) Abbrechen mitten im Lauf → Buttons zurückgesetzt
# 4) Export lädt tempus-batch.json mit {results,total,processing_time_ms}
kill %1
```

- [ ] **Step 7: Verify + Commit**

```bash
make verify
git add internal/adapters/http/index.html internal/adapters/http/frontend_test.go
git commit -m "feat(frontend): add batch tab with NDJSON streaming, progress bar, and JSON export"
```

---

### Task 8: Doku, Doc-Drift-Gate, Abschluss

**Files:**
- Modify: `docs/reference/http-api.md` (Batch-Endpoint-Abschnitt)
- Modify: `docs/reference/configuration.md` (neue Config-Keys)

**Interfaces:**
- Consumes: alles aus Task 1–7.

- [ ] **Step 1: `docs/reference/http-api.md` ergänzen** — Abschnitt `POST /api/v1/query/batch` im Stil der Seite: Request-Beispiel (aus der Spec, inkl. `gddBase: 7`-Beispiel), beide Antwortmodi (sync-Envelope, NDJSON), Kappungen (400 ab `max_points`, 413 ab `max_sync_points` mit NDJSON-Hinweis, Body-Cap), Per-Item-Fehlerform `{id, error:{message}}`, Hinweis auf Drosselung/Tagesbudget (transiente „retry tomorrow"-Fehler) und dass GDD5/GDD10 immer enthalten sind.

- [ ] **Step 2: `docs/reference/configuration.md` ergänzen** — Tabelle mit den 9 neuen Keys aus der Spec (`query.batch.max_points|max_sync_points|concurrency`, `providers.openmeteo.rate_per_minute|retry_attempts|daily_budget|weights.weather|weights.aggregate|weights.bioclim`), Defaults (10000/1000/4, 500/3/8000, 1/2/30) und Env-Namen (`TEMPUS_QUERY_BATCH_MAX_POINTS`, `TEMPUS_PROVIDERS_OPENMETEO_RATE_PER_MINUTE`, …), im Tabellenformat der Seite.

- [ ] **Step 3: Doc-Drift-Gate fahren** — den Skill `doc-drift-check` ausführen (vergleicht Code ↔ Spec ↔ Prosa und endet mit `scripts/check-doc-drift.sh`, muss grün sein).

- [ ] **Step 4: Abschluss-Verify über alles**

Run: `make verify`
Expected: grün (fmt-check, vet, lint, test, arch, debt-guard). Falls der CodeCharta-/Komplexitäts-Ratchet über neue Dateien meckert: betroffene Funktion aufteilen, NICHT die Baseline anheben.

- [ ] **Step 5: Commit**

```bash
git add docs/
git commit -m "docs: document POST /query/batch and the Open-Meteo throttle/budget config"
```

- [ ] **Step 6: Branch/PR** — gemäß Ship-Workflow des Repos (`superpowers:finishing-a-development-branch`): PR gegen `main`, OpenAPI-Änderung ist additiv (kein `api-breaking-ok`-Label nötig), Copilot-Review-Schleife abwarten.

---

## Self-Review-Notizen (bereits eingearbeitet)

- Spec-Abdeckung: Teil 1 → Task 6, Teil 2 → Tasks 1–3, Teil 3 → Task 4, Teil 4 → Task 7; Kappungen/NDJSON/Per-Item-Fehler/Dedup/Budget je in den Task-Tests verankert. Die Spec-Formulierung zur Budget-Prüfung wurde auf die Kontext-Marker-Durchsetzung im Transport präzisiert (Spec-Commit zusammen mit diesem Plan).
- `TestRoutesMatchOpenAPISpec` bleibt nur grün, wenn Route + Spec im selben Commit landen → in Task 6 zusammengelegt; Spiegel-Spec byte-identisch kopieren.
- Typkonsistenz: `input.BatchPoint/BatchItem/BatchItemError/BatchService` (Task 5) werden in Task 6 exakt so konsumiert; `omhttp.Options/NewTransport/NewBudget/ErrBudgetExhausted` (Task 1) in Task 3; `CacheKey(…, params string)` (Task 4) ändert alle Aufrufer im selben Task.
- Bekannte bewusste Lücken (YAGNI, in der Spec als Nicht-Ziele): kein Job-Modell, kein Multi-Koordinaten-Packing, keine Persistenz, keine Auth. Der `retryAfter`-Hinweis des aggregate-Adapters bleibt hartkodiert 30 s (kosmetisch): das tatsächliche Warten übernimmt jetzt der Transport anhand des echten Retry-After-Headers.
