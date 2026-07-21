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
separate port (default `9090`) and exposes `GET /metrics` in the Prometheus
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
tempus.query                       (root span, route: /api/v1/query)
  └── provider.fetch open-meteo   (per-provider upstream fetch or cache read)
```

Span attributes include `lat`, `lon`, `datetime`, `provider_id`, and
`cache_hit`.
