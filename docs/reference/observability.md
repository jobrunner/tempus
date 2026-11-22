# Observability reference

## Health endpoints

| Endpoint | Description |
|---|---|
| `GET /health` | Combined: `{"status":"ok"}` when live and ready |
| `GET /health/live` | Liveness probe — process is running |
| `GET /health/ready` | Readiness probe — providers initialised |

Both liveness and readiness return HTTP 200 on success, HTTP 503 on failure.

## Metrics (Prometheus)

Enable with `TEMPUS_METRICS_ENABLED=true`. The metrics server runs on a
separate port (default `2112`) and exposes `GET /metrics` in the Prometheus
text format.

Key metrics:

| Metric | Type | Description |
|---|---|---|
| `tempus_query_duration_seconds` | Histogram | End-to-end `/api/v1/query` latency |
| `tempus_provider_requests_total` | Counter | Provider fetch attempts, labelled by `provider` and `status` |
| `tempus_cache_hits_total` | Counter | Cache hits, labelled by `provider` |
| `tempus_cache_misses_total` | Counter | Cache misses, labelled by `provider` |
| Standard Go runtime metrics | — | `go_goroutines`, `go_gc_duration_seconds`, etc. |

## Tracing (OpenTelemetry)

Enable with `TEMPUS_TRACING_ENABLED=true` and set `TEMPUS_TRACING_ENDPOINT`
to your OTLP collector. Spans are exported via gRPC (default) or HTTP.

Trace structure for a typical query:

```
GET /api/v1/query        (server span, one per inbound request)
  ├── HTTP GET           (client span, one per range request to the bucket)
  └── HTTP GET           (a retry gets its own client span)
```

Every range request to the reanalysis bucket
(`providers.reanalysis.base_url`) that actually goes over the wire becomes a
client span (with `http.response.status_code`) under the inbound request's
span; result-cache hits and in-memory metadata hits make none. A cold 30-year
bioclim read therefore shows about 90 client spans per variable read. Backoff
waits are recorded as `bucket.retry` events on the inbound span
(`retry.attempt`, `retry.wait_ms`, and the status code or error that triggered
it). There is no rate limiter and no daily budget any more, so the former
`open-meteo.budget_exhausted` event and the
`tempus_openmeteo_daily_budget_*` gauges are gone. Trace context is **not**
propagated to the bucket: it is third-party storage, so no `traceparent` header
is sent upstream.
