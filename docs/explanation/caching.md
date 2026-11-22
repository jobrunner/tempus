# Caching model

## Why cache at all?

Historical weather data is **immutable once final**: the temperature at
48.137°N, 11.576°E, 520 m on 1 July 2025 at noon UTC does not change after the
fact. A cache that stores provider results keyed by (coordinate, elevation,
datetime) serves every later request for the same query without reading the
reanalysis archive again.

## Cache key

The caching decorator (`application.CachingProvider`, used by the weather and
aggregate providers) derives its key from:

1. **Provider ID and cache version** — each provider has its own namespace;
   bumping the version retires every old entry. The current version (`2`)
   ignores all entries computed from the former Open-Meteo API.
2. **Coordinate rounded to 0.01°** (about 1 km) — nearby points share an entry.
3. **Elevation, exactly as sent** (not rounded; 182.6 m and 182.7 m are two
   entries) — it selects the model cell and drives
   the lapse-rate correction, so two heights at the same coordinate are
   different results (see [Data source](data-source.md)).
4. **Instant** — truncated to the hour when the request is parsed, so
   `2025-07-01T12:05:00Z` and `2025-07-01T12:30:00Z` share an entry.
5. **Provider parameters** — e.g. the aggregate provider's `gddBase`.

The bioclim provider keeps its own entries, keyed by the coordinate rounded to
0.0001°, the exact elevation and the reference period (not the
instant), cached for one year.

Batch deduplication uses the same coordinate rounding and exact elevation, so
points that would share a cache entry are also fetched only once within a
batch.

## Data maturity

The newest hours of the archive come from ECMWF IFS and are still revised for a
while after they appear; older hours are final. There is no longer a separate
archive and forecast source — one archive covers everything up to the current
day — so maturity only decides how long a result is cached:

- Instants **older than `TEMPUS_PROVIDERS_REANALYSIS_MATURITY`** (default 72 h)
  → cached for a year.
- Younger instants → cached for an hour, then recomputed with whatever the
  archive holds by then.

Errors (including "not yet available" for an hour the archive has not filled
in) are never cached.

## What is not cached on disk

The archive itself is not mirrored. The store keeps bucket listings (for
`TEMPUS_PROVIDERS_REANALYSIS_LIST_TTL`) and file metadata in memory only, so a
restart costs a few extra range requests, nothing more.

## Cache backends

| Backend | `TEMPUS_CACHE_TYPE` | Notes |
|---|---|---|
| bbolt (disk) | `disk` | Persistent across restarts; file path via `TEMPUS_CACHE_PATH` |
| In-memory | `memory` | Faster but lost on restart; suitable for development |

## Size cap

Both backends are capped by `TEMPUS_CACHE_MAX_BYTES` (default 256 MiB; `0`
means unbounded). Every entry records when it was last read; going over the cap
evicts the least recently used entries. The bbolt backend evicts down to 90 %
of the cap (best effort: the entry just written is never evicted), so a full
disk cache does not run an eviction transaction on every write; the in-memory
backend, where eviction is cheap, evicts just enough to fit. A year-long TTL is therefore
an upper bound, not a promise: a result nobody asks for again gives way to ones
that are.

The bbolt backend rewrites an entry's access time at most once an hour, so a
warm read stays a read-only transaction; eviction is least-recently-used at
that granularity.

The cap counts **live entries, not the file**. bbolt never gives disk space
back: pages freed by eviction are reused for new entries, so in steady state
the file sits a little above the cap, but it keeps whatever size it once
reached. A cache file written before the cap existed is emptied on the first
start, and a file filled under a larger cap is trimmed to the new one — in
both cases the entries go, the file size stays. To reclaim the space, stop
the service and delete the file (`TEMPUS_CACHE_PATH`); it is only a cache and
is rebuilt on demand.
