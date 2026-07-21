# Caching model

## Why cache at all?

Historical weather data is **immutable once published**: the temperature at
48.137°N, 11.576°E on 1 July 2025 at noon UTC does not change after the fact.
A cache that stores provider responses keyed by (coordinate, datetime) can serve
all subsequent requests for the same query instantly without touching the upstream
provider.

## Cache key

The cache key encodes:

1. **Provider ID** — each provider has its own cache namespace.
2. **Coordinate grid cell** — the provider's native spatial resolution (e.g.
   Open-Meteo snaps to its model grid). Queries at nearby coordinates that fall
   in the same cell share a cache entry.
3. **Datetime bucket** — the provider's temporal resolution (typically 1 hour).

This means a request for `2025-07-01T12:05:00Z` and one for `2025-07-01T12:30:00Z`
may share a cache entry if both fall in the provider's noon hour bucket.

## Data maturity

Some providers distinguish between **forecast** data (available immediately but
subject to revision) and **archive** data (available after a delay, typically
5 days, but finalised and accurate). tempus routes requests to the appropriate
upstream endpoint based on age:

- Datetimes **older than `TEMPUS_PROVIDERS_OPENMETEO_ARCHIVE_DELAY`** (default 5 days)
  → archive API (final, stable data, safe to cache indefinitely).
- Datetimes **within the archive delay window** → forecast/recent API (data is
  stable enough to cache with a shorter TTL).

Cached entries from the forecast endpoint may be evicted and re-fetched as the
data matures.

## Cache backends

| Backend | `TEMPUS_CACHE_TYPE` | Notes |
|---|---|---|
| bbolt (disk) | `disk` | Persistent across restarts; file path via `TEMPUS_CACHE_PATH` |
| In-memory | `memory` | Faster but lost on restart; suitable for development |
