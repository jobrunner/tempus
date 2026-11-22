# How to configure providers and cache

All configuration is via environment variables with the prefix `TEMPUS_`.

## Server

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_SERVER_HOST` | `0.0.0.0` | Listen address |
| `TEMPUS_SERVER_PORT` | `8080` | Listen port |
| `TEMPUS_SERVER_READ_TIMEOUT` | `30s` | Whole-request read timeout (headers + body) |
| `TEMPUS_SERVER_SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown window |
| `TEMPUS_SERVER_CORS_ALLOWED_ORIGINS` | _(empty)_ | Comma-separated browser origins allowed cross-origin; empty disables CORS. See [configuration reference](../reference/configuration.md#cors). |
| `TEMPUS_SERVER_RATE_LIMIT_ENABLED` | `false` | Per-client-IP rate limiting on `/api/v1`; see [configuration reference](../reference/configuration.md#rate-limiting) for the rate, burst and trusted-proxy knobs |

## Logging

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_LOGGING_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `TEMPUS_LOGGING_FORMAT` | `json` | `json` or `text` |

## Metrics

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_METRICS_ENABLED` | `false` | Expose Prometheus metrics |
| `TEMPUS_METRICS_PORT` | `2112` | Metrics listen port |
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
| `TEMPUS_CACHE_PATH` | `./data/cache.bolt` | File path for disk cache (bbolt) |
| `TEMPUS_CACHE_MAX_BYTES` | `268435456` (256 MiB) | Size cap; least recently used entries are evicted first. `0` = unbounded |

The cache stores archive reads per model cell and month, and provider results
keyed by the exact point (coordinate, elevation, hour); see
[Caching model](../explanation/caching.md).
Cached results are served immediately and flagged with `"cached": true` in the
provider status. This avoids repeat upstream calls for the same query inputs.

## Providers — reanalysis archive

Weather, aggregate and bioclim read the ECMWF IFS / ERA5 / ERA5-Land archive
straight from Open-Meteo's open-data bucket (see
[Data source](../explanation/data-source.md)). Out of the box nothing needs
configuring: no API key, no quota, no rate limit to stay under.

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_PROVIDERS_REANALYSIS_ENABLED` | `true` | Enable the archive (and with it the weather, aggregate and bioclim providers) |
| `TEMPUS_PROVIDERS_REANALYSIS_BASE_URL` | `https://openmeteo.s3.amazonaws.com` | Bucket root; set it to a mirror with the same layout |
| `TEMPUS_PROVIDERS_REANALYSIS_TIMEOUT` | `30s` | Timeout of a single range request |
| `TEMPUS_PROVIDERS_REANALYSIS_MAX_PARALLEL` | `32` | Concurrent range requests to the bucket |
| `TEMPUS_PROVIDERS_REANALYSIS_RETRY_ATTEMPTS` | `3` | Retries for 429/5xx/network errors |
| `TEMPUS_PROVIDERS_REANALYSIS_LIST_TTL` | `1h` | How long a bucket directory listing is trusted |
| `TEMPUS_PROVIDERS_REANALYSIS_MATURITY` | `72h` | Results for instants older than this are cached for a year, younger ones for an hour |
| `TEMPUS_PROVIDERS_AGGREGATE_ENABLED` | `true` | Enable the `aggregate` provider |
| `TEMPUS_PROVIDERS_BIOCLIM_ENABLED` | `true` | Enable the `bioclim` provider |

To run with astronomy only (no outbound traffic at all):

```bash
TEMPUS_PROVIDERS_REANALYSIS_ENABLED=false ./tempus
```

The former `TEMPUS_PROVIDERS_OPENMETEO_*` variables (archive/forecast URLs,
archive delay, rate per minute, daily budget, weights) are gone; remove them
from deployments.

## Query

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_TIMEOUT` | `30s` | Total timeout per `GET /api/v1/query` call |

## Query — Batch

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_QUERY_BATCH_MAX_POINTS` | `10000` | Hard cap on points per `POST /api/v1/query/batch` request |
| `TEMPUS_QUERY_BATCH_MAX_SYNC_POINTS` | `1000` | Cap for the synchronous JSON envelope; stream NDJSON above this |
| `TEMPUS_QUERY_BATCH_CONCURRENCY` | `4` | Worker-pool size for deduplicated point fetches |
