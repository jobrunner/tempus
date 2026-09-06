# Configuration reference

All configuration is via environment variables prefixed with `TEMPUS_`. No config
file is required; the binary applies sensible defaults for all settings.

See [How to configure providers and cache](../how-to/configure-providers.md) for a
practical walkthrough. This page lists every knob.

## Server

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_SERVER_HOST` | `0.0.0.0` | Bind address |
| `TEMPUS_SERVER_PORT` | `8080` | HTTP listen port |
| `TEMPUS_SERVER_READ_TIMEOUT` | `10s` | Header read timeout |
| `TEMPUS_SERVER_SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown window |

## Logging

| Variable | Default | Values | Description |
|---|---|---|---|
| `TEMPUS_LOGGING_LEVEL` | `info` | `debug` `info` `warn` `error` | Minimum log severity |
| `TEMPUS_LOGGING_FORMAT` | `json` | `json` `text` | Structured JSON or human-readable |

## Metrics

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_METRICS_ENABLED` | `false` | Enable Prometheus `/metrics` endpoint |
| `TEMPUS_METRICS_PORT` | `9090` | Metrics server port |
| `TEMPUS_METRICS_PATH` | `/metrics` | Metrics URL path |

## Tracing (OpenTelemetry)

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_TRACING_ENABLED` | `false` | Enable OTLP span export |
| `TEMPUS_TRACING_ENDPOINT` | — | Collector `host:port` (required when enabled) |
| `TEMPUS_TRACING_TRANSPORT` | `grpc` | `grpc` or `http` |
| `TEMPUS_TRACING_SAMPLE_RATIO` | `1.0` | Fraction of traces to sample |

## Cache

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_CACHE_TYPE` | `disk` | `disk` (bbolt) or `memory` |
| `TEMPUS_CACHE_PATH` | `./data/cache.bolt` | Path for the disk cache file |

## Query

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_TIMEOUT` | `30s` | Total deadline per `/api/v1/query` call |

## Query — Batch

Bounds for `POST /api/v1/query/batch` — see the
[HTTP API reference](http-api.md#post-apiv1querybatch).

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_BATCH_MAX_POINTS` | `10000` | Hard cap on points per request (`400` above this) |
| `TEMPUS_QUERY_BATCH_MAX_SYNC_POINTS` | `1000` | Cap for the synchronous JSON envelope (`413` above this; stream NDJSON instead) |
| `TEMPUS_QUERY_BATCH_CONCURRENCY` | `4` | Worker-pool size for deduplicated point fetches |

## Providers — Open-Meteo

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_PROVIDERS_OPENMETEO_ENABLED` | `true` | Enable Open-Meteo |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_BASE_URL` | `https://archive-api.open-meteo.com` | Historical archive endpoint |
| `TEMPUS_PROVIDERS_OPENMETEO_FORECAST_BASE_URL` | `https://api.open-meteo.com` | Forecast endpoint (recent past) |
| `TEMPUS_PROVIDERS_OPENMETEO_TIMEOUT` | `10s` | Per-request HTTP timeout |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_DELAY` | `5d` | Age threshold: queries older than this use the archive API |
| `TEMPUS_PROVIDERS_OPENMETEO_RATE_PER_MINUTE` | `500` | Shared token-bucket rate limit (calls/minute) across all Open-Meteo adapters |
| `TEMPUS_PROVIDERS_OPENMETEO_RETRY_ATTEMPTS` | `3` | Retries for 429/5xx/network errors, honoring the upstream `Retry-After` header |
| `TEMPUS_PROVIDERS_OPENMETEO_DAILY_BUDGET` | `8000` | Weighted daily call budget enforced **only** for batch-originated requests (`GET /api/v1/query` and the astronomy providers are never budget-checked); exhausted budget surfaces as a retryable per-provider error |
| `TEMPUS_PROVIDERS_OPENMETEO_WEIGHTS_WEATHER` | `1` | Budget weight charged per weather-provider call |
| `TEMPUS_PROVIDERS_OPENMETEO_WEIGHTS_AGGREGATE` | `2` | Budget weight charged per aggregate-provider call (now cached, keyed by `gddBase`). The aggregate provider itself makes two upstream calls per fetch (daily range + hourly precipitation), so a point costs 2× this weight against the daily budget |
| `TEMPUS_PROVIDERS_OPENMETEO_WEIGHTS_BIOCLIM` | `30` | Budget weight charged per bioclim-provider call |

The rate limiter and the daily budget are per-process, in-memory state: running multiple replicas multiplies the effective rate and daily budget (each replica enforces its own), and restarting a replica resets its share of the day's spent count.
