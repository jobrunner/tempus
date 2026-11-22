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
| `elevation` | query | yes | number | Height of the location in metres **above sea level** (orthometric — from a map, a DEM or a GNSS receiver's MSL altitude; not the ellipsoidal height), range [-500, 9000]. Selects the reanalysis grid cell and drives the lapse-rate correction — see [Data source](../explanation/data-source.md). |
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
        "provider": "open-meteo",
        "kind": "weather",
        "observedAt": "2025-07-01T12:00:00Z",
        "elevation": 520,
        "modelCells": [
          {"model": "ecmwf_ifs", "lat": 48.13, "lon": 11.57, "elevation": 520}
        ],
        "temperature2m": 24.3,
        "precipitation": 0.0,
        "windSpeed10m": 5.1
      },
      "license": {
        "name": "CC-BY 4.0",
        "url": "https://open-meteo.com/en/license",
        "attribution": "Weather data: ECMWF IFS (CC-BY 4.0) and Copernicus Climate Change Service ERA5/ERA5-Land (contains modified Copernicus Climate Change Service information), processed and distributed by Open-Meteo.com (CC-BY 4.0 open data)"
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
| `geometry` | object | GeoJSON Point at the requested coordinate |
| `properties` | object | Provider-specific data (e.g. weather variables) |
| `license.name` | string | **Required.** Human-readable data source name |
| `license.url` | string | **Required.** URL to the provider's terms / site |
| `license.attribution` | string | **Required.** Attribution string to display to end-users |

The `properties.kind` discriminates the feature type: `weather` (provider id
`open-meteo`, read from the reanalysis archive — see
[Data source](../explanation/data-source.md); it also reports the request
`elevation` and the `modelCells` the values came from),
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

A missing or out-of-range `elevation` is rejected the same way:
`"invalid elevation: is required: metres above sea level in [-500,9000]"`.

A future datetime is **not** a client error: the request returns `200 OK`, the
weather provider reports a non-retryable `error` status, and the `sun`/`moon`
features are still computed.

---

## `POST /api/v1/query/batch`

Query many coordinate+time points in one request through the same provider
pipeline as `GET /api/v1/query`. Points that share coordinate,
elevation, instant, and options are fetched once and fanned out to every matching
result, so duplicate points in a batch cost nothing extra.

### Request — `BatchQueryRequest`

```json
{
  "providers": ["open-meteo"],
  "gddBase": 7,
  "refPeriod": "1991-2020",
  "points": [
    {"id": "a", "lat": 48.137, "lon": 11.576, "elevation": 520, "datetime": "2025-07-01T12:00:00Z"},
    {"id": "b", "lat": 52.520, "lon": 13.405, "elevation": 34, "datetime": "2025-07-02T12:00:00Z"}
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
| `points[].elevation` | number | yes | Metres above sea level, same rules as `GET /api/v1/query`'s `elevation`. |
| `points[].datetime` | string | yes | Same formats as `GET /api/v1/query`'s `datetime`. |

An invalid point (bad coordinate, missing or out-of-range elevation,
unparseable datetime, out-of-range `gddBase`, …) does **not** fail the whole request: it becomes a per-item
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

### Upstream load

There is no rate limiter and no daily budget: the weather, aggregate and bioclim
providers read the reanalysis archive from a public bucket without quota (see
[Data source](../explanation/data-source.md)). What bounds a batch is the
worker pool (`query.batch.concurrency`) and the cap on concurrent range
requests (`providers.reanalysis.max_parallel`, see
[configuration](configuration.md#providers-reanalysis-archive)). A bucket
failure that survives the retries surfaces per point as a transient provider
status (`"status": "unavailable"`, `"retryable": true`), never as a failed
batch.

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
        "name": "CC-BY 4.0",
        "url": "https://open-meteo.com/en/license",
        "attribution": "Weather data: ECMWF IFS (CC-BY 4.0) and Copernicus Climate Change Service ERA5/ERA5-Land (contains modified Copernicus Climate Change Service information), processed and distributed by Open-Meteo.com (CC-BY 4.0 open data)"
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

The `/docs` page loads Swagger UI's script and stylesheet from unpkg.com, pinned
to one exact `swagger-ui-dist` release with Subresource Integrity hashes
(`internal/adapters/http/openapi.go`). The browser viewing it therefore needs
internet access; the spec itself is always served by tempus.

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
