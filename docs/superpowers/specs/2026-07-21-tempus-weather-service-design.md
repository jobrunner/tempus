# tempus — Feature-Query Service (Design)

**Status:** Approved (2026-07-21)
**Module:** `github.com/jobrunner/tempus` · env prefix `TEMPUS_`

## Purpose

`tempus` is a Go backend service that takes a WGS84 coordinate, a UTC datetime, and
an IANA timezone id, calls one or more external/computational **feature providers**,
caches each provider's result, and returns the collected features to the caller — with
**mandatory per-feature attribution**. It is the compute/fetch analogue of *ortus*:
same feature-query shape, but instead of point-in-polygon it performs calculations or
calls external services.

The first (and MVP-only) provider is **weather** via **Open-Meteo**. Because the goal
is to describe the conditions *during a past field-sampling excursion*, weather for a
**future** datetime is rejected.

## Non-goals (YAGNI)

- No WeatherKit in the first iteration (design leaves room for it as a second provider
  behind the same port).
- No providers other than weather yet (registry is generic for later: astronomy,
  elevation, …).
- No user auth / rate limiting beyond what the harness ships.

## Core decisions (from brainstorming)

| Decision | Choice |
|---|---|
| Engineering harness | **Full** `new-go-service` harness (hexagonal, ratchets, fitness functions, OTel, MkDocs, release-please/goreleaser, per-feature dev containers, CI gates) |
| Providers (MVP) | **Open-Meteo only**; registry generic for more |
| Cache backend | **Pluggable `Cache` output-port**, **disk default** (embedded BoltDB); Redis/S3 later |
| Weather temporal resolution | **Exact hour** at the UTC instant (rounded to the full hour) |
| Partial/total failure | **Always HTTP 200** on valid input; per-provider status in `providers[]` |
| Future datetime | **400, not retryable**; past-but-not-yet-in-archive → provider `unavailable`, **retryable** |
| `timezoneId` role | Data selected at the UTC instant; tz **annotates** response with local time + day context (`is_day`) |
| API shape | **`GET /api/v1/query`** with query params (idempotent) |
| Core abstraction | **Provider registry + caching decorator** (Ortus-style registry) |

## Architecture (hexagonal)

```
cmd/tempus/main.go            thin entrypoint: flags → config → app.New → run → shutdown
internal/domain/              Coordinate, Instant, TimezoneID, Feature, License,
                              QueryRequest, QueryResult, ProviderStatus, error taxonomy
internal/ports/input/         FeatureService, HealthChecker
internal/ports/output/        FeatureProvider, Cache, Clock, Tracer
internal/application/         featureservice (fan-out + assembly), registry, caching decorator
internal/adapters/
  http/                       gorilla/mux server, /api/v1, OpenAPI embed + contract test
  openmeteo/                  Open-Meteo weather provider
  cache/                      bolt/ (disk default), memory/
  telemetry/, metrics/        OTel wiring
internal/app/app.go           composition root: build providers, wrap with cache, register
internal/config/config.go     TEMPUS_-prefixed env config
```

Dependency rule (enforced by golangci depguard): `domain` imports nothing internal;
`application` imports `domain` + `ports` only; `adapters` implement `ports`; `app`
is the only package that wires adapters into ports.

### Ports

```go
// input
type FeatureService interface {
    Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error)
}

// output
type FeatureProvider interface {
    ID() string
    Kind() string                 // e.g. "weather"
    Attribution() domain.License
    Fetch(ctx context.Context, req domain.QueryRequest) (domain.Feature, error)
}
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
type Clock interface{ Now() time.Time }   // testable "now" for future-date validation
```

### Registry + caching decorator

- `application.Registry` holds `map[id]FeatureProvider`, like the ortus source registry.
- `application.CachingProvider` decorates a `FeatureProvider` with a `Cache`:
  key = `sha256(providerID | version | latRound | lonRound | hourUTC)`; on hit it
  returns the decoded feature (marking `cached:true`), on miss it calls the inner
  provider, stores the result with a maturity-based TTL, and returns it.
- `featureservice` fans out the requested (or all) registered providers concurrently
  (`errgroup`), each independent, then assembles the envelope. One slow/failing
  provider never blocks the others.

## Domain model & validation

- `Coordinate{Lat, Lon float64}` — WGS84; `Lat ∈ [-90,90]`, `Lon ∈ [-180,180]`.
- `Instant time.Time` — parsed from `datetime`; **no offset ⇒ interpreted as UTC**;
  normalized to the full hour for querying.
- `TimezoneID string` — IANA; validated via `time.LoadLocation`.
- Optional `Providers []string` filter.

**Validation → HTTP 400, `retryable:false`** (client error) for: lat/lon out of range,
unparseable `datetime`, `datetime > Clock.Now()` (future), invalid IANA timezone.

## Request / Response envelope

`GET /api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z&timezone=Europe/Berlin[&providers=open-meteo]`

```jsonc
{
  "query": {
    "coordinate": {"lat": 49.79, "lon": 9.93},
    "datetime": "2025-06-15T13:00:00Z",
    "timezone": "Europe/Berlin",
    "localTime": "2025-06-15T15:00:00+02:00"
  },
  "features": [{
    "type": "Feature",
    "geometry": {"type": "Point", "coordinates": [9.94, 49.79]},  // provider-resolved grid cell
    "properties": {
      "provider": "open-meteo", "kind": "weather",
      "observedAt": "2025-06-15T13:00:00Z", "localTime": "2025-06-15T15:00:00+02:00",
      "isDay": true,
      "temperature2m": 21.4, "relativeHumidity2m": 55, "precipitation": 0.0,
      "weatherCode": 3, "windSpeed10m": 12.1, "cloudCover": 75,
      "units": {"temperature2m": "°C", "windSpeed10m": "km/h", "precipitation": "mm"}
    },
    "license": {
      "name": "CC-BY 4.0",
      "url": "https://open-meteo.com/en/license",
      "attribution": "Weather data by Open-Meteo.com; ERA5 (Copernicus Climate Change Service / ECMWF)"
    }
  }],
  "providers": [
    {"id": "open-meteo", "kind": "weather", "status": "ok", "cached": true}
  ]
}
```

- **Every feature carries its own `license`** (name/url/attribution), mirroring ortus.
- `query` echoes the resolved request (incl. computed `localTime`) — makes the
  idempotent contract explicit.
- `providers[]` reports the outcome of each requested provider.

## Error taxonomy & retry coding

Provider `Fetch` returns typed errors mapped into a `providers[]` entry:

| Error class | Cause | `status` | `retryable` | Extra |
|---|---|---|---|---|
| transient | network, timeout, 5xx, 429 | `unavailable` | `true` | `retryAfter?` (from `Retry-After` or backoff) |
| not-yet-available | past but younger than archive delay (~5 d); provider returns null for the hour | `unavailable` | `true` | `retryAfter` = estimated maturity time |
| permanent | unexpected 4xx / contract violation | `error` | `false` | `error` message |
| ok | data returned | `ok` | — | `cached` bool |

Per the chosen convention, **even if all providers fail the HTTP status stays 200**
for a valid request; the client inspects `providers[].retryable` and re-issues the
same idempotent request later. Only *request validation* failures produce 4xx.

## Caching

- `Cache` output-port; **disk default** via embedded BoltDB (single file, no external
  service in dev/test). `memory` impl for tests; Redis/S3 later behind the same port.
- Key: `sha256(providerID | schemaVersion | latRound | lonRound | hourUTC)`.
  Coordinates rounded to **2 decimals (~1 km)** — matches Open-Meteo's coarse grid and
  raises hit rate; deterministic, so idempotency holds.
- **TTL by data maturity:** instant older than the archive delay ⇒ data is immutable ⇒
  very long / effectively permanent TTL; instant inside the maturity window ⇒ short TTL
  so the value is refreshed once the archive matures. The provider reports the maturity;
  the decorator picks the TTL.

## Open-Meteo adapter

- **Endpoint by data age:** instant older than ~5 days ⇒ **Archive API**
  (`archive-api.open-meteo.com/v1/archive`, ERA5); more recent ⇒ **Forecast API**
  (`api.open-meteo.com/v1/forecast` with `past_days`). Attribution set to match the
  model actually used (ERA5/Copernicus for archive; note the source for forecast).
- Requests hourly variables for the target date: `temperature_2m`,
  `relative_humidity_2m`, `precipitation`, `weather_code`, `wind_speed_10m`,
  `cloud_cover`, `is_day` (+ room to extend). Selects the hour matching the UTC instant;
  passes `timezone` for local-time labeling and `is_day`.
- Surfaces the provider-resolved grid cell lat/lon as the feature geometry.
- Base URLs, timeout, enabled flag are config. Tested against recorded HTTP fixtures
  via `httptest`.

## HTTP surface

- `GET /api/v1/query` — the feature query (above).
- `GET /api/v1/providers` — available providers + their attribution (like ortus
  `/sources`).
- `GET /health`, `GET /health/ready`, `GET /metrics`, `GET /openapi.json`, `GET /docs`.
- Middleware chain: tracing → trace-id → logging → recovery. `writeJSON`/`writeError`
  envelope. `Router().Walk` powers the routes↔OpenAPI contract test.

## Configuration (`TEMPUS_` env prefix)

```
server.port                 (8080)
query.timeout               (30s)
cache.type                  (disk | memory | redis)  default disk
cache.path                  (./data/cache.bolt)
providers.openMeteo.enabled (true)
providers.openMeteo.archiveBaseURL / forecastBaseURL
providers.openMeteo.timeout (10s)
providers.openMeteo.archiveDelay (5d)   // maturity window boundary
logging.level / format
otel.*                      (tracing/metrics wiring; NoOp when disabled)
```

## Quality harness (full `new-go-service`)

- Hexagonal layout + **depguard** import boundaries (lint-time fitness function).
- **Contract test** `TestRoutesMatchOpenAPISpec` (routes ↔ embedded OpenAPI, both ways).
- **Ratchets:** `.debt-budget` (suppression count, shrink-only; zero TODO/FIXME),
  `.coverage-floors` (per-package, raise-only), `gremlins.yaml` (mutation, CI only —
  panics on macOS).
- **Observability:** OTel metrics on `/metrics`, `Tracer` port (NoOp default), slog
  with trace-id injection + canceled-context hygiene.
- **Docs:** MkDocs Material / Diátaxis; OpenAPI kept byte-identical in embedded +
  `api/openapi/`.
- **Dev env:** multi-stage Dockerfile + per-feature `make dev-*` containers (air,
  Traefik, Dozzle).
- **CI/CD:** GitHub Actions gates + required-status-checks ruleset + release-please
  (conventional commits) + goreleaser. Claude hooks (`format-and-lint.sh`,
  `debt-guard.sh` advisory, `doc-drift-guard.sh`) + `make hooks` pre-commit.

## Testing strategy

- TDD throughout.
- `domain`: table-driven validation tests (ranges, future date via fake `Clock`, tz
  parsing, hour normalization).
- `application`: registry fan-out, caching decorator (hit/miss/TTL), partial-failure
  assembly, error→`providers[]` mapping — with fake providers + fake cache.
- `openmeteo`: recorded HTTP fixtures via `httptest`; endpoint-selection by age;
  attribution correctness; not-yet-available mapping.
- `http`: handler tests for 200-with-failures, 400 validation, contract test.

## Open assumptions baked in (override if wrong)

1. Module path `github.com/jobrunner/tempus`, env prefix `TEMPUS_`.
2. Cache-key coordinate rounding to 2 decimals (~1 km).
3. Weather-only MVP; registry generic for later providers.
