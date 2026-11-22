# Caching model

## Why cache at all?

Historical weather data is **immutable once final**: the temperature at
48.137°N, 11.576°E, 520 m on 1 July 2025 at noon UTC does not change after the
fact. tempus caches at two levels:

1. **Archive reads** — one month of one variable at one grid cell of one model.
   This is the expensive part (range requests to the open-data bucket), and
   every point that falls into the same model cells shares it.
2. **Provider results** — the finished feature for one exact request. Cheap to
   recompute from cached archive reads, but a hit skips cell selection,
   elevation correction and mixing altogether.

Everything between the two levels — which cells a point uses, the correction
to the point's elevation, the mix of models — is computed for the exact point,
so a cached answer is identical to an uncached one.

## Archive reads

`application.CachingSeriesReader` sits between the reanalysis service and the
open-data store. A read is split into UTC calendar months; months already in
the cache are served from it, the missing ones are read in as few upstream
calls as possible (adjacent months merged) and cached. A weather lookup for one
day therefore warms the whole month of that cell: every other hour and day of
the month, for any point in the same cells, is served without the bucket.

Each entry is keyed by model, variable, grid location and month, and holds the
month's hourly values as raw float32 (about 3 KB). Reads spanning more than
400 days — the 30-year normals behind bioclim — bypass this level: bioclim
caches its own result, and decades of hourly blocks per cell would crowd
everything else out of a capped cache.

## Result cache key

The caching decorator (`application.CachingProvider`, used by the weather and
aggregate providers) derives its key from:

1. **Provider ID and cache version** — each provider has its own namespace;
   bumping the version retires every old entry. The current version (`3`)
   keys on the exact point; version 2 rounded the coordinate to 0.01° and
   served a nearby point's geometry.
2. **Exact coordinate** — every feature carries its point (the geometry, and
   values such as `isDay` or the GDD season start that follow from it), so
   two points never share a result, however close. They share the archive
   reads instead.
3. **Exact elevation** — it selects the model cell and drives the lapse-rate
   correction (see [Data source](data-source.md)).
4. **Instant** — truncated to the hour when the request is parsed, so
   `2025-07-01T12:05:00Z` and `2025-07-01T12:30:00Z` share an entry.
5. **Provider parameters** — e.g. the aggregate provider's `gddBase`.

The bioclim provider keeps its own entries, keyed by the coordinate rounded to
0.0001° (about 10 m), the exact elevation and the reference period (not the
instant), cached for one year.

Batch deduplication uses the same notion of "the same point" as the result
cache: identical coordinate, elevation, instant and options are fetched once
within a batch.

## Data maturity

The newest hours of the archive come from ECMWF IFS and are still revised for a
while after they appear; older hours are final. There is no longer a separate
archive and forecast source — one archive covers everything up to the current
day — so maturity only decides how long something is cached:

- Instants (and archive months whose last hour is) **older than
  `TEMPUS_PROVIDERS_REANALYSIS_MATURITY`** (default 72 h) → cached for a year.
- Younger ones → cached for an hour, then re-read with whatever the archive
  holds by then. An archive month still holding young hours expires sooner if
  one of them turns final first (at the next whole hour), so a result cached
  for a year as final is never computed from a value read while its hour was
  still being revised. A read that crosses that moment is not cached at all,
  and results count as final only ten minutes after their hour does, so a
  result computed from a block served just before the moment stays short-lived.

Concurrent misses for the same month of the same cell — nearby points in one
batch, typically — share a single upstream read.

Errors (including "not yet available" for an hour the archive has not filled
in) are never cached.

## What is not cached on disk

The archive is not mirrored wholesale — only the months points were actually
asked about. The store keeps bucket listings (for
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
