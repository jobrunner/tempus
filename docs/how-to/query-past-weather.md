# How to query past weather for an excursion

Use `GET /api/v1/query` to fetch historical weather data for a specific location
and time. The weather and aggregate providers serve only past and present hours:
for a future datetime they report a non-retryable `error` status in the
envelope, while the response is still `200 OK` and `sun`/`moon` are computed
anyway (see [Future datetimes](../explanation/no-future-datetimes.md)).

## Basic query

Supply the WGS84 coordinate, the **elevation in metres above sea level**, and a
datetime (RFC 3339 or offset-less = UTC):

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=47.376' \
  --data-urlencode 'lon=8.541' \
  --data-urlencode 'elevation=408' \
  --data-urlencode 'datetime=2024-08-15T10:00:00Z'
```

The elevation is required. It decides which reanalysis grid cell is used and
corrects temperature and dew point to the location's height (0.65 °C per
100 m). Take it from a topographic map, a DEM or a GNSS receiver's altitude
above mean sea level — not the raw ellipsoidal height. See
[Data source](../explanation/data-source.md#which-elevation-to-send).

## Using an offset-less datetime (treated as UTC)

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=51.505' \
  --data-urlencode 'lon=-0.091' \
  --data-urlencode 'elevation=15' \
  --data-urlencode 'datetime=2024-06-21T14:30:00'
```

!!! note
    An offset-less datetime like `2024-06-21T14:30:00` is interpreted as UTC
    and truncated to the hour. The response echoes the resolved UTC datetime
    in `query.datetime`.

## Filtering providers

To request data from a specific provider only, add the `providers` parameter
(comma-separated provider IDs):

```bash
curl -G 'http://localhost:8080/api/v1/query' \
  --data-urlencode 'lat=48.137' \
  --data-urlencode 'lon=11.576' \
  --data-urlencode 'elevation=520' \
  --data-urlencode 'datetime=2024-09-01T08:00:00Z' \
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
