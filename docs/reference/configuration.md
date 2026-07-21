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
| `TEMPUS_CACHE_PATH` | `./tempus.db` | Path for the disk cache file |

## Query

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_TIMEOUT` | `15s` | Total deadline per `/api/v1/query` call |

## Providers — Open-Meteo

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_PROVIDERS_OPENMETEO_ENABLED` | `true` | Enable Open-Meteo |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_BASE_URL` | `https://archive-api.open-meteo.com` | Historical archive endpoint |
| `TEMPUS_PROVIDERS_OPENMETEO_FORECAST_BASE_URL` | `https://api.open-meteo.com` | Forecast endpoint (recent past) |
| `TEMPUS_PROVIDERS_OPENMETEO_TIMEOUT` | `10s` | Per-request HTTP timeout |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_DELAY` | `5d` | Age threshold: queries older than this use the archive API |
