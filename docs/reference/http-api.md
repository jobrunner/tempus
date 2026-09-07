# HTTP API reference

The tempus HTTP API is formally described by an OpenAPI 3.0 spec embedded in
the binary and served at runtime:

- **Interactive docs (Swagger UI):** `GET /docs`
- **Raw spec (JSON):** `GET /openapi.json`
- **Source spec:** `api/openapi/openapi.yaml` (byte-identical mirror of the
  embedded copy at `internal/adapters/http/openapi.yaml`)

---

## `GET /api/v1/query`

Query registered feature providers for a coordinate and point in time.

### Parameters

| Name | In | Required | Type | Description |
|---|---|---|---|---|
| `lat` | query | yes | number | WGS84 latitude |
| `lon` | query | yes | number | WGS84 longitude |
| `datetime` | query | yes | string | RFC 3339 (`2025-07-01T12:00:00Z`) **or** offset-less (`2025-07-01T12:00:00`, treated as UTC). Future instants are allowed — see [Future datetimes](../explanation/no-future-datetimes.md). |
| `providers` | query | no | string | Comma-separated provider IDs. Omit to query all enabled providers. |
| `gddBase` | query | no | number | Base temperature (°C, [-50,50]) for the aggregate provider's growing-degree-days. Omit for the server default (10 °C). See [Weather aggregates](../explanation/aggregates.md). |
| `refPeriod` | query | no | string | Reference period `YYYY-YYYY` (start ≥ 1940) for the bioclim provider. Omit to auto-select the contemporaneous 30-year normal. See [Bioclimatic variables](../explanation/bioclim.md). |

### Responses

#### `200 OK` — `QueryResult`

The service always returns `200 OK` for valid input, even when a provider is
unavailable. Per-provider errors are encoded in `providers[].status`.

```json
{
  "query": {
    "coordinate": {"lat": 48.137, "lon": 11.576},
    "datetime": "2025-07-01T12:00:00Z"
  },
  "features": [
    {
      "type": "Feature",
      "geometry": {"type": "Point", "coordinates": [11.576, 48.137]},
      "properties": {
        "temperature_2m": 24.3,
        "precipitation": 0.0,
        "wind_speed_10m": 5.1
      },
      "license": {
        "name": "Open-Meteo",
        "url": "https://open-meteo.com",
        "attribution": "Weather data by Open-Meteo.com"
      }
    }
  ],
  "providers": [
    {
      "id": "open-meteo",
      "kind": "weather",
      "status": "ok",
      "cached": true
    }
  ]
}
```

**`query`** — echoes the resolved request so the client can verify what was
actually queried:

| Field | Type | Description |
|---|---|---|
| `coordinate.lat` | number | Resolved latitude |
| `coordinate.lon` | number | Resolved longitude |
| `datetime` | string | UTC datetime used for the query |

**`features[]`** — GeoJSON Features, one per successful provider. Each feature
**must** contain a `license` block:

| Field | Type | Description |
|---|---|---|
| `type` | string | Always `"Feature"` |
| `geometry` | object | GeoJSON geometry (Point at provider-resolved location) |
| `properties` | object | Provider-specific data (e.g. weather variables) |
| `license.name` | string | **Required.** Human-readable data source name |
| `license.url` | string | **Required.** URL to the provider's terms / site |
| `license.attribution` | string | **Required.** Attribution string to display to end-users |

The `properties.kind` discriminates the feature type: `weather` (Open-Meteo),
`dewpoint` (derived — see [Derived features](../explanation/derived-features.md)),
`sun` / `moon` (computed — see [Sun and moon](../explanation/astronomy.md)), and
`aggregate` (antecedent precipitation, day extrema, growing-degree-days — see
[Weather aggregates](../explanation/aggregates.md)), and `bioclim` (19 WorldClim
variables + Köppen-Geiger — see [Bioclimatic variables](../explanation/bioclim.md)).
The `sun` and `moon` features are available for any date, including the future;
`weather` and `aggregate` need past data; `bioclim` is a location climate normal.

**`providers[]`** — one entry per queried provider, regardless of outcome:

| Field | Type | Description |
|---|---|---|
| `id` | string | Provider identifier (e.g. `open-meteo`) |
| `kind` | string | Data kind (e.g. `weather`) |
| `status` | string | `ok` \| `unavailable` \| `error` |
| `cached` | bool | `true` if the feature was served from cache |
| `retryable` | bool | `true` when `status` is `unavailable` and retrying will help |
| `retryAfter` | string | RFC 3339 hint for when to retry (optional) |
| `error` | string | Error message when `status` is `error` |

#### `400 Bad Request` — `Error`

Returned for missing/invalid parameters (e.g. an out-of-range coordinate or an
unparseable datetime):

```json
{"error": "invalid_request", "message": "invalid lat: must be a number in [-90,90]"}
```

A future datetime is **not** a client error: the request returns `200 OK`, the
weather provider reports a non-retryable `error` status, and the `sun`/`moon`
features are still computed.

---

## `POST /api/v1/query/batch`

Query many coordinate+time points in one request through the same provider
pipeline as `GET /api/v1/query`. Points that share a rounded coordinate,
instant, and options are fetched once and fanned out to every matching
result, so duplicate points in a batch cost nothing extra.

### Request — `BatchQueryRequest`

```json
{
  "providers": ["open-meteo"],
  "gddBase": 7,
  "refPeriod": "1991-2020",
  "points": [
    {"id": "a", "lat": 48.137, "lon": 11.576, "datetime": "2025-07-01T12:00:00Z"},
    {"id": "b", "lat": 52.520, "lon": 13.405, "datetime": "2025-07-02T12:00:00Z"}
  ]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `providers` | string[] | no | Provider-id filter applied to every point. Omit to query all enabled providers. |
| `gddBase` | number | no | Extra GDD base (°C, [-50,50]) applied to every point, **in addition to** the two bases the aggregate provider always computes — see [note below](#gdd-base-5-and-10). |
| `refPeriod` | string | no | Bioclim reference period `YYYY-YYYY`, same rules as `GET /api/v1/query`. |
| `points` | array | yes | 1..`query.batch.max_points` points (default cap 10000, see [configuration](configuration.md)). |
| `points[].id` | string | no | Opaque echo id. Defaults to the 0-based input index as a string. |
| `points[].lat` / `.lon` | number | yes | WGS84 coordinate. |
| `points[].datetime` | string | yes | Same formats as `GET /api/v1/query`'s `datetime`. |

An invalid point (bad coordinate, unparseable datetime, out-of-range
`gddBase`, …) does **not** fail the whole request: it becomes a per-item
error result (see below) and every other point is still processed.

### Response modes

The response shape depends on the `Accept` header of the request:

#### Synchronous — `200 OK`, `BatchQueryResponse`

Default when the client does not send `Accept: application/x-ndjson`. All
results are buffered and returned as one JSON envelope, capped at
`query.batch.max_sync_points` points (default 1000; see
[`413` below](#413-request-entity-too-large)):

```json
{
  "results": [
    {
      "id": "a",
      "query": {"coordinate": {"lat": 48.137, "lon": 11.576}, "datetime": "2025-07-01T12:00:00Z"},
      "features": [ /* same Feature[] shape as GET /api/v1/query */ ],
      "providers": [ /* same ProviderStatus[] shape as GET /api/v1/query */ ]
    }
  ],
  "total": 1,
  "processing_time_ms": 842
}
```

| Field | Type | Description |
|---|---|---|
| `results[]` | array | One item per input point, in input order — see [Result items](#result-items) below. |
| `total` | integer | `len(results)` |
| `processing_time_ms` | integer | Wall-clock time spent processing the batch |

#### Streaming — `200 OK`, NDJSON

Sent when the request carries `Accept: application/x-ndjson`. The response
is `Content-Type: application/x-ndjson`: one result item per line, flushed as
each point finishes, in input order — no envelope, no trailing summary. A
client detects a truncated stream by comparing the line count to the number
of points it sent. Streaming has no sync-mode point cap, so it is the way to
process batches larger than `query.batch.max_sync_points`.

#### Result items

Both modes emit the same per-point item shape (`BatchQueryResultItem`): the
single-query envelope (`query`, `features`, `providers`) plus the echo `id` —
or, for a point that failed to parse, only `id` and `error`:

```json
{"id": "c", "error": {"message": "invalid lat: must be a number in [-90,90]"}}
```

| Field | Type | Description |
|---|---|---|
| `id` | string | Echo of the point's `id` (or its input index) |
| `query` / `features` / `providers` | — | Present for a processed point; same shape as `GET /api/v1/query`'s `200` response |
| `error.message` | string | Present instead of `query`/`features`/`providers` for a point that never reached the query path |

#### GDD base 5 and 10

The aggregate provider's growing-degree-days are always computed for base
temperatures 5 °C and 10 °C, whatever `gddBase` says — a batch client can
never lose those two series. A request-level `gddBase` (like the `7` in the
example above) only adds one more, `custom`, base alongside them.

### Errors

#### `400 Bad Request`

Empty `points`, malformed JSON, or more points than `query.batch.max_points`:

```json
{"error": "invalid_request", "message": "too many points: 12000 > 10000"}
```

#### `413 Request Entity Too Large`

Two distinct causes share this status:

- The request body exceeds the size cap (`query.batch.max_points * 512
  bytes + 64 KiB`, sized generously for small per-point JSON objects):
  `"request body too large"`.
- The client asked for the synchronous envelope (no NDJSON `Accept` header)
  with more points than `query.batch.max_sync_points`:
  `"more than 1000 points require streaming; retry with Accept:
  application/x-ndjson"`.

### Throttling and the daily budget

The shared Open-Meteo token-bucket rate limiter applies to every Open-Meteo
call — `GET /api/v1/query` included — not only batch traffic. The weighted
daily call budget, however, is batch-only (see
[configuration](configuration.md#providers-open-meteo) for the knobs). The
aggregate provider makes two upstream calls per fetch, so a point costs 2×
its configured weight against the budget. The astronomy providers
(`sun`/`moon`) are never budget-checked. When the day's weighted budget is
spent, the affected provider(s) report a transient failure per point rather
than failing the batch:

```json
{"id": "a", "providers": [{"id": "open-meteo", "kind": "weather", "status": "unavailable", "retryable": true, "error": "Get \"https://api.open-meteo.com/v1/forecast?...\": open-meteo daily budget exhausted; retry tomorrow"}]}
```

Both the rate limiter and the daily budget are per-process, in-memory state: running multiple replicas multiplies the effective rate and daily budget, and restarting a replica resets its share of the day's spent count.

---

## `GET /api/v1/providers`

List all registered providers and their attribution metadata.

### Response `200 OK`

```json
{
  "providers": [
    {
      "id": "open-meteo",
      "kind": "weather",
      "license": {
        "name": "Open-Meteo",
        "url": "https://open-meteo.com",
        "attribution": "Weather data by Open-Meteo.com"
      }
    }
  ]
}
```

---

## Health endpoints

These endpoints are **not** part of the business contract (not in the OpenAPI
spec) and are intended for Kubernetes liveness/readiness probes.

| Endpoint | Description |
|---|---|
| `GET /health` | Combined health check (`{"status":"ok"}`) |
| `GET /health/live` | Liveness: service process is running |
| `GET /health/ready` | Readiness: all providers initialised and reachable |

---

## OpenAPI spec endpoints

| Endpoint | Description |
|---|---|
| `GET /openapi.json` | OpenAPI 3.0 spec (JSON) |
| `GET /docs` | Swagger UI — interactive API explorer |

The spec lives twice in the repository — `internal/adapters/http/openapi.yaml`
(embedded and served) and `api/openapi/openapi.yaml` (the published copy). The
`OpenAPI Spec` workflow checks on every pull request that the two are
byte-identical.

### Breaking-change policy

The same workflow runs `oasdiff breaking` against the base branch's spec and
fails on a breaking change: a removed or renamed property, a narrowed type, a
new required field, a changed response shape. Payload-compatible changes that
alter the *generated types* count too — turning `Feature.properties` from a free
object into a discriminated union did (see #43).

An intended break is allowed: label the pull request `api-breaking-ok` and give
the reason in its description. The label is the record of a deliberate decision
rather than an oversight, and adding it re-runs the check. It does not bypass a
check that failed for a tool or setup reason — only a completed comparison that
found breaking changes.

Two details matter if you ever touch that workflow: oasdiff runs with
`--flatten-allof`, because the per-kind feature schemas are `allOf`
compositions and without flattening every finding inside them is demoted to a
warning that `--fail-on ERR` ignores; and the oasdiff version is pinned, so a
new release cannot silently reclassify what counts as breaking.
