# How to operate the reanalysis archive

The weather, aggregate and bioclim providers read ECMWF IFS, ERA5 and
ERA5-Land straight from Open-Meteo's open-data bucket on AWS (see
[Data source](../explanation/data-source.md)). This page covers what that
means for a deployment: network, latency, disk, outages, attribution, and how
to check that tempus still reproduces the Open-Meteo archive.

## Network

tempus needs **outbound HTTPS (443) to `openmeteo.s3.amazonaws.com`** — and
nothing else for weather data. There is no API key, no account, no quota and no
rate limit to stay under; the bucket is anonymous AWS Open Data.

- Requests are plain `GET`s: S3 `ListObjectsV2` listings of a variable's
  directory and HTTP range requests into `.om` files. No AWS SDK, no
  credentials, no signed requests.
- Behind an egress proxy, allow that host name; Go's HTTP client honours
  `HTTPS_PROXY` / `NO_PROXY`.
- To read from a mirror with the same layout, set
  `TEMPUS_PROVIDERS_REANALYSIS_BASE_URL`.
- Without any outbound traffic, disable the archive
  (`TEMPUS_PROVIDERS_REANALYSIS_ENABLED=false`); sun and moon keep working.

## Latency and parallelism

The bucket lives in AWS `us-west-2` (Oregon), so from Europe every round trip
crosses the Atlantic. tempus therefore reads all files of a series in parallel,
fetches only the byte ranges a series needs, and caches file metadata in
memory. Measured against the bucket:

| Read | Range requests | Cost |
|---|---|---|
| One weather day for one location | a few per model | tens of kB transferred |
| 30 years of one variable in one cell, cold | about 90 | about 2.6 s, 1.2 MB |
| The same, metadata cached | one per file | about 1 s |

A bioclim query reads 30 years of several variables, each from more than one
model, in parallel; its end-to-end time is therefore a multiple of one series
read and depends on `MAX_PARALLEL`. The series figures come from
`TestLive_ThirtyYears` (see below) and are the thing to compare when the
uplink changes.

The knobs (see [Configure providers](configure-providers.md#providers-reanalysis-archive)):

- `TEMPUS_PROVIDERS_REANALYSIS_MAX_PARALLEL` (default `32`) — concurrent range
  requests per tempus process. Raise it for many concurrent cold bioclim
  queries; lower it on a thin uplink.
- `TEMPUS_PROVIDERS_REANALYSIS_TIMEOUT` (default `30s`) — the whole budget of
  one range request: all its attempts **and** the backoff waits between them
  share it, so a slow first attempt or a long `Retry-After` can use it up
  before a retry starts. Size it for attempts plus backoff, not for one
  attempt.
- `TEMPUS_PROVIDERS_REANALYSIS_RETRY_ATTEMPTS` (default `3`) — retries of 429,
  5xx and network failures, with exponential backoff from 2 s (or the
  `Retry-After` the bucket sends).
- `TEMPUS_PROVIDERS_REANALYSIS_LIST_TTL` (default `1h`) — how long a directory
  listing, and with it the ETags that pin cached metadata, is trusted. The
  newest chunk files are rewritten daily; a longer TTL means fewer listings but
  a later view of the newest hours.

Every request that goes over the wire is a client span in the trace (see
[Observability](../reference/observability.md#tracing-opentelemetry)); that is
the place to look when a query is slow.

## Disk and memory

The archive is **not** mirrored wholesale. tempus writes exactly one file: the
cache (`TEMPUS_CACHE_PATH`, default `./data/cache.bolt`, `/data/cache.bolt` in
the image). It holds the archive months that queries actually touched (one
month of one variable at one model cell, about 3 KB) and finished results.

- **Memory:** the store keeps directory listings and up to 4096 file headers
  and 4096 lookup-table blocks per process (bounded LRU caches). A restart only
  costs some extra range requests.
- **Cache size:** `TEMPUS_CACHE_MAX_BYTES` (default 256 MiB, `0` = unbounded)
  caps the live entries of either backend; going over it evicts the least
  recently used ones. Final data is kept for a year at most, data younger than
  `TEMPUS_PROVIDERS_REANALYSIS_MATURITY` (default `72h`) for an hour at most
  (see [Caching model](../explanation/caching.md)).
- **Disk:** the cap bounds the entries, not the file. bbolt never returns freed
  pages to the file system — the file stays at its high-water mark, a little
  above the cap in steady state, and reuses the space. Plan the volume for the
  cap plus some slack.
- **What fills it:** archive months are shared by every point in the same
  model cells; results are per exact point (coordinate, elevation, hour, and
  aggregate's `gddBase`). Bioclim results key on the coordinate rounded to
  0.0001° (about 11 m), the exact elevation and the reference period; its
  30-year reads bypass the archive-month level. Clients sending fractional
  GNSS heights produce more result entries than ones sending whole metres.
- **Reclaiming space:** stop tempus and delete the file; it is a cache, nothing
  in it is authoritative. `TEMPUS_CACHE_TYPE=memory` avoids the file entirely
  at the price of a cold cache after every restart.

## When the bucket fails

The bucket comes without an SLA. tempus treats its failures as temporary and
says so per provider, never as an HTTP error of the whole query:

| What happens | Provider status in the response |
|---|---|
| Network error, timeout, 429 or 5xx after all retries | `unavailable`, `retryable: true`, `retryAfter: "30s"` |
| The hour is not in the archive yet (the newest IFS hours) | `unavailable`, `retryable: true`, `retryAfter: "2h0m0s"` (weather) or `"6h0m0s"` (aggregate) |
| A file is corrupt or uses an unknown format version or codec | `error`, `retryable: false` — loud on purpose, see below |

Clients follow the [retry recipe](../explanation/retry-semantics.md); cached
results keep being served during an outage.

A permanent `error` is not always the bucket: it is also the answer to a
request the provider cannot serve (for example an instant in the future, or an
invalid bioclim reference period). Classified provider
errors are not logged; read the provider's `error` field in the response.
Only when that message comes from the decoder or the store (`omfile`,
`omstore`: unknown format version or codec, corrupt chunk) does the data no
longer decode as expected — typically Open-Meteo changed the file layout. Then
run the conformance check below; do not just raise retries.

## Attribution

Every weather, aggregate and bioclim feature carries the CC-BY attribution of
ECMWF, the Copernicus Climate Change Service and Open-Meteo in its `license`
block. A deployment that shows or republishes values must keep it visible — see
[Attribution requirement](../explanation/attribution.md) and the
[attribution chain](../explanation/data-source.md#attribution-chain).

## Check conformance with the Open-Meteo archive

tempus re-implements the Open-Meteo archive's `best_match` (cell selection,
mixing, height correction, derived variables). An opt-in live check compares
it with the Open-Meteo archive API, which serves only as a black-box oracle:

```bash
make om-conformance                     # 20 seeded points
TEMPUS_OM_POINTS=100 make om-conformance
```

It runs three tests, all against the public bucket and API (no secrets):

- `TestLive_ThirtyYears` (`internal/adapters/omstore`) — 30 years of one cell,
  cold and warm, with request and byte counts.
- `TestConformance_WestPalearctic` (`internal/adapters/reanalysis`) —
  seeded points in the West Palearctic (30–72° N, 25° W–50° E, elevations
  0–2500 m drawn independently of the terrain) × four windows: a day in 2005
  (ERA5/ERA5-Land only), the start of ECMWF IFS on 1 January 2017, the same day
  last year, and the ten days before the last three. Hourly temperature,
  humidity, precipitation, weather code, wind speed and cloud cover must
  **equal** the API's values after rounding to the API's decimals. One API call
  per point and window; the seed is fixed, so a run is reproducible and
  `TEMPUS_OM_POINTS` only extends the sample.
- `TestLive_ReanalysisMatchesOpenMeteoArchive` (same package) — curated edge
  cases (coast, date line, Greenwich, mountains, southern hemisphere), hourly
  and daily.

The log shows the first 20 misses of each test with point, window, variable
and hour; the per-variable and per-window counts below it include every
comparison, and count misses of one step in the last decimal apart from larger
ones. To see the misses of one window in detail, rerun with fewer points. Any miss fails
the sweep. A miss that appears only in the `recent` window can mean the
archive revised its newest IFS hours between the two reads; rerun before
investigating. A miss in old windows points at a change on Open-Meteo's side
or a regression in tempus — run the check after every change to
`internal/domain/reanalysis`, `internal/adapters/reanalysis`,
`internal/adapters/omstore` or `internal/adapters/omfile`.
