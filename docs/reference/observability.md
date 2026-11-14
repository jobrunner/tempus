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
| `tempus_openmeteo_daily_budget_spent` | Gauge | Weighted Open-Meteo calls spent from today's daily budget (only when Open-Meteo is enabled) |
| `tempus_openmeteo_daily_budget_limit` | Gauge | Configured daily weighted-call budget for Open-Meteo (only when Open-Meteo is enabled) |
| Standard Go runtime metrics | — | `go_goroutines`, `go_gc_duration_seconds`, etc. |

## Tracing (OpenTelemetry)

Enable with `TEMPUS_TRACING_ENABLED=true` and set `TEMPUS_TRACING_ENDPOINT`
to your OTLP collector. Spans are exported via gRPC (default) or HTTP.

Trace structure for a typical query:

```
GET /api/v1/query        (server span, one per inbound request)
  ├── HTTP GET           (client span, one per Open-Meteo attempt)
  └── HTTP GET           (a retry gets its own client span)
```

Every call to Open-Meteo that actually goes over the wire becomes a client
span (with `http.response.status_code`) under the inbound request's span; cache
hits make none. The waits around them are recorded as events on the inbound
span: `open-meteo.retry` (`retry.attempt`, `retry.wait_ms`, and the status code
or error that triggered it) and `open-meteo.budget_exhausted` when a batch
request is rejected locally for lack of daily budget. Trace context is **not**
propagated to Open-Meteo: it is a third party, so no `traceparent` header is
sent upstream.
