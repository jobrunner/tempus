# How to query past weather for an excursion

Use `GET /api/v1/query` to fetch historical weather data for a specific location
and time. The datetime **must not be in the future** — the service rejects future
timestamps with `400 Bad Request`.

## Basic query

Supply the WGS84 coordinate, a datetime (RFC 3339 or offset-less = UTC), and the
local IANA timezone:

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=47.376' \
  --data-urlencode 'lon=8.541' \
  --data-urlencode 'datetime=2024-08-15T10:00:00Z' \
  --data-urlencode 'timezone=Europe/Zurich'
```

## Using an offset-less datetime (treated as UTC)

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=51.505' \
  --data-urlencode 'lon=-0.091' \
  --data-urlencode 'datetime=2024-06-21T14:30:00' \
  --data-urlencode 'timezone=Europe/London'
```

!!! note
    An offset-less datetime like `2024-06-21T14:30:00` is interpreted as UTC.
    The response always echoes the resolved UTC datetime and the local time in the
    requested timezone.

## Filtering providers

To request data from a specific provider only, add the `providers` parameter
(comma-separated provider IDs):

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=48.137' \
  --data-urlencode 'lon=11.576' \
  --data-urlencode 'datetime=2024-09-01T08:00:00Z' \
  --data-urlencode 'timezone=Europe/Berlin' \
  --data-urlencode 'providers=open-meteo'
```

## Handling provider unavailability

When a provider is temporarily unavailable, its entry in `providers[]` will have
`"status": "unavailable"` and `"retryable": true`. The HTTP response is still
`200 OK`. Your client should check each provider status and retry the whole
request later (the endpoint is idempotent on the same inputs):

```json
{
  "providers": [
    {
      "id": "open-meteo",
      "status": "unavailable",
      "retryable": true,
      "retryAfter": "2025-07-01T13:00:00Z"
    }
  ]
}
```

Retry at or after the `retryAfter` time. If `retryAfter` is absent, use
exponential back-off.
