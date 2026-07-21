# tempus Weather Feature-Query Service — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go service that takes a WGS84 coordinate + UTC datetime + IANA timezone, calls cached feature providers (weather via Open-Meteo first), and returns Ortus-style features each carrying mandatory attribution, encoding per-provider failures so an idempotent client can retry later.

**Architecture:** Hexagonal (domain / ports / application / adapters / app composition root) on the full `new-go-service` harness. External sources are `FeatureProvider` output-port implementations registered in a `Registry`; a `CachingProvider` decorator adds a pluggable `Cache` (disk default). `FeatureService` fans providers out concurrently and assembles the envelope. A `GET /api/v1/query` HTTP adapter (gorilla/mux) returns 200 on any valid request; validation errors return 400.

**Tech Stack:** Go 1.23+, gorilla/mux, spf13/viper, go.etcd.io/bbolt, OpenTelemetry, gopkg.in/yaml.v3, golangci-lint (depguard), gremlins, MkDocs, release-please, goreleaser.

**Harness source:** The `new-go-service` skill and its templates live in this repo at `claude-skills/skills/new-go-service/` (`templates/`, `reference/`). "Copy template X" means copy from there and apply the substitutions below.

## Global Constraints

- Module path: `github.com/jobrunner/tempus`. Env prefix: `TEMPUS`. Service name: `tempus`.
- Substitutions when copying any template: `<module>`→`github.com/jobrunner/tempus`, `<svc>`→`tempus`, `<PREFIX>`→`TEMPUS`. Strip the `// Template:` / `// Copy to …` header comments after copying.
- Go version floor: `go 1.23`.
- **Every returned feature MUST carry a non-empty `license` object (`name`, `url`, `attribution`).** A feature without attribution is a bug.
- A **valid** request always returns **HTTP 200**, even if every provider fails; failures live in `providers[]` with `retryable`. Only request-validation failures return **400** (`retryable:false`).
- The query endpoint is **idempotent**: same inputs ⇒ same output; no side effects other than cache population.
- **No weather for a future datetime**: `datetime > now(UTC)` ⇒ 400.
- Datetime with no offset is interpreted as **UTC**; the instant is normalized to the full hour.
- Conventional-commit messages (release-please). Each task's final commit uses `feat:` / `test:` / `chore:` / `docs:` as fitting.

---

## Phase A — Scaffold & harness bootstrap

### Task 1: Module, hexagonal skeleton, core templates

**Files:**
- Create: `go.mod`, `cmd/tempus/main.go`, `internal/config/config.go`, `internal/ports/output/tracer.go`, `internal/adapters/telemetry/sloghandler.go`
- Create empty package dirs: `internal/domain/`, `internal/ports/input/`, `internal/application/`, `internal/adapters/http/`, `internal/adapters/openmeteo/`, `internal/adapters/cache/`, `internal/app/`

**Interfaces:**
- Produces: `config.Config`, `config.Load(path string) (*Config, error)`, `config.Defaults()`; `output.Tracer` + `output.NoOpTracer`; `telemetry.NewSpanContextHandler`.

- [ ] **Step 1: Init module and tidy deps**

```bash
cd /Users/jbrunner/work/projects/tempus
go mod init github.com/jobrunner/tempus
go get github.com/spf13/viper@latest
go get go.opentelemetry.io/otel/trace@latest
go get go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux@latest
go get github.com/gorilla/mux@latest
go get gopkg.in/yaml.v3@latest
go get go.etcd.io/bbolt@latest
```

- [ ] **Step 2: Copy core templates with substitutions**

```bash
cp claude-skills/skills/new-go-service/templates/config.go internal/config/config.go
cp claude-skills/skills/new-go-service/templates/tracer_port.go internal/ports/output/tracer.go
cp claude-skills/skills/new-go-service/templates/slog_setup.go internal/adapters/telemetry/sloghandler.go
# substitutions
sed -i '' 's/<PREFIX>/TEMPUS/g; s/<module>/github.com\/jobrunner\/tempus/g; s/<svc>/tempus/g' \
  internal/config/config.go internal/ports/output/tracer.go internal/adapters/telemetry/sloghandler.go
```
Then hand-edit `internal/adapters/telemetry/sloghandler.go`: keep the `SpanContextHandler` type + methods; delete the trailing `/* … */` block (its `setupLogger`/`buildHandler`/`isCanceled` snippets are reproduced in `main.go` in Step 3).

- [ ] **Step 3: Write the thin entrypoint**

Create `cmd/tempus/main.go`:

```go
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jobrunner/tempus/internal/adapters/telemetry"
	"github.com/jobrunner/tempus/internal/app"
	"github.com/jobrunner/tempus/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger := setupLogger(cfg.Logging)
	slog.SetDefault(logger)

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("build app", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}

func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	return slog.New(telemetry.NewSpanContextHandler(buildHandler(cfg, os.Stdout)))
}

func buildHandler(cfg config.LoggingConfig, w io.Writer) slog.Handler {
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.Level))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}
```

(`app.New`/`Run` land in Task 16; until then this won't compile — that's expected. Do NOT commit a broken build; this task's commit comes after Step 4 stubs `app`.)

- [ ] **Step 4: Add a temporary app stub so the tree compiles**

Create `internal/app/app.go`:

```go
package app

import (
	"context"
	"log/slog"

	"github.com/jobrunner/tempus/internal/config"
)

// App is the composition root. Fully wired in Task 16.
type App struct {
	cfg    *config.Config
	logger *slog.Logger
}

// New builds the application from config. Expanded in Task 16.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	return &App{cfg: cfg, logger: logger}, nil
}

// Run blocks until ctx is canceled. Expanded in Task 16.
func (a *App) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
```

- [ ] **Step 5: Verify build + commit**

Run: `go build ./... && go vet ./...`
Expected: no output (success).

```bash
git add -A
git commit -m "chore: scaffold hexagonal skeleton and core templates"
```

---

### Task 2: Enforce import boundaries + Makefile

**Files:**
- Create: `.golangci.yml`, `Makefile`

- [ ] **Step 1: Copy the lint + Makefile templates**

```bash
cp claude-skills/skills/new-go-service/templates/golangci.yml .golangci.yml
cp claude-skills/skills/new-go-service/templates/Makefile Makefile
sed -i '' 's/<PREFIX>/TEMPUS/g; s/<module>/github.com\/jobrunner\/tempus/g; s/<svc>/tempus/g' .golangci.yml Makefile
```

- [ ] **Step 2: Point depguard at our packages**

Edit `.golangci.yml` depguard rules so the sealed layers match this repo:
- `internal/domain` may import: stdlib only (no other `internal/...`).
- `internal/application` may import: `internal/domain`, `internal/ports/...` (NOT `internal/adapters/...`).
- `internal/ports/...` may import: `internal/domain` only.
Use the module path `github.com/jobrunner/tempus/...` in the `deny`/`allow` `pkg:` entries.

- [ ] **Step 3: Run the architecture gate**

Run: `make arch`
Expected: passes (depguard + gomodguard + `go mod tidy -diff` clean). If `golangci-lint` is missing: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`.

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "chore: enforce hexagonal import boundaries and add Makefile"
```

---

### Task 3: Extend config for cache + providers

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.CacheConfig{Type, Path}`, `config.ProvidersConfig{OpenMeteo OpenMeteoConfig}`, `config.OpenMeteoConfig{Enabled, ArchiveBaseURL, ForecastBaseURL, Timeout, ArchiveDelay}`; `config.QueryConfig{Timeout}`. All reachable via `TEMPUS_*` env.

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	t.Setenv("TEMPUS_CACHE_TYPE", "memory")
	t.Setenv("TEMPUS_PROVIDERS_OPENMETEO_TIMEOUT", "7s")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Cache.Type != "memory" {
		t.Errorf("cache.type = %q, want memory", cfg.Cache.Type)
	}
	if cfg.Cache.Path == "" {
		t.Error("cache.path default must be set")
	}
	if cfg.Providers.OpenMeteo.Timeout != 7*time.Second {
		t.Errorf("openmeteo.timeout = %v, want 7s", cfg.Providers.OpenMeteo.Timeout)
	}
	if !cfg.Providers.OpenMeteo.Enabled {
		t.Error("openmeteo enabled default must be true")
	}
	if cfg.Providers.OpenMeteo.ArchiveDelay != 5*24*time.Hour {
		t.Errorf("archiveDelay = %v, want 120h", cfg.Providers.OpenMeteo.ArchiveDelay)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/config/ -run TestLoadDefaultsAndEnvOverride -v`
Expected: FAIL (unknown fields `Cache`, `Providers`).

- [ ] **Step 3: Add the config structs + defaults**

In `internal/config/config.go`, add to `Config`:

```go
	Cache     CacheConfig     `mapstructure:"cache"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Query     QueryConfig     `mapstructure:"query"`
```

Add the types:

```go
type CacheConfig struct {
	Type string `mapstructure:"type"` // disk|memory|redis
	Path string `mapstructure:"path"`
}

type QueryConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
}

type ProvidersConfig struct {
	OpenMeteo OpenMeteoConfig `mapstructure:"openmeteo"`
}

type OpenMeteoConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	ArchiveBaseURL  string        `mapstructure:"archive_base_url"`
	ForecastBaseURL string        `mapstructure:"forecast_base_url"`
	Timeout         time.Duration `mapstructure:"timeout"`
	ArchiveDelay    time.Duration `mapstructure:"archive_delay"`
}
```

Add to `Defaults()`:

```go
	viper.SetDefault("cache.type", "disk")
	viper.SetDefault("cache.path", "./data/cache.bolt")
	viper.SetDefault("query.timeout", 30*time.Second)
	viper.SetDefault("providers.openmeteo.enabled", true)
	viper.SetDefault("providers.openmeteo.archive_base_url", "https://archive-api.open-meteo.com/v1/archive")
	viper.SetDefault("providers.openmeteo.forecast_base_url", "https://api.open-meteo.com/v1/forecast")
	viper.SetDefault("providers.openmeteo.timeout", 10*time.Second)
	viper.SetDefault("providers.openmeteo.archive_delay", 5*24*time.Hour)
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/config/ -run TestLoadDefaultsAndEnvOverride -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add cache, query, and provider configuration"
```

---

## Phase B — Domain

### Task 4: Core domain types

**Files:**
- Create: `internal/domain/feature.go`
- Test: `internal/domain/feature_test.go`

**Interfaces:**
- Produces: `domain.License`, `domain.Coordinate`, `domain.Geometry`, `domain.Feature`, `domain.ProviderResult`, `domain.ProviderStatus`, `domain.QueryResult`, `domain.QueryEcho`, status constants `StatusOK`/`StatusUnavailable`/`StatusError`, and `domain.NewPointFeature(coord Coordinate, props map[string]any, lic License) Feature`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/feature_test.go`:

```go
package domain

import "testing"

func TestNewPointFeatureShape(t *testing.T) {
	lic := License{Name: "CC-BY 4.0", URL: "https://x", Attribution: "by X"}
	f := NewPointFeature(Coordinate{Lat: 49.79, Lon: 9.93}, map[string]any{"temperature2m": 21.4}, lic)

	if f.Type != "Feature" {
		t.Errorf("Type = %q, want Feature", f.Type)
	}
	if f.Geometry.Type != "Point" {
		t.Errorf("Geometry.Type = %q, want Point", f.Geometry.Type)
	}
	// GeoJSON order is [lon, lat].
	if got := f.Geometry.Coordinates; len(got) != 2 || got[0] != 9.93 || got[1] != 49.79 {
		t.Errorf("Coordinates = %v, want [9.93 49.79]", got)
	}
	if f.License.Attribution == "" {
		t.Error("feature must carry attribution")
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/domain/ -run TestNewPointFeatureShape -v`
Expected: FAIL (undefined types).

- [ ] **Step 3: Implement the types**

Create `internal/domain/feature.go`:

```go
package domain

// Provider status values reported in QueryResult.Providers.
const (
	StatusOK          = "ok"
	StatusUnavailable = "unavailable"
	StatusError       = "error"
)

// License is the attribution block attached to every feature. All three fields
// are required — a feature without attribution is a contract violation.
type License struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
}

// Coordinate is a WGS84 point.
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Geometry is a GeoJSON geometry (only Point is produced today).
type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [lon, lat]
}

// Feature is a GeoJSON-style feature returned by a provider.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   Geometry       `json:"geometry"`
	Properties map[string]any `json:"properties"`
	License    License        `json:"license"`
}

// NewPointFeature builds a Point Feature at coord with the given properties and
// license. coord here is the provider-resolved location (e.g. grid cell).
func NewPointFeature(coord Coordinate, props map[string]any, lic License) Feature {
	return Feature{
		Type:       "Feature",
		Geometry:   Geometry{Type: "Point", Coordinates: []float64{coord.Lon, coord.Lat}},
		Properties: props,
		License:    lic,
	}
}

// ProviderResult is what a FeatureProvider returns: the feature plus whether it
// was served from cache (set by the caching decorator, false otherwise).
type ProviderResult struct {
	Feature Feature
	Cached  bool
}

// ProviderStatus reports the outcome of one provider in the response envelope.
type ProviderStatus struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Cached     bool   `json:"cached,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
	RetryAfter string `json:"retryAfter,omitempty"`
	Error      string `json:"error,omitempty"`
}

// QueryEcho echoes the resolved request (makes the idempotent contract explicit).
type QueryEcho struct {
	Coordinate Coordinate `json:"coordinate"`
	Datetime   string     `json:"datetime"`
	Timezone   string     `json:"timezone"`
	LocalTime  string     `json:"localTime"`
}

// QueryResult is the assembled response payload.
type QueryResult struct {
	Query     QueryEcho        `json:"query"`
	Features  []Feature        `json:"features"`
	Providers []ProviderStatus `json:"providers"`
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/domain/ -run TestNewPointFeatureShape -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add core domain feature and result types"
```

---

### Task 5: Request parsing & validation

**Files:**
- Create: `internal/domain/request.go`
- Test: `internal/domain/request_test.go`

**Interfaces:**
- Consumes: `domain.Coordinate`.
- Produces: `domain.QueryRequest{Coordinate, Instant time.Time, Timezone *time.Location, TimezoneID string, Providers []string}`; `domain.ValidationError{Field, Message}` (implements `error`); `domain.ParseQueryRequest(lat, lon, datetime, tzID string, providers []string, now time.Time) (QueryRequest, error)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/domain/request_test.go`:

```go
package domain

import (
	"errors"
	"testing"
	"time"
)

func now() time.Time { return time.Date(2026, 7, 21, 12, 30, 0, 0, time.UTC) }

func TestParseQueryRequest_OK_AssumesUTCAndTruncatesHour(t *testing.T) {
	req, err := ParseQueryRequest("49.79", "9.93", "2025-06-15T13:45:00", "Europe/Berlin", nil, now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !req.Instant.Equal(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("Instant = %v, want 2025-06-15T13:00:00Z", req.Instant)
	}
	if req.Timezone.String() != "Europe/Berlin" {
		t.Errorf("Timezone = %v", req.Timezone)
	}
}

func TestParseQueryRequest_FutureRejected(t *testing.T) {
	_, err := ParseQueryRequest("0", "0", "2026-07-21T13:00:00Z", "UTC", nil, now())
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "datetime" {
		t.Fatalf("want ValidationError on datetime (future), got %v", err)
	}
}

func TestParseQueryRequest_BadInputs(t *testing.T) {
	cases := []struct{ name, lat, lon, dt, tz, field string }{
		{"lat-range", "91", "0", "2020-01-01T00:00:00Z", "UTC", "lat"},
		{"lon-range", "0", "181", "2020-01-01T00:00:00Z", "UTC", "lon"},
		{"bad-datetime", "0", "0", "not-a-date", "UTC", "datetime"},
		{"bad-tz", "0", "0", "2020-01-01T00:00:00Z", "Mars/Phobos", "timezone"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseQueryRequest(c.lat, c.lon, c.dt, c.tz, nil, now())
			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("want ValidationError on %q, got %v", c.field, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/domain/ -run TestParseQueryRequest -v`
Expected: FAIL (undefined `ParseQueryRequest`/`ValidationError`).

- [ ] **Step 3: Implement**

Create `internal/domain/request.go`:

```go
package domain

import (
	"fmt"
	"strconv"
	"time"
)

// QueryRequest is a validated feature-query request. Instant is UTC, truncated
// to the hour. Providers is an optional filter (empty ⇒ all registered).
type QueryRequest struct {
	Coordinate Coordinate
	Instant    time.Time
	Timezone   *time.Location
	TimezoneID string
	Providers  []string
}

// ValidationError is a client input error → HTTP 400, not retryable.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// datetimeLayouts are tried in order. RFC3339 (with offset) first; the
// offset-less forms are interpreted as UTC per the service contract.
var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

// ParseQueryRequest validates raw string inputs and builds a QueryRequest.
// now is injected (from the Clock port) so "future" is testable.
func ParseQueryRequest(lat, lon, datetime, tzID string, providers []string, now time.Time) (QueryRequest, error) {
	latF, err := strconv.ParseFloat(lat, 64)
	if err != nil || latF < -90 || latF > 90 {
		return QueryRequest{}, ValidationError{"lat", "must be a number in [-90,90]"}
	}
	lonF, err := strconv.ParseFloat(lon, 64)
	if err != nil || lonF < -180 || lonF > 180 {
		return QueryRequest{}, ValidationError{"lon", "must be a number in [-180,180]"}
	}

	instant, ok := parseInstant(datetime)
	if !ok {
		return QueryRequest{}, ValidationError{"datetime", "must be RFC3339 or YYYY-MM-DDTHH:MM[:SS] (UTC assumed)"}
	}
	instant = instant.UTC().Truncate(time.Hour)
	if instant.After(now.UTC()) {
		return QueryRequest{}, ValidationError{"datetime", "must not be in the future"}
	}

	loc, err := time.LoadLocation(tzID)
	if err != nil {
		return QueryRequest{}, ValidationError{"timezone", "must be a valid IANA timezone id"}
	}

	return QueryRequest{
		Coordinate: Coordinate{Lat: latF, Lon: lonF},
		Instant:    instant,
		Timezone:   loc,
		TimezoneID: tzID,
		Providers:  providers,
	}, nil
}

func parseInstant(s string) (time.Time, bool) {
	for _, layout := range datetimeLayouts {
		// Offset-less layouts parse in UTC because we pass time.UTC via ParseInLocation.
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/domain/ -v`
Expected: PASS (all domain tests).

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: parse and validate feature-query requests"
```

---

## Phase C — Ports & application

### Task 6: Output ports, provider error taxonomy, system clock

**Files:**
- Create: `internal/ports/output/provider.go`, `internal/ports/output/cache.go`, `internal/ports/output/clock.go`, `internal/ports/output/errors.go`
- Create: `internal/adapters/clock/clock.go`
- Test: `internal/ports/output/errors_test.go`

**Interfaces:**
- Consumes: `domain.*`.
- Produces:
  - `output.FeatureProvider{ ID() string; Kind() string; Attribution() domain.License; Fetch(ctx, domain.QueryRequest) (domain.ProviderResult, error) }`
  - `output.Cache{ Get(ctx, key) ([]byte, bool, error); Set(ctx, key, []byte, ttl) error }`
  - `output.Clock{ Now() time.Time }`
  - `output.ProviderError{ Class ErrorClass; Retryable bool; RetryAfter time.Duration; Err error }` with `ClassTransient`/`ClassNotYetAvailable`/`ClassPermanent`, constructors `NewTransientError(err, retryAfter)`, `NewNotYetAvailableError(retryAfter)`, `NewPermanentError(err)`, and `AsProviderError(error) (ProviderError, bool)`.
  - `clock.System{}` implementing `output.Clock`.

- [ ] **Step 1: Write the failing test**

Create `internal/ports/output/errors_test.go`:

```go
package output

import (
	"errors"
	"testing"
	"time"
)

func TestAsProviderError(t *testing.T) {
	pe := NewTransientError(errors.New("dial tcp: timeout"), 30*time.Second)
	got, ok := AsProviderError(pe)
	if !ok || got.Class != ClassTransient || !got.Retryable || got.RetryAfter != 30*time.Second {
		t.Fatalf("transient not classified correctly: %+v ok=%v", got, ok)
	}
	if _, ok := AsProviderError(errors.New("plain")); ok {
		t.Error("plain error must not classify as ProviderError")
	}
	if !NewNotYetAvailableError(time.Hour).Retryable {
		t.Error("not-yet-available must be retryable")
	}
	if NewPermanentError(errors.New("x")).Retryable {
		t.Error("permanent must not be retryable")
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/ports/output/ -v`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Implement the ports + errors + clock**

Create `internal/ports/output/provider.go`:

```go
package output

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureProvider is a driven port: one external source or computation.
// Fetch returns the feature (with attribution) and whether it was cached.
type FeatureProvider interface {
	ID() string
	Kind() string
	Attribution() domain.License
	Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error)
}
```

Create `internal/ports/output/cache.go`:

```go
package output

import (
	"context"
	"time"
)

// Cache is a driven port for a byte-blob cache. ttl == 0 means "never expire".
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
```

Create `internal/ports/output/clock.go`:

```go
package output

import "time"

// Clock is a driven port for the current time (injected for testability).
type Clock interface {
	Now() time.Time
}
```

Create `internal/ports/output/errors.go`:

```go
package output

import (
	"errors"
	"fmt"
	"time"
)

// ErrorClass classifies a provider failure for retry coding.
type ErrorClass int

const (
	ClassTransient       ErrorClass = iota // network/5xx/timeout/429 — retry soon
	ClassNotYetAvailable                   // past but not yet in the archive — retry later
	ClassPermanent                         // 4xx/contract violation — do not retry
)

// ProviderError is a classified failure a FeatureProvider may return.
type ProviderError struct {
	Class      ErrorClass
	Retryable  bool
	RetryAfter time.Duration // 0 = unknown
	Err        error
}

func (e ProviderError) Error() string { return fmt.Sprintf("provider error (class %d): %v", e.Class, e.Err) }
func (e ProviderError) Unwrap() error { return e.Err }

// NewTransientError: source unreachable / temporary — retryable.
func NewTransientError(err error, retryAfter time.Duration) ProviderError {
	return ProviderError{Class: ClassTransient, Retryable: true, RetryAfter: retryAfter, Err: err}
}

// NewNotYetAvailableError: the datetime is valid+past but the source has no data
// for it yet (archive delay) — retryable once the data matures.
func NewNotYetAvailableError(retryAfter time.Duration) ProviderError {
	return ProviderError{Class: ClassNotYetAvailable, Retryable: true, RetryAfter: retryAfter,
		Err: errors.New("data not yet available for the requested time")}
}

// NewPermanentError: the source rejected the request in a non-recoverable way.
func NewPermanentError(err error) ProviderError {
	return ProviderError{Class: ClassPermanent, Retryable: false, Err: err}
}

// AsProviderError extracts a ProviderError from err, if present.
func AsProviderError(err error) (ProviderError, bool) {
	var pe ProviderError
	if errors.As(err, &pe) {
		return pe, true
	}
	return ProviderError{}, false
}
```

Create `internal/adapters/clock/clock.go`:

```go
package clock

import "time"

// System is the real Clock (output.Clock port).
type System struct{}

// Now returns the current time.
func (System) Now() time.Time { return time.Now() }
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/ports/output/ -v && go build ./...`
Expected: PASS + build succeeds.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add provider/cache/clock output ports and error taxonomy"
```

---

### Task 7: Input ports (replace template Item surface)

**Files:**
- Create: `internal/ports/input/ports.go`

**Interfaces:**
- Consumes: `domain.*`.
- Produces: `input.FeatureService{ Query(ctx, domain.QueryRequest) (domain.QueryResult, error) }`; `input.HealthChecker{ Ready(ctx) bool }`; `input.ProviderLister{ Providers(ctx) []ProviderInfo }`; `input.ProviderInfo{ ID, Kind string; License domain.License }`.

- [ ] **Step 1: Write the file (interfaces only — no test; exercised via adapters/application)**

Create `internal/ports/input/ports.go`:

```go
// Package input holds the driving ports the HTTP adapter depends on.
package input

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureService is the primary business port the HTTP adapter calls.
type FeatureService interface {
	Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error)
}

// ProviderInfo describes an available provider for GET /api/v1/providers.
type ProviderInfo struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"`
	License domain.License `json:"license"`
}

// ProviderLister lists the registered providers and their attribution.
type ProviderLister interface {
	Providers(ctx context.Context) []ProviderInfo
}

// HealthChecker backs the readiness probe.
type HealthChecker interface {
	Ready(ctx context.Context) bool
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: define feature-query driving ports"
```

---

### Task 8: Provider registry

**Files:**
- Create: `internal/application/registry.go`
- Test: `internal/application/registry_test.go`

**Interfaces:**
- Consumes: `output.FeatureProvider`, `input.ProviderInfo`.
- Produces: `application.Registry` with `NewRegistry()`, `Register(output.FeatureProvider)`, `Get(id) (output.FeatureProvider, bool)`, `All() []output.FeatureProvider` (stable registration order), and `Providers(ctx) []input.ProviderInfo` (implements `input.ProviderLister`).

- [ ] **Step 1: Write the failing test**

Create `internal/application/registry_test.go`:

```go
package application

import (
	"context"
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
)

type stubProvider struct{ id, kind string }

func (s stubProvider) ID() string                 { return s.id }
func (s stubProvider) Kind() string               { return s.kind }
func (s stubProvider) Attribution() domain.License { return domain.License{Name: s.id} }
func (s stubProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, nil
}

func TestRegistryOrderAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(stubProvider{"open-meteo", "weather"})
	r.Register(stubProvider{"astro", "astronomy"})

	if got := r.All(); len(got) != 2 || got[0].ID() != "open-meteo" || got[1].ID() != "astro" {
		t.Fatalf("All() order wrong: %v", got)
	}
	if _, ok := r.Get("astro"); !ok {
		t.Error("Get(astro) missing")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("Get(nope) should be absent")
	}
	infos := r.Providers(context.Background())
	if len(infos) != 2 || infos[0].License.Name != "open-meteo" {
		t.Fatalf("Providers() wrong: %v", infos)
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/application/ -run TestRegistry -v`
Expected: FAIL (undefined `NewRegistry`).

- [ ] **Step 3: Implement**

Create `internal/application/registry.go`:

```go
package application

import (
	"context"

	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// Registry holds the registered feature providers in registration order.
type Registry struct {
	providers map[string]output.FeatureProvider
	order     []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]output.FeatureProvider{}}
}

// Register adds a provider (last registration for an id wins; order preserved).
func (r *Registry) Register(p output.FeatureProvider) {
	if _, exists := r.providers[p.ID()]; !exists {
		r.order = append(r.order, p.ID())
	}
	r.providers[p.ID()] = p
}

// Get returns the provider with id, if registered.
func (r *Registry) Get(id string) (output.FeatureProvider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// All returns providers in registration order.
func (r *Registry) All() []output.FeatureProvider {
	out := make([]output.FeatureProvider, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.providers[id])
	}
	return out
}

// Providers implements input.ProviderLister.
func (r *Registry) Providers(context.Context) []input.ProviderInfo {
	out := make([]input.ProviderInfo, 0, len(r.order))
	for _, id := range r.order {
		p := r.providers[id]
		out = append(out, input.ProviderInfo{ID: p.ID(), Kind: p.Kind(), License: p.Attribution()})
	}
	return out
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/application/ -run TestRegistry -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add provider registry"
```

---

### Task 9: Caching decorator

**Files:**
- Create: `internal/application/caching.go`
- Test: `internal/application/caching_test.go`

**Interfaces:**
- Consumes: `output.FeatureProvider`, `output.Cache`, `output.Clock`, `domain.*`.
- Produces: `application.CachingProvider` (implements `output.FeatureProvider`); `application.NewCachingProvider(inner output.FeatureProvider, cache output.Cache, clock output.Clock, opts CachingOptions)`; `application.CachingOptions{Version string, ArchiveDelay, MatureTTL, ImmatureTTL time.Duration, LatLonPrecision int}`; `application.CacheKey(providerID, version string, req domain.QueryRequest, precision int) string`.

- [ ] **Step 1: Write the failing tests**

Create `internal/application/caching_test.go`:

```go
package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

type fakeCache struct {
	store map[string][]byte
	sets  int
	setTTL time.Duration
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string][]byte{}} }
func (c *fakeCache) Get(_ context.Context, k string) ([]byte, bool, error) {
	v, ok := c.store[k]
	return v, ok, nil
}
func (c *fakeCache) Set(_ context.Context, k string, v []byte, ttl time.Duration) error {
	c.store[k] = v
	c.sets++
	c.setTTL = ttl
	return nil
}

type countingProvider struct {
	calls int
	feat  domain.Feature
}

func (p *countingProvider) ID() string                 { return "open-meteo" }
func (p *countingProvider) Kind() string               { return "weather" }
func (p *countingProvider) Attribution() domain.License { return domain.License{Name: "CC-BY 4.0"} }
func (p *countingProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	p.calls++
	return domain.ProviderResult{Feature: p.feat}, nil
}

type fixedClock struct{ t time.Time }
func (c fixedClock) Now() time.Time { return c.t }

func req(instant time.Time) domain.QueryRequest {
	return domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.79123, Lon: 9.93456}, Instant: instant}
}

func opts() CachingOptions {
	return CachingOptions{Version: "1", ArchiveDelay: 5 * 24 * time.Hour,
		MatureTTL: 365 * 24 * time.Hour, ImmatureTTL: time.Hour, LatLonPrecision: 2}
}

func TestCaching_MissThenHit(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	inner := &countingProvider{feat: domain.NewPointFeature(domain.Coordinate{Lat: 49.79, Lon: 9.93}, map[string]any{"t": 1.0}, domain.License{Name: "x"})}
	cp := NewCachingProvider(inner, newFakeCache(), fixedClock{now}, opts())

	old := req(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) // well before archive delay
	r1, err := cp.Fetch(context.Background(), old)
	if err != nil || r1.Cached {
		t.Fatalf("first fetch: cached=%v err=%v", r1.Cached, err)
	}
	r2, err := cp.Fetch(context.Background(), old)
	if err != nil || !r2.Cached {
		t.Fatalf("second fetch must be cached: cached=%v err=%v", r2.Cached, err)
	}
	if inner.calls != 1 {
		t.Errorf("inner called %d times, want 1", inner.calls)
	}
}

func TestCaching_TTLByMaturity(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	mature := newFakeCache()
	NewCachingProvider(&countingProvider{}, mature, fixedClock{now}, opts()).
		Fetch(context.Background(), req(now.Add(-30*24*time.Hour)))
	if mature.setTTL != 365*24*time.Hour {
		t.Errorf("mature TTL = %v, want 8760h", mature.setTTL)
	}
	immature := newFakeCache()
	NewCachingProvider(&countingProvider{}, immature, fixedClock{now}, opts()).
		Fetch(context.Background(), req(now.Add(-2*time.Hour)))
	if immature.setTTL != time.Hour {
		t.Errorf("immature TTL = %v, want 1h", immature.setTTL)
	}
}

func TestCacheKey_RoundsCoords(t *testing.T) {
	instant := time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)
	a := CacheKey("open-meteo", "1", domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.791, Lon: 9.934}, Instant: instant}, 2)
	b := CacheKey("open-meteo", "1", domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.789, Lon: 9.937}, Instant: instant}, 2)
	if a != b {
		t.Errorf("coords within rounding must share a key: %s vs %s", a, b)
	}
}

func TestCaching_InnerErrorNotCached(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	fc := newFakeCache()
	failing := errorProvider{err: errors.New("boom")}
	cp := NewCachingProvider(failing, fc, fixedClock{now}, opts())
	if _, err := cp.Fetch(context.Background(), req(now.Add(-time.Hour))); err == nil {
		t.Fatal("expected error to propagate")
	}
	if fc.sets != 0 {
		t.Errorf("errors must not be cached, sets=%d", fc.sets)
	}
}

type errorProvider struct{ err error }

func (e errorProvider) ID() string                  { return "open-meteo" }
func (e errorProvider) Kind() string                { return "weather" }
func (e errorProvider) Attribution() domain.License { return domain.License{} }
func (e errorProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, e.err
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/application/ -run TestCach -v`
Expected: FAIL (undefined `NewCachingProvider`/`CacheKey`/`CachingOptions`).

- [ ] **Step 3: Implement**

Create `internal/application/caching.go`:

```go
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// CachingOptions configures the caching decorator.
type CachingOptions struct {
	Version         string        // cache schema version — bump to invalidate
	ArchiveDelay    time.Duration // instants older than this are immutable
	MatureTTL       time.Duration // TTL for immutable (mature) data
	ImmatureTTL     time.Duration // TTL for data still inside the maturity window
	LatLonPrecision int           // decimal places for the cache-key coordinate rounding
}

// CachingProvider decorates a FeatureProvider with a Cache. It implements
// output.FeatureProvider so the service treats it like any provider.
type CachingProvider struct {
	inner output.FeatureProvider
	cache output.Cache
	clock output.Clock
	opts  CachingOptions
}

// NewCachingProvider wraps inner with cache.
func NewCachingProvider(inner output.FeatureProvider, cache output.Cache, clock output.Clock, opts CachingOptions) *CachingProvider {
	return &CachingProvider{inner: inner, cache: cache, clock: clock, opts: opts}
}

func (c *CachingProvider) ID() string                  { return c.inner.ID() }
func (c *CachingProvider) Kind() string                { return c.inner.Kind() }
func (c *CachingProvider) Attribution() domain.License { return c.inner.Attribution() }

// Fetch returns a cached feature when present, else calls the inner provider and
// caches the result with a maturity-based TTL. Errors are never cached.
func (c *CachingProvider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	key := CacheKey(c.inner.ID(), c.opts.Version, req, c.opts.LatLonPrecision)

	if raw, ok, err := c.cache.Get(ctx, key); err == nil && ok {
		var f domain.Feature
		if json.Unmarshal(raw, &f) == nil {
			return domain.ProviderResult{Feature: f, Cached: true}, nil
		}
	}

	res, err := c.inner.Fetch(ctx, req)
	if err != nil {
		return domain.ProviderResult{}, err
	}

	if raw, mErr := json.Marshal(res.Feature); mErr == nil {
		_ = c.cache.Set(ctx, key, raw, c.ttlFor(req.Instant))
	}
	res.Cached = false
	return res, nil
}

func (c *CachingProvider) ttlFor(instant time.Time) time.Duration {
	if c.clock.Now().UTC().Sub(instant) >= c.opts.ArchiveDelay {
		return c.opts.MatureTTL
	}
	return c.opts.ImmatureTTL
}

// CacheKey derives a deterministic key. Coordinates are rounded to precision
// decimals so nearby points share the coarse weather grid cell (better hit rate);
// determinism preserves idempotency.
func CacheKey(providerID, version string, req domain.QueryRequest, precision int) string {
	lat := roundTo(req.Coordinate.Lat, precision)
	lon := roundTo(req.Coordinate.Lon, precision)
	raw := fmt.Sprintf("%s|%s|%.*f|%.*f|%s",
		providerID, version, precision, lat, precision, lon, req.Instant.UTC().Format(time.RFC3339))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func roundTo(v float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	// round half away from zero
	if v >= 0 {
		return float64(int64(v*p+0.5)) / p
	}
	return float64(int64(v*p-0.5)) / p
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/application/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add caching provider decorator with maturity-based TTL"
```

---

### Task 10: Feature service (fan-out + assembly)

**Files:**
- Create: `internal/application/featureservice.go`
- Test: `internal/application/featureservice_test.go`

**Interfaces:**
- Consumes: `Registry`, `output.Clock`, `output.ProviderError`, `domain.*`.
- Produces: `application.FeatureService` (implements `input.FeatureService`); `application.NewFeatureService(reg *Registry, logger *slog.Logger, timeout time.Duration)`.
- Behavior: selects providers (`req.Providers` filter or all), fetches concurrently, maps each outcome to a `domain.ProviderStatus`, collects successful features, builds `QueryEcho` (with `LocalTime` from `req.Timezone`), returns 200-shaped result even when all providers fail. Provider ordering in output follows registry order. Unknown (non-`ProviderError`) errors map to `unavailable` + `retryable:true` (conservative, logged).

- [ ] **Step 1: Write the failing tests**

Create `internal/application/featureservice_test.go`:

```go
package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func okProv(id string) output.FeatureProvider {
	return okProvider{id: id, feat: domain.NewPointFeature(
		domain.Coordinate{Lat: 1, Lon: 2}, map[string]any{"v": 1.0}, domain.License{Name: id, Attribution: "by " + id})}
}

type okProvider struct {
	id   string
	feat domain.Feature
}

func (p okProvider) ID() string                  { return p.id }
func (p okProvider) Kind() string                { return "weather" }
func (p okProvider) Attribution() domain.License { return p.feat.License }
func (p okProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{Feature: p.feat}, nil
}

type failProvider struct {
	id  string
	err error
}

func (p failProvider) ID() string                  { return p.id }
func (p failProvider) Kind() string                { return "weather" }
func (p failProvider) Attribution() domain.License { return domain.License{} }
func (p failProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, p.err
}

func sampleReq() domain.QueryRequest {
	loc, _ := time.LoadLocation("Europe/Berlin")
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93},
		Instant:    time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC),
		Timezone:   loc, TimezoneID: "Europe/Berlin",
	}
}

func TestFeatureService_PartialFailure(t *testing.T) {
	reg := NewRegistry()
	reg.Register(okProv("open-meteo"))
	reg.Register(failProvider{"astro", output.NewTransientError(errors.New("dial timeout"), 30*time.Second)})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error on provider failure: %v", err)
	}
	if len(res.Features) != 1 || res.Features[0].License.Attribution == "" {
		t.Fatalf("want 1 attributed feature, got %+v", res.Features)
	}
	if len(res.Providers) != 2 {
		t.Fatalf("want 2 provider statuses, got %d", len(res.Providers))
	}
	byID := map[string]domain.ProviderStatus{}
	for _, p := range res.Providers {
		byID[p.ID] = p
	}
	if byID["open-meteo"].Status != domain.StatusOK {
		t.Errorf("open-meteo status = %q", byID["open-meteo"].Status)
	}
	if s := byID["astro"]; s.Status != domain.StatusUnavailable || !s.Retryable || s.RetryAfter == "" {
		t.Errorf("astro status = %+v, want unavailable+retryable+retryAfter", s)
	}
	// localTime echoes the target instant in Berlin (CEST = +02:00).
	if res.Query.LocalTime != "2025-06-15T15:00:00+02:00" {
		t.Errorf("localTime = %q", res.Query.LocalTime)
	}
}

func TestFeatureService_AllFailStill200Shape(t *testing.T) {
	reg := NewRegistry()
	reg.Register(failProvider{"open-meteo", output.NewNotYetAvailableError(2 * time.Hour)})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error: %v", err)
	}
	if len(res.Features) != 0 {
		t.Errorf("want 0 features, got %d", len(res.Features))
	}
	if !res.Providers[0].Retryable {
		t.Error("not-yet-available must be retryable")
	}
}

func TestFeatureService_ProviderFilter(t *testing.T) {
	reg := NewRegistry()
	reg.Register(okProv("open-meteo"))
	reg.Register(okProv("astro"))
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	r := sampleReq()
	r.Providers = []string{"astro"}
	res, _ := svc.Query(context.Background(), r)
	if len(res.Providers) != 1 || res.Providers[0].ID != "astro" {
		t.Fatalf("filter ignored: %+v", res.Providers)
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/application/ -run TestFeatureService -v`
Expected: FAIL (undefined `NewFeatureService`).

- [ ] **Step 3: Implement**

Create `internal/application/featureservice.go`:

```go
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// FeatureService orchestrates providers and assembles the response envelope.
type FeatureService struct {
	registry *Registry
	logger   *slog.Logger
	timeout  time.Duration
}

// NewFeatureService builds the service.
func NewFeatureService(reg *Registry, logger *slog.Logger, timeout time.Duration) *FeatureService {
	return &FeatureService{registry: reg, logger: logger, timeout: timeout}
}

// Query fetches from the selected providers concurrently and assembles the
// result. Provider failures are encoded in Providers[]; the call itself only
// errors on a caller-canceled context.
func (s *FeatureService) Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error) {
	providers := s.selected(req)

	type outcome struct {
		feature *domain.Feature
		status  domain.ProviderStatus
	}
	outcomes := make([]outcome, len(providers))

	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p output.FeatureProvider) {
			defer wg.Done()
			fctx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			outcomes[i] = s.fetchOne(fctx, p, req)
		}(i, p)
	}
	wg.Wait()

	res := domain.QueryResult{
		Query:     s.echo(req),
		Features:  []domain.Feature{},
		Providers: make([]domain.ProviderStatus, 0, len(outcomes)),
	}
	for _, o := range outcomes {
		if o.feature != nil {
			res.Features = append(res.Features, *o.feature)
		}
		res.Providers = append(res.Providers, o.status)
	}
	return res, nil
}

func (s *FeatureService) selected(req domain.QueryRequest) []output.FeatureProvider {
	if len(req.Providers) == 0 {
		return s.registry.All()
	}
	var out []output.FeatureProvider
	for _, id := range req.Providers {
		if p, ok := s.registry.Get(id); ok {
			out = append(out, p)
		}
	}
	return out
}

func (s *FeatureService) fetchOne(ctx context.Context, p output.FeatureProvider, req domain.QueryRequest) (o struct {
	feature *domain.Feature
	status  domain.ProviderStatus
}) {
	res, err := p.Fetch(ctx, req)
	if err == nil {
		f := res.Feature
		o.feature = &f
		o.status = domain.ProviderStatus{ID: p.ID(), Kind: p.Kind(), Status: domain.StatusOK, Cached: res.Cached}
		return o
	}
	o.status = s.statusFor(p, err)
	return o
}

func (s *FeatureService) statusFor(p output.FeatureProvider, err error) domain.ProviderStatus {
	st := domain.ProviderStatus{ID: p.ID(), Kind: p.Kind(), Error: err.Error()}
	pe, ok := output.AsProviderError(err)
	if !ok {
		// Unknown error: be conservative and let the client retry.
		s.logger.Warn("unclassified provider error", "provider", p.ID(), "error", err)
		st.Status = domain.StatusUnavailable
		st.Retryable = true
		return st
	}
	switch pe.Class {
	case output.ClassPermanent:
		st.Status = domain.StatusError
		st.Retryable = false
	default: // transient, not-yet-available
		st.Status = domain.StatusUnavailable
		st.Retryable = true
	}
	if pe.RetryAfter > 0 {
		st.RetryAfter = pe.RetryAfter.String()
	}
	return st
}

func (s *FeatureService) echo(req domain.QueryRequest) domain.QueryEcho {
	return domain.QueryEcho{
		Coordinate: req.Coordinate,
		Datetime:   req.Instant.UTC().Format(time.RFC3339),
		Timezone:   req.TimezoneID,
		LocalTime:  req.Instant.In(req.Timezone).Format(time.RFC3339),
	}
}
```

Add `"sync"` to the import block.

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/application/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add feature service with concurrent fan-out and retry coding"
```

---

## Phase D — Cache & provider adapters

### Task 11: In-memory cache

**Files:**
- Create: `internal/adapters/cache/memory/memory.go`
- Test: `internal/adapters/cache/memory/memory_test.go`

**Interfaces:**
- Produces: `memory.New() *Cache` implementing `output.Cache`. `ttl == 0` ⇒ never expires; expired entries return `(nil, false, nil)`.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/cache/memory/memory_test.go`:

```go
package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetGetExpire(t *testing.T) {
	c := New()
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("get miss: %q ok=%v", v, ok)
	}
	c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Error("entry should have expired")
	}
	// ttl 0 = permanent
	_ = c.Set(ctx, "p", []byte("x"), 0)
	c.now = func() time.Time { return time.Now().Add(1000 * time.Hour) }
	if _, ok, _ := c.Get(ctx, "p"); !ok {
		t.Error("ttl=0 entry must not expire")
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/adapters/cache/memory/ -v`
Expected: FAIL (undefined `New`).

- [ ] **Step 3: Implement**

Create `internal/adapters/cache/memory/memory.go`:

```go
// Package memory is an in-process output.Cache (used for tests and dev).
package memory

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	value   []byte
	expires time.Time // zero = never
}

// Cache is a concurrency-safe in-memory cache.
type Cache struct {
	mu  sync.RWMutex
	m   map[string]entry
	now func() time.Time
}

// New returns an empty cache.
func New() *Cache {
	return &Cache{m: map[string]entry{}, now: time.Now}
}

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expires.IsZero() && c.now().After(e.expires) {
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		return nil, false, nil
	}
	return e.value, true, nil
}

// Set stores value under key. ttl == 0 means never expire.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	c.mu.Lock()
	c.m[key] = entry{value: value, expires: exp}
	c.mu.Unlock()
	return nil
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/adapters/cache/memory/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add in-memory cache adapter"
```

---

### Task 12: BoltDB disk cache

**Files:**
- Create: `internal/adapters/cache/bolt/bolt.go`
- Test: `internal/adapters/cache/bolt/bolt_test.go`

**Interfaces:**
- Produces: `bolt.Open(path string) (*Cache, error)`, `(*Cache).Close() error`, implementing `output.Cache`. Stored value = 8-byte big-endian unix-nano expiry (0 = never) + payload. Expired entries are treated as absent (lazy delete).

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/cache/bolt/bolt_test.go`:

```go
package bolt

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestBoltCache_Roundtrip(t *testing.T) {
	c, err := Open(filepath.Join(t.TempDir(), "c.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("miss: %q ok=%v", v, ok)
	}
	if err := c.Set(ctx, "gone", []byte("x"), -time.Hour); err != nil { // already expired
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, "gone"); ok {
		t.Error("expired entry must read as absent")
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/adapters/cache/bolt/ -v`
Expected: FAIL (undefined `Open`).

- [ ] **Step 3: Implement**

Create `internal/adapters/cache/bolt/bolt.go`:

```go
// Package bolt is a persistent output.Cache backed by a single BoltDB file.
package bolt

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("cache")

// Cache is a BoltDB-backed cache.
type Cache struct {
	db *bolt.DB
}

// Open opens (creating parent dirs and the bucket as needed) the cache at path.
func Open(path string) (*Cache, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(bucket)
		return e
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

// Close closes the underlying database.
func (c *Cache) Close() error { return c.db.Close() }

// Get returns the value if present and unexpired.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	var (
		out    []byte
		found  bool
		expired bool
	)
	err := c.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucket).Get([]byte(key))
		if raw == nil || len(raw) < 8 {
			return nil
		}
		found = true
		expNano := int64(binary.BigEndian.Uint64(raw[:8]))
		if expNano != 0 && time.Now().UnixNano() > expNano {
			expired = true
			return nil
		}
		out = append(out, raw[8:]...)
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if expired {
		_ = c.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucket).Delete([]byte(key)) })
		return nil, false, nil
	}
	if !found {
		return nil, false, nil
	}
	return out, true, nil
}

// Set stores value under key. ttl == 0 means never expire.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	var expNano int64
	if ttl > 0 {
		expNano = time.Now().Add(ttl).UnixNano()
	}
	buf := make([]byte, 8+len(value))
	binary.BigEndian.PutUint64(buf[:8], uint64(expNano))
	copy(buf[8:], value)
	return c.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Put([]byte(key), buf)
	})
}
```

- [ ] **Step 4: Run to pass**

Run: `go test ./internal/adapters/cache/bolt/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat: add BoltDB disk cache adapter"
```

---

### Task 13: Open-Meteo weather provider

**Files:**
- Create: `internal/adapters/openmeteo/openmeteo.go`
- Test: `internal/adapters/openmeteo/openmeteo_test.go`, `internal/adapters/openmeteo/testdata/archive_ok.json`

**Interfaces:**
- Consumes: `output.Clock`, `output.NewTransientError`/`NewNotYetAvailableError`/`NewPermanentError`, `domain.*`.
- Produces: `openmeteo.New(opts Options) *Provider` implementing `output.FeatureProvider`; `openmeteo.Options{ArchiveBaseURL, ForecastBaseURL string; Timeout, ArchiveDelay time.Duration; Clock output.Clock; HTTPClient *http.Client}`.
- Behavior: `ID()="open-meteo"`, `Kind()="weather"`. Endpoint chosen by data age (older than `ArchiveDelay` ⇒ archive, else forecast with `past_days`). Selects the hourly index whose timestamp equals `req.Instant` (UTC). Missing/null value ⇒ `NewNotYetAvailableError`. Network/5xx/429 ⇒ `NewTransientError`. Other 4xx ⇒ `NewPermanentError`. Feature geometry uses the response's `latitude`/`longitude`; attribution reflects the endpoint used.

- [ ] **Step 1: Add a fixture**

Create `internal/adapters/openmeteo/testdata/archive_ok.json`:

```json
{
  "latitude": 49.8,
  "longitude": 9.94,
  "hourly_units": {"temperature_2m": "°C", "wind_speed_10m": "km/h", "precipitation": "mm"},
  "hourly": {
    "time": ["2025-06-15T12:00", "2025-06-15T13:00", "2025-06-15T14:00"],
    "temperature_2m": [20.1, 21.4, 22.0],
    "relative_humidity_2m": [58, 55, 52],
    "precipitation": [0.0, 0.0, 0.1],
    "weather_code": [2, 3, 61],
    "wind_speed_10m": [10.0, 12.1, 12.5],
    "cloud_cover": [60, 75, 90],
    "is_day": [1, 1, 1]
  }
}
```

- [ ] **Step 2: Write the failing tests**

Create `internal/adapters/openmeteo/openmeteo_test.go`:

```go
package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func req(instant time.Time) domain.QueryRequest {
	loc, _ := time.LoadLocation("Europe/Berlin")
	return domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93}, Instant: instant, Timezone: loc, TimezoneID: "Europe/Berlin"}
}

func newProvider(t *testing.T, handler http.HandlerFunc) (*Provider, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	p := New(Options{
		ArchiveBaseURL:  srv.URL,
		ForecastBaseURL: srv.URL,
		Timeout:         2 * time.Second,
		ArchiveDelay:    5 * 24 * time.Hour,
		Clock:           fixedClock{time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
	})
	return p, srv.Close
}

func TestFetch_SelectsHourAndAttributes(t *testing.T) {
	body, _ := os.ReadFile("testdata/archive_ok.json")
	p, done := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("latitude") == "" {
			t.Errorf("missing latitude param: %s", r.URL.RawQuery)
		}
		_, _ = w.Write(body)
	})
	defer done()

	res, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := res.Feature.Properties["temperature2m"]; got != 21.4 {
		t.Errorf("temperature2m = %v, want 21.4", got)
	}
	if res.Feature.Properties["isDay"] != true {
		t.Errorf("isDay = %v, want true", res.Feature.Properties["isDay"])
	}
	// geometry uses the provider-resolved grid cell.
	if c := res.Feature.Geometry.Coordinates; c[0] != 9.94 || c[1] != 49.8 {
		t.Errorf("geometry = %v, want [9.94 49.8]", c)
	}
	if res.Feature.License.Attribution == "" || res.Feature.License.Name == "" {
		t.Error("feature must carry attribution")
	}
}

func TestFetch_MissingHourIsNotYetAvailable(t *testing.T) {
	p, done := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"latitude":49.8,"longitude":9.94,"hourly_units":{},"hourly":{"time":["2025-06-15T13:00"],"temperature_2m":[null]}}`))
	})
	defer done()
	_, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassNotYetAvailable {
		t.Fatalf("want not-yet-available, got %v", err)
	}
}

func TestFetch_ServerErrorIsTransient(t *testing.T) {
	p, done := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	defer done()
	_, err := p.Fetch(context.Background(), req(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassTransient {
		t.Fatalf("want transient, got %v", err)
	}
}
```

- [ ] **Step 3: Run to fail**

Run: `go test ./internal/adapters/openmeteo/ -v`
Expected: FAIL (undefined `New`/`Provider`).

- [ ] **Step 4: Implement**

Create `internal/adapters/openmeteo/openmeteo.go`:

```go
// Package openmeteo implements the weather FeatureProvider using Open-Meteo.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const (
	providerID   = "open-meteo"
	providerKind = "weather"
	licenseName  = "CC-BY 4.0"
	licenseURL   = "https://open-meteo.com/en/license"
)

// hourlyVars are requested from Open-Meteo, mapped to output property names.
var hourlyVars = []struct{ api, prop string }{
	{"temperature_2m", "temperature2m"},
	{"relative_humidity_2m", "relativeHumidity2m"},
	{"precipitation", "precipitation"},
	{"weather_code", "weatherCode"},
	{"wind_speed_10m", "windSpeed10m"},
	{"cloud_cover", "cloudCover"},
	{"is_day", "isDay"},
}

// Options configures the provider.
type Options struct {
	ArchiveBaseURL  string
	ForecastBaseURL string
	Timeout         time.Duration
	ArchiveDelay    time.Duration
	Clock           output.Clock
	HTTPClient      *http.Client
}

// Provider is the Open-Meteo weather provider.
type Provider struct {
	archiveBaseURL  string
	forecastBaseURL string
	archiveDelay    time.Duration
	clock           output.Clock
	client          *http.Client
}

// New builds the provider.
func New(opts Options) *Provider {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}
	return &Provider{
		archiveBaseURL:  opts.ArchiveBaseURL,
		forecastBaseURL: opts.ForecastBaseURL,
		archiveDelay:    opts.ArchiveDelay,
		clock:           opts.Clock,
		client:          client,
	}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the base license; Fetch may refine the attribution text.
func (p *Provider) Attribution() domain.License {
	return domain.License{Name: licenseName, URL: licenseURL, Attribution: "Weather data by Open-Meteo.com"}
}

type apiResponse struct {
	Latitude    float64                    `json:"latitude"`
	Longitude   float64                    `json:"longitude"`
	HourlyUnits map[string]string          `json:"hourly_units"`
	Hourly      map[string]json.RawMessage `json:"hourly"`
}

// Fetch retrieves the weather for the request's hour.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	useArchive := p.clock.Now().UTC().Sub(req.Instant) >= p.archiveDelay
	endpoint := p.forecastBaseURL
	if useArchive {
		endpoint = p.archiveBaseURL
	}

	u, err := p.buildURL(endpoint, req, useArchive)
	if err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return domain.ProviderResult{}, output.NewTransientError(err, 30*time.Second)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return domain.ProviderResult{}, output.NewTransientError(
			fmt.Errorf("open-meteo status %d", resp.StatusCode), retryAfter(resp))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return domain.ProviderResult{}, output.NewPermanentError(fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, b))
	}

	var data apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}
	return p.toFeature(data, req, useArchive)
}

func (p *Provider) buildURL(base string, req domain.QueryRequest, useArchive bool) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", req.Coordinate.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", req.Coordinate.Lon))
	q.Set("timezone", "UTC")
	names := make([]string, len(hourlyVars))
	for i, v := range hourlyVars {
		names[i] = v.api
	}
	q.Set("hourly", joinComma(names))
	day := req.Instant.UTC().Format("2006-01-02")
	if useArchive {
		q.Set("start_date", day)
		q.Set("end_date", day)
	} else {
		// forecast endpoint: pull recent past so the target hour is covered.
		q.Set("past_days", "7")
		q.Set("forecast_days", "1")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Provider) toFeature(data apiResponse, req domain.QueryRequest, useArchive bool) (domain.ProviderResult, error) {
	var times []string
	if raw, ok := data.Hourly["time"]; ok {
		_ = json.Unmarshal(raw, &times)
	}
	target := req.Instant.UTC().Format("2006-01-02T15:04")
	idx := indexOf(times, target)
	if idx < 0 {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(2 * time.Hour)
	}

	props := map[string]any{
		"provider":   providerID,
		"kind":       providerKind,
		"observedAt": req.Instant.UTC().Format(time.RFC3339),
		"localTime":  req.Instant.In(req.Timezone).Format(time.RFC3339),
	}
	units := map[string]string{}
	valueMissing := false
	for _, v := range hourlyVars {
		raw, ok := data.Hourly[v.api]
		if !ok {
			continue
		}
		var vals []*float64
		if json.Unmarshal(raw, &vals) != nil || idx >= len(vals) || vals[idx] == nil {
			if v.api == "temperature_2m" { // primary variable: null ⇒ not ready
				valueMissing = true
			}
			continue
		}
		props[v.prop] = normalize(v.api, *vals[idx])
		if unit, ok := data.HourlyUnits[v.api]; ok {
			units[v.prop] = unit
		}
	}
	if valueMissing {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(2 * time.Hour)
	}
	props["units"] = units

	feat := domain.NewPointFeature(
		domain.Coordinate{Lat: data.Latitude, Lon: data.Longitude},
		props,
		p.license(useArchive),
	)
	return domain.ProviderResult{Feature: feat}, nil
}

func (p *Provider) license(useArchive bool) domain.License {
	src := "GFS/ICON forecast models"
	if useArchive {
		src = "ERA5 (Copernicus Climate Change Service / ECMWF)"
	}
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; " + src,
	}
}

// normalize converts is_day (0/1) to bool; weather_code to int; leaves the rest.
func normalize(apiName string, v float64) any {
	switch apiName {
	case "is_day":
		return v != 0
	case "weather_code", "relative_humidity_2m", "cloud_cover":
		return int(v)
	default:
		return v
	}
}

func retryAfter(resp *http.Response) time.Duration {
	if s := resp.Header.Get("Retry-After"); s != "" {
		if secs, err := time.ParseDuration(s + "s"); err == nil {
			return secs
		}
	}
	return 30 * time.Second
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
```

- [ ] **Step 5: Run to pass**

Run: `go test ./internal/adapters/openmeteo/ -v`
Expected: PASS.

Note on `weather_code`/`cloud_cover` int conversion: the fixture test asserts `temperature2m == 21.4` (float) and `isDay == true` (bool). If you add assertions on `weatherCode`, expect `int(3)`.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: add Open-Meteo weather provider"
```

---

## Phase E — HTTP adapter & composition

### Task 14: HTTP server (query + providers)

**Files:**
- Create: `internal/adapters/http/server.go` (from template, adapted)
- Test: `internal/adapters/http/server_test.go`

**Interfaces:**
- Consumes: `input.FeatureService`, `input.ProviderLister`, `input.HealthChecker`, `output.Clock`, `domain.ParseQueryRequest`, `domain.ValidationError`.
- Produces: `httpapi.NewServer(addr string, features input.FeatureService, providers input.ProviderLister, health input.HealthChecker, clock output.Clock, logger *slog.Logger, opts Options) *Server`; routes `GET /api/v1/query`, `GET /api/v1/providers`, health, `/openapi.json`, `/docs`; `(*Server).Router()`, `Start()`, `Shutdown(ctx)`.

- [ ] **Step 1: Copy the template and adapt**

```bash
cp claude-skills/skills/new-go-service/templates/http_server.go internal/adapters/http/server.go
sed -i '' 's/<module>/github.com\/jobrunner\/tempus/g; s/<svc>/tempus/g' internal/adapters/http/server.go
```

Then edit `internal/adapters/http/server.go`:
1. Replace the `items input.ItemService` field with `features input.FeatureService`, `providers input.ProviderLister`, and add `clock output.Clock`. Update `NewServer` signature accordingly and its callers below.
2. Replace the `/items` routes with:
```go
	api.HandleFunc("/query", s.handleQuery).Methods(http.MethodGet)
	api.HandleFunc("/providers", s.handleProviders).Methods(http.MethodGet)
```
3. Delete the demo `handleListItems`/`handleGetItem`/`handleCreateItem` handlers.
4. Add the import `"github.com/jobrunner/tempus/internal/domain"`, `"github.com/jobrunner/tempus/internal/ports/output"`, `"errors"`, and `"time"` (already present).
5. Append the new handlers:

```go
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var providers []string
	if p := q.Get("providers"); p != "" {
		providers = strings.Split(p, ",")
	}
	req, err := domain.ParseQueryRequest(
		q.Get("lat"), q.Get("lon"), q.Get("datetime"), q.Get("timezone"), providers, s.clock.Now(),
	)
	if err != nil {
		var ve domain.ValidationError
		if errors.As(err, &ve) {
			s.writeError(w, http.StatusBadRequest, ve.Error())
			return
		}
		s.writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	result, err := s.features.Query(r.Context(), req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.logger.Debug("query canceled by client")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"providers": s.providers.Providers(r.Context())})
}
```

Add `"strings"` and `"context"` to the imports if not present. Update `NewServer` body to store the new fields; update `setupRoutes` uses accordingly.

- [ ] **Step 2: Write the failing tests**

Create `internal/adapters/http/server_test.go`:

```go
package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

type stubFeatures struct{ res domain.QueryResult }

func (s stubFeatures) Query(context.Context, domain.QueryRequest) (domain.QueryResult, error) {
	return s.res, nil
}

type stubProviders struct{}

func (stubProviders) Providers(context.Context) []input.ProviderInfo {
	return []input.ProviderInfo{{ID: "open-meteo", Kind: "weather", License: domain.License{Name: "CC-BY 4.0"}}}
}

type stubHealth struct{}

func (stubHealth) Ready(context.Context) bool { return true }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) }

func testServer() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	res := domain.QueryResult{Features: []domain.Feature{}, Providers: []domain.ProviderStatus{{ID: "open-meteo", Status: "ok"}}}
	return NewServer(":0", stubFeatures{res}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{})
}

func TestHandleQuery_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z&timezone=Europe/Berlin", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body domain.QueryResult
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Providers) != 1 {
		t.Errorf("providers = %d, want 1", len(body.Providers))
	}
}

func TestHandleQuery_FutureIs400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=0&lon=0&datetime=2026-07-21T13:00:00Z&timezone=UTC", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleProviders_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/adapters/http/ -run 'TestHandleQuery|TestHandleProviders' -v`
Expected: PASS. (The contract test comes in Task 15 and will fail until then — run only these named tests here.)

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add HTTP query and providers endpoints"
```

---

### Task 15: OpenAPI spec, embed, contract test

**Files:**
- Create: `internal/adapters/http/openapi.yaml`, `api/openapi/openapi.yaml` (byte-identical copy)
- Create: `internal/adapters/http/openapi.go` (from template)
- Create: `internal/adapters/http/contract_test.go` (from template, adapted fakes)

**Interfaces:**
- Consumes: routes registered in Task 14.
- Produces: `/openapi.json`, `/docs`; `TestRoutesMatchOpenAPISpec` passing for the real surface.

- [ ] **Step 1: Write the OpenAPI spec for the real routes**

Create `internal/adapters/http/openapi.yaml`:

```yaml
openapi: 3.0.3
info:
  title: tempus API
  version: 0.1.0
  description: Coordinate + time feature queries with per-feature attribution.
paths:
  /api/v1/query:
    get:
      summary: Query cached feature providers for a coordinate and time
      parameters:
        - {name: lat, in: query, required: true, schema: {type: number}}
        - {name: lon, in: query, required: true, schema: {type: number}}
        - {name: datetime, in: query, required: true, schema: {type: string}, description: RFC3339 or YYYY-MM-DDTHH:MM[:SS]; no offset ⇒ UTC; must not be in the future}
        - {name: timezone, in: query, required: true, schema: {type: string}, description: IANA timezone id}
        - {name: providers, in: query, required: false, schema: {type: string}, description: comma-separated provider ids}
      responses:
        "200": {description: Feature collection with per-provider status, content: {application/json: {schema: {$ref: "#/components/schemas/QueryResult"}}}}
        "400": {description: Invalid request, content: {application/json: {schema: {$ref: "#/components/schemas/Error"}}}}
  /api/v1/providers:
    get:
      summary: List available providers and their attribution
      responses:
        "200": {description: Provider list, content: {application/json: {schema: {type: object}}}}
components:
  schemas:
    Error:
      type: object
      properties: {error: {type: string}, message: {type: string}}
    License:
      type: object
      properties: {name: {type: string}, url: {type: string}, attribution: {type: string}}
    Feature:
      type: object
      properties:
        type: {type: string}
        geometry: {type: object}
        properties: {type: object}
        license: {$ref: "#/components/schemas/License"}
    ProviderStatus:
      type: object
      properties:
        id: {type: string}
        kind: {type: string}
        status: {type: string, enum: [ok, unavailable, error]}
        cached: {type: boolean}
        retryable: {type: boolean}
        retryAfter: {type: string}
        error: {type: string}
    QueryResult:
      type: object
      properties:
        query: {type: object}
        features: {type: array, items: {$ref: "#/components/schemas/Feature"}}
        providers: {type: array, items: {$ref: "#/components/schemas/ProviderStatus"}}
```

- [ ] **Step 2: Copy the embed + contract templates**

```bash
cp claude-skills/skills/new-go-service/templates/openapi_embed.go internal/adapters/http/openapi.go
cp claude-skills/skills/new-go-service/templates/contract_test.go internal/adapters/http/contract_test.go
sed -i '' 's/<module>/github.com\/jobrunner\/tempus/g; s/<svc>/tempus/g' internal/adapters/http/openapi.go internal/adapters/http/contract_test.go
cp internal/adapters/http/openapi.yaml api/openapi/openapi.yaml
```
(Create `api/openapi/` first: `mkdir -p api/openapi`.)

- [ ] **Step 3: Adapt the contract test's fakes**

Edit `internal/adapters/http/contract_test.go`: replace `newContractTestServer` and the `fakeItemService`/`fakeHealth` types so it builds our `Server`:

```go
func newContractTestServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{})
}
```
Delete the `fakeItemService`/`fakeHealth` types and their `input` import (the stubs from `server_test.go` in the same package are reused). Add `"io"` to imports.

- [ ] **Step 4: Run the contract test + full http package**

Run: `go test ./internal/adapters/http/ -v`
Expected: PASS, including `TestRoutesMatchOpenAPISpec`. If it reports a route not documented (or vice-versa), reconcile `openapi.yaml` with the registered routes.

- [ ] **Step 5: Keep the mirror in sync + commit**

```bash
cp internal/adapters/http/openapi.yaml api/openapi/openapi.yaml
git add -A && git commit -m "feat: add OpenAPI spec, embed, and routes-contract test"
```

---

### Task 16: Composition root & main wiring

**Files:**
- Modify: `internal/app/app.go` (replace the Task 1 stub)
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces: `app.New(cfg *config.Config, logger *slog.Logger) (*App, error)` that builds cache (by `cfg.Cache.Type`), the Open-Meteo provider wrapped in `CachingProvider`, the registry, the `FeatureService`, and the HTTP server; `(*App).Run(ctx) error` (serve + graceful shutdown); `(*App).Handler() http.Handler` for tests; readiness via a `HealthChecker`.

- [ ] **Step 1: Write the failing test**

Create `internal/app/app_test.go`:

```go
package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jobrunner/tempus/internal/config"
)

func TestApp_QueryEndToEndWithMemoryCache(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Cache.Type = "memory"
	cfg.Cache.Path = filepath.Join(t.TempDir(), "c.bolt")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	// providers endpoint proves wiring without hitting the network.
	resp, err := http.Get(srv.URL + "/api/v1/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("providers status = %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run to fail**

Run: `go test ./internal/app/ -v`
Expected: FAIL (`Handler` undefined, `New` stub returns no wiring).

- [ ] **Step 3: Implement the composition root**

Replace `internal/app/app.go`:

```go
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	httpapi "github.com/jobrunner/tempus/internal/adapters/http"
	boltcache "github.com/jobrunner/tempus/internal/adapters/cache/bolt"
	memcache "github.com/jobrunner/tempus/internal/adapters/cache/memory"
	"github.com/jobrunner/tempus/internal/adapters/clock"
	"github.com/jobrunner/tempus/internal/adapters/openmeteo"
	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/config"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// App is the composition root: it owns adapters and the server lifecycle.
type App struct {
	cfg      *config.Config
	logger   *slog.Logger
	server   *httpapi.Server
	closers  []func() error
}

type readyAlways struct{}

func (readyAlways) Ready(context.Context) bool { return true }

// New wires adapters into ports.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	a := &App{cfg: cfg, logger: logger}

	cache, closer, err := buildCache(cfg.Cache)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		a.closers = append(a.closers, closer)
	}

	clk := clock.System{}
	registry := application.NewRegistry()

	if cfg.Providers.OpenMeteo.Enabled {
		om := openmeteo.New(openmeteo.Options{
			ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
			ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
			Timeout:         cfg.Providers.OpenMeteo.Timeout,
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			Clock:           clk,
		})
		cached := application.NewCachingProvider(om, cache, clk, application.CachingOptions{
			Version:         "1",
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			MatureTTL:       365 * 24 * time.Hour,
			ImmatureTTL:     time.Hour,
			LatLonPrecision: 2,
		})
		registry.Register(cached)
	}

	features := application.NewFeatureService(registry, logger, cfg.Query.Timeout)
	addr := cfg.Server.Host + ":" + itoa(cfg.Server.Port)
	a.server = httpapi.NewServer(addr, features, registry, readyAlways{}, clk, logger, httpapi.Options{ServiceName: "tempus"})
	return a, nil
}

func buildCache(cfg config.CacheConfig) (output.Cache, func() error, error) {
	switch cfg.Type {
	case "memory":
		return memcache.New(), nil, nil
	default: // "disk"
		c, err := boltcache.Open(cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		return c, c.Close, nil
	}
}

// Handler exposes the router for tests.
func (a *App) Handler() http.Handler { return a.server.Router() }

// Run starts the server and shuts down gracefully on ctx cancellation.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := a.server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
		defer cancel()
		err := a.server.Shutdown(shutCtx)
		for _, c := range a.closers {
			_ = c()
		}
		return err
	}
}

func itoa(i int) string { return strconv.Itoa(i) }
```

Add `"strconv"` to imports (or replace `itoa` with `strconv.Itoa` inline and drop the helper).

- [ ] **Step 4: Run to pass + full build/test**

Run: `go test ./... && go build ./...`
Expected: PASS + build succeeds.

- [ ] **Step 5: Smoke-run the binary**

```bash
TEMPUS_CACHE_TYPE=memory go run ./cmd/tempus &
sleep 1
curl -s "http://localhost:8080/api/v1/providers" | head -c 300
curl -s "http://localhost:8080/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z&timezone=Europe/Berlin" | head -c 600
kill %1
```
Expected: providers JSON lists `open-meteo`; the query returns a feature with `license.attribution` set, or (if offline) `providers[0].status="unavailable", retryable=true`. Either is a valid, correctly-shaped response.

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "feat: wire composition root and end-to-end query path"
```

---

## Phase F — Complete the quality harness

> These tasks scaffold the remaining `new-go-service` machinery. Each copies a concrete template/reference from `claude-skills/skills/new-go-service/` and runs its gate. Open the cited reference file before starting the task.

### Task 17: Ratchets (debt, coverage, mutation)

**Files:** Create `.debt-budget`, `debt-guard.sh`, `.coverage-floors`, `coverage-gate.sh`, `gremlins.yaml`; add Makefile targets if not already present.

- [ ] **Step 1:** Copy the templates and reference:
```bash
cp claude-skills/skills/new-go-service/templates/debt-guard.sh debt-guard.sh
cp claude-skills/skills/new-go-service/templates/coverage-gate.sh coverage-gate.sh
cp claude-skills/skills/new-go-service/templates/gremlins.yaml gremlins.yaml
chmod +x debt-guard.sh coverage-gate.sh
```
Open `claude-skills/skills/new-go-service/reference/ratchets-and-harnesses.md` and follow the exact file formats.
- [ ] **Step 2:** Baseline `.debt-budget` at the current suppression count (`./debt-guard.sh` prints it) and confirm zero `TODO`/`FIXME`.
- [ ] **Step 3:** Run `go test ./... -coverprofile=coverage.out`, then write per-package floors into `.coverage-floors` at (or just below) current coverage; run `./coverage-gate.sh` → passes.
- [ ] **Step 4:** `make verify` green. Commit: `chore: add debt, coverage, and mutation ratchets`.

### Task 18: Observability wiring (OTel metrics + tracer adapter)

**Files:** Create `internal/adapters/telemetry/tracer.go` (OTel implementation of `output.Tracer`), `internal/adapters/metrics/metrics.go`; wire into `app.New` (tracing enabled by `cfg.Tracing.Enabled`, NoOp otherwise) and expose `/metrics` on `cfg.Metrics.Port`.

- [ ] Follow `claude-skills/skills/new-go-service/reference/observability.md`. Keep domain/application telemetry-free (only the `output.Tracer` port). Default is `NoOpTracer` so existing tests are unaffected. `make verify` green. Commit: `feat: add OpenTelemetry tracing and metrics adapters`.

### Task 19: Docs (MkDocs + OpenAPI mirror) & doc-drift gate

**Files:** Create `mkdocs.yml`, `docs/{tutorials,how-to,reference,explanation}/` with initial pages; add the doc-drift check that fails if `internal/adapters/http/openapi.yaml` and `api/openapi/openapi.yaml` differ.

- [ ] Copy `mkdocs.yml` guidance from `claude-skills/skills/new-go-service/reference/ci-and-release.md`; document the `/api/v1/query` contract (attribution requirement, retry semantics, no-future rule) under `reference/`. `make docs` (`mkdocs build --strict`) green. Commit: `docs: add MkDocs site and OpenAPI mirror gate`.

### Task 20: Claude hooks, dev env, CI/CD & release

**Files:** `.claude/settings.json` hooks + hook scripts; `Dockerfile` + `deploy/dev/`; `.github/workflows/*`; `release-please-config.json`; `.goreleaser.yml`; `.commitlintrc.yml`.

- [ ] **Step 1:** Copy `claude-skills/skills/new-go-service/templates/settings.json` into `.claude/settings.json` and the hook scripts; `make hooks`. Follow `reference/ci-and-release.md` and `reference/dev-environment.md`.
- [ ] **Step 2:** Add CI workflows (ci, mutation, commitlint, release-please, security), `.goreleaser.yml`, and the required-status-checks note. Substitute `tempus`/module path.
- [ ] **Step 3:** Push the branch and confirm every CI gate is green. Commit: `chore: add CI/CD, release pipeline, dev environment, and Claude hooks`.
- [ ] **Step 4:** Open a PR from `feat/weather-service` (see `superpowers:finishing-a-development-branch`).

---

## Self-Review

**Spec coverage:**
- Coordinate/datetime(UTC)/timezoneId intake + validation → Tasks 5, 14. ✅
- Calls external service, caches, returns collected → Tasks 10, 9, 13. ✅
- Weather first provider (Open-Meteo, REST) → Task 13. ✅
- No future weather → Tasks 5 (domain), 14 (400). ✅
- Attribution mandatory per feature → Tasks 4 (`License` on `Feature`), 13 (set), 10 (carried), plus `NewPointFeature` shape test. ✅
- Ortus-like feature queries, compute/fetch not PiP → Tasks 4, 10 (GeoJSON Feature + `providers[]`). ✅
- Unreachable service encoded for later idempotent retry → Tasks 6 (error taxonomy), 10 (`providers[].retryable`/`retryAfter`), always-200. ✅
- timezoneId = display/local context → Tasks 5, 10 (`LocalTime`), 13 (`localTime`, `isDay`). ✅
- Pluggable cache, disk default → Tasks 11, 12, 16. ✅
- Full harness → Tasks 1–3, 15, 17–20. ✅

**Placeholder scan:** No TBD/TODO in feature-logic tasks; harness Task 17–20 steps cite exact templates + reference files (concrete artifacts in-repo). ✅

**Type consistency:** `ProviderResult{Feature,Cached}`, `FeatureProvider.Fetch(...) (domain.ProviderResult, error)`, `NewServer(addr, features, providers, health, clock, logger, opts)`, `CacheKey(id, version, req, precision)`, `CachingOptions` fields — consistent across Tasks 4/6/9/10/13/14/16. ✅
