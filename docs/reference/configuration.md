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
| `TEMPUS_SERVER_READ_TIMEOUT` | `30s` | Whole-request read timeout (headers + body). Batch POSTs must upload within it. |
| `TEMPUS_SERVER_SHUTDOWN_TIMEOUT` | `15s` | Graceful shutdown window |
| `TEMPUS_SERVER_CORS_ALLOWED_ORIGINS` | _(empty)_ | Comma-separated browser origins allowed to call the API cross-origin. Empty disables CORS entirely. |
| `TEMPUS_SERVER_RATE_LIMIT_ENABLED` | `false` | Per-client-IP rate limiting on `/api/v1` |
| `TEMPUS_SERVER_RATE_LIMIT_RATE` | `100` | Sustained requests per second per client IP |
| `TEMPUS_SERVER_RATE_LIMIT_BURST` | `200` | Token-bucket depth per client IP |
| `TEMPUS_SERVER_RATE_LIMIT_TRUSTED_PROXIES` | _(empty)_ | Comma-separated CIDRs of front proxies whose `X-Forwarded-For` may be believed |

### CORS

The bundled frontend is served from the same origin as the API, so it needs no
CORS. Set `TEMPUS_SERVER_CORS_ALLOWED_ORIGINS` only when a **browser** client on
a different domain must call the API — native apps and server-side clients are
unaffected by CORS.

```bash
TEMPUS_SERVER_CORS_ALLOWED_ORIGINS="https://app.example.com,https://*.staging.example.com"
```

Entries are matched either exactly or as a `*.domain` wildcard. Only the host
label is wildcarded — scheme and port still have to match exactly, so
`https://*.example.com` admits `https://app.example.com` but neither
`http://app.example.com` (plaintext) nor `https://app.example.com:8443` (a
different service on the same host), and not the bare `https://example.com`
either.

Allowed origins are echoed back in `Access-Control-Allow-Origin`; requests from
other origins are served normally but without CORS headers, so the browser
blocks them. A preflight (`OPTIONS` carrying `Origin` and
`Access-Control-Request-Method`) is answered with `204` and advertises
`GET, POST, OPTIONS` — POST matters because `/api/v1/query/batch` posts JSON,
which always triggers a preflight. A plain `OPTIONS` without those headers is
left to the router, exactly as before CORS was available.

### Rate limiting

Off by default, and only worth enabling when tempus is reachable directly on a
public IP without a rate-limiting gateway in front. It applies to `/api/v1`
only — health and readiness probes must keep answering under load, or an
orchestrator kills a container that is merely busy. Over-limit requests get
`429` with `Retry-After: 1` in the usual error envelope.

```bash
TEMPUS_SERVER_RATE_LIMIT_ENABLED=true
TEMPUS_SERVER_RATE_LIMIT_RATE=100
TEMPUS_SERVER_RATE_LIMIT_BURST=200
```

Buckets are per client IP and expire after 10 idle minutes, so memory stays
bounded without a background sweeper.

**Behind a proxy**, set `TEMPUS_SERVER_RATE_LIMIT_TRUSTED_PROXIES` to the
proxy's CIDRs — otherwise every request appears to come from the proxy and all
clients share one bucket. `X-Forwarded-For` is believed only when the direct
peer is inside one of those CIDRs, and the client is then taken as the
**right-most** entry that is not itself a trusted proxy. Anything further left
is attacker-controlled: clients can prepend arbitrary values, and trusting them
would let anyone mint a fresh bucket per request.

Two caveats worth knowing before turning it on:

- **Shared IPs.** Mobile clients behind CGNAT share one address, so a per-IP
  limit counts them together. That is why the default is deliberately generous.
- **Batch requests.** `POST /api/v1/query/batch` is a single request no matter
  how many points it carries, so the rate limit barely constrains it. What
  bounds batch cost is the worker pool (`TEMPUS_QUERY_BATCH_CONCURRENCY`) and
  the parallel-read cap of the reanalysis archive
  (`TEMPUS_PROVIDERS_REANALYSIS_MAX_PARALLEL`).

## Logging

| Variable | Default | Values | Description |
|---|---|---|---|
| `TEMPUS_LOGGING_LEVEL` | `info` | `debug` `info` `warn` `error` | Minimum log severity |
| `TEMPUS_LOGGING_FORMAT` | `json` | `json` `text` | Structured JSON or human-readable |

## Metrics

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_METRICS_ENABLED` | `false` | Enable Prometheus `/metrics` endpoint |
| `TEMPUS_METRICS_PORT` | `2112` | Metrics server port |
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
| `TEMPUS_CACHE_MAX_BYTES` | `268435456` | Size cap in bytes for either cache type; going over it evicts the least recently used entries (down to 90 %). `0` or negative = unbounded. The bbolt file does not shrink: freed pages are reused, so it settles slightly above the cap |

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

## Providers — reanalysis archive

The weather, aggregate and bioclim providers read the ECMWF IFS / ERA5 /
ERA5-Land archive from Open-Meteo's open-data bucket — no API, no key, no
quota. See [Data source](../explanation/data-source.md).

| Variable | Default | Description |
|---|---|---|
| `TEMPUS_PROVIDERS_REANALYSIS_ENABLED` | `true` | Enable the archive. When `false`, the `open-meteo` (weather), `aggregate` and `bioclim` providers are not registered; `sun`, `moon` stay. |
| `TEMPUS_PROVIDERS_REANALYSIS_BASE_URL` | `https://openmeteo.s3.amazonaws.com` | Bucket root served over HTTPS (AWS Open Data). Point it at a mirror with the same layout if needed. |
| `TEMPUS_PROVIDERS_REANALYSIS_TIMEOUT` | `30s` | Timeout of a single range request |
| `TEMPUS_PROVIDERS_REANALYSIS_MAX_PARALLEL` | `32` | Maximum concurrent range requests to the bucket (per process) |
| `TEMPUS_PROVIDERS_REANALYSIS_RETRY_ATTEMPTS` | `3` | Retries for 429/5xx/network errors, honoring a `Retry-After` header |
| `TEMPUS_PROVIDERS_REANALYSIS_LIST_TTL` | `1h` | How long a bucket directory listing is trusted; the newest files are rewritten daily |
| `TEMPUS_PROVIDERS_REANALYSIS_MATURITY` | `72h` | Age after which an hour's values are final: results for older instants are cached for a year, younger ones (still revised IFS hours) for an hour |
| `TEMPUS_PROVIDERS_AGGREGATE_ENABLED` | `true` | Enable the `aggregate` provider (needs the archive) |
| `TEMPUS_PROVIDERS_BIOCLIM_ENABLED` | `true` | Enable the `bioclim` provider (needs the archive) |

There is no rate limiter and no daily budget: the bucket has no quota. What
bounds upstream load is `MAX_PARALLEL` and, for batches, the worker pool
(`TEMPUS_QUERY_BATCH_CONCURRENCY`).

The `TEMPUS_PROVIDERS_OPENMETEO_*` variables (archive/forecast URLs, archive
delay, rate per minute, daily budget, weights) no longer exist and are
ignored if set.
