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
| `datetime` | query | yes | string | RFC 3339 (`2025-07-01T12:00:00Z`) **or** offset-less (`2025-07-01T12:00:00`, treated as UTC). **Must not be in the future.** |
| `providers` | query | no | string | Comma-separated provider IDs. Omit to query all enabled providers. |

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

Returned for missing/invalid parameters or a future datetime:

```json
{"error": "invalid_request", "message": "datetime must not be in the future"}
```

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
