# Getting started with tempus

This tutorial walks you from zero to a working query in about five minutes.
You will start the service locally and retrieve historical weather data for a
coordinate.

## Prerequisites

- Docker (or a Go 1.26 toolchain)
- `curl` or any HTTP client
- Outbound HTTPS to `openmeteo.s3.amazonaws.com` (no account or API key)

## Step 1 — Start the service

=== "Docker"

    ```bash
    docker run --rm -p 8080:8080 ghcr.io/jobrunner/tempus:latest
    ```

=== "From source"

    ```bash
    git clone https://github.com/jobrunner/tempus
    cd tempus
    go build -o tempus ./cmd/tempus
    ./tempus
    ```

The service starts on `0.0.0.0:8080` by default.

## Step 2 — Verify it is healthy

```bash
curl -s http://localhost:8080/health | jq .
```

Expected response:

```json
{"status": "ok"}
```

## Step 3 — List available providers

```bash
curl -s http://localhost:8080/api/v1/providers | jq .
```

You will see a list of configured providers (e.g. `open-meteo`) with their
attribution metadata.

## Step 4 — Make your first weather query

Query the weather for Munich (48.137, 11.576, 520 m above sea level) at noon
UTC on 1 July 2025. The elevation is required: it picks the model grid cell and
corrects the temperature to the location's height.

```bash
curl -s 'http://localhost:8080/api/v1/query?lat=48.137&lon=11.576&elevation=520&datetime=2025-07-01T12:00:00Z' | jq .
```

A successful response looks like this (shortened to the weather feature):

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
        "kind": "weather",
        "elevation": 520,
        "temperature2m": 24.3,
        "precipitation": 0.0
      },
      "license": {
        "name": "CC-BY 4.0",
        "url": "https://open-meteo.com/en/license",
        "attribution": "Weather data: ECMWF IFS (CC-BY 4.0) and Copernicus Climate Change Service ERA5/ERA5-Land (contains modified Copernicus Climate Change Service information), processed and distributed by Open-Meteo.com (CC-BY 4.0 open data)"
      }
    }
  ],
  "providers": [
    {"id": "open-meteo", "kind": "weather", "status": "ok", "cached": false}
  ]
}
```

The first query for a location reads the reanalysis archive from Open-Meteo's
open-data bucket (see [Data source](../explanation/data-source.md)); repeating
it is served from the cache and reports `"cached": true`.

## Step 5 — Explore the interactive API docs

Open <http://localhost:8080/docs> in your browser to explore the full API
contract via Swagger UI. The raw OpenAPI 3.0 spec is at
<http://localhost:8080/openapi.json>.

## Next steps

- [How-to: query a past excursion's weather](../how-to/query-past-weather.md)
- [How-to: configure providers and cache](../how-to/configure-providers.md)
- [Reference: full API contract](../reference/http-api.md)
