# How to configure providers and cache

All configuration is via environment variables with the prefix `TEMPUS_`.

## Server

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_SERVER_HOST` | `0.0.0.0` | Listen address |
| `TEMPUS_SERVER_PORT` | `8080` | Listen port |
| `TEMPUS_SERVER_READ_TIMEOUT` | `10s` | HTTP read timeout |
| `TEMPUS_SERVER_SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown window |

## Logging

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_LOGGING_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `TEMPUS_LOGGING_FORMAT` | `json` | `json` or `text` |

## Metrics

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_METRICS_ENABLED` | `false` | Expose Prometheus metrics |
| `TEMPUS_METRICS_PORT` | `9090` | Metrics listen port |
| `TEMPUS_METRICS_PATH` | `/metrics` | Metrics path |

## Tracing (OpenTelemetry)

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_TRACING_ENABLED` | `false` | Enable OTLP trace export |
| `TEMPUS_TRACING_ENDPOINT` | — | OTLP collector `host:port` |
| `TEMPUS_TRACING_TRANSPORT` | `grpc` | `grpc` or `http` |
| `TEMPUS_TRACING_SAMPLE_RATIO` | `1.0` | Sampling fraction (0–1) |

## Cache

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_CACHE_TYPE` | `disk` | `disk`, `memory` |
| `TEMPUS_CACHE_PATH` | `./tempus.db` | File path for disk cache (bbolt) |

The cache stores provider responses keyed by (coordinate-grid-cell, datetime-bucket).
Cached results are served immediately and flagged with `"cached": true` in the
provider status. This avoids repeat upstream calls for the same query inputs.

## Providers — Open-Meteo

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_PROVIDERS_OPENMETEO_ENABLED` | `true` | Enable the Open-Meteo provider |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_BASE_URL` | `https://archive-api.open-meteo.com` | Historical data endpoint |
| `TEMPUS_PROVIDERS_OPENMETEO_FORECAST_BASE_URL` | `https://api.open-meteo.com` | Forecast endpoint (recent past only) |
| `TEMPUS_PROVIDERS_OPENMETEO_TIMEOUT` | `10s` | Per-request timeout |
| `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_DELAY` | `5d` | How far in the past archive data is available (forecast used before this window) |

## Query

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_TIMEOUT` | `15s` | Total timeout per `GET /api/v1/query` call |
