# Data source

The weather, aggregate and bioclim providers all read from one source: the
**reanalysis archive** that Open-Meteo publishes as open data. tempus does not
call the Open-Meteo HTTP API (neither `archive-api` nor `api.open-meteo.com`).
It reads the archive's files straight from the public bucket and computes the
point values itself, following the same rules the Open-Meteo archive uses.

## What is still Open-Meteo — and what is not

| | Open-Meteo? | Where it lives in tempus |
|---|---|---|
| **Dataset** — ECMWF IFS, ERA5 and ERA5-Land, regridded and published by Open-Meteo | yes | read from `https://openmeteo.s3.amazonaws.com` (AWS Open Data, `s3://openmeteo`): no API key, no quota, no rate limit |
| **File format** — Open-Meteo's OM format (compressed chunks + lookup table) | yes, the format | decoded by `internal/adapters/omfile`, a clean-room decoder written from the format description and black-box tests; its provenance is documented in `internal/adapters/omfile/FORMAT.md` |
| **Algorithm** — how the archive's `best_match` turns grid series into a point value | yes, as the reference | reimplemented in `internal/domain/reanalysis` and `internal/adapters/reanalysis` from its observable behaviour, verified against the live API |
| **Service** — the HTTP API, its throttling and its free-tier terms | **no** | not used at all |

The model surface heights each grid cell refers to (`static/HSURF.om` of every
model) are embedded in the binary (`internal/adapters/reanalysis/data`, about
4.9 MB; refreshed with `scripts/fetch-model-elevations.sh`, checksums in
`data/MANIFEST`), so cell selection needs no extra request.

## How a point value is made

For a coordinate, an **elevation** (metres above sea level) and a time range:

1. **Three models in priority order.** ECMWF IFS (9 km, octahedral grid
   O1280; available from 2017), ERA5-Land (0.1°, temperature and dew point
   only) and ERA5 (0.25°). A lower-priority model only fills hours the higher
   one leaves empty, so recent dates come from IFS and older ones from
   ERA5-Land/ERA5. At the seam the last hours before a gap are blended linearly
   so the series has no jump. There is no separate "archive" and "forecast"
   source any more: IFS reaches up to the current day (23:00 UTC).
2. **Terrain-aware cell selection.** On each grid the nearest cell is taken if
   its model height is within 100 m of the requested elevation; otherwise a
   neighbouring land cell with a better trade-off between horizontal distance
   and height difference is chosen. That is why the request elevation is
   required: in mountains, the nearest cell can be hundreds of metres too high
   or too low.
3. **Lapse-rate correction.** Temperature and dew point are shifted from the
   selected cell's model height to the requested elevation with 0.0065 K/m.
4. **Derived variables.** Relative humidity (from temperature and dew point),
   the WMO weather code (from cloud cover, precipitation and snowfall) and the
   wind speed (from its components) are computed after the correction, as the
   archive does.
5. **Daily values** (aggregate, bioclim): the daily minimum and maximum are
   the extrema of a Hermite curve through the hourly temperatures, the mean is
   the mean of the 24 hours, precipitation the sum of hours 00–23 UTC.

The result matches the Open-Meteo archive's `best_match` for the same
coordinate and elevation. An opt-in live check compares both (`make
om-conformance`, see
[Operate the reanalysis archive](../how-to/operate-reanalysis.md#check-conformance-with-the-open-meteo-archive)).

Every weather feature reports what it was computed for: `properties.elevation`
is the request elevation and `properties.modelCells` lists the grid points
(model, lat, lon, height the series refers to) in priority order. The feature
geometry is the request coordinate, not a grid cell.

## Which elevation to send

The height of the location **above sea level** (orthometric), in metres, in the
range −500 … 9000 — as read from a topographic map or a DEM, or the "altitude
above mean sea level" of a GNSS receiver. Not the ellipsoidal height a raw GNSS
fix reports, which differs from it by the geoid undulation (several tens of
metres in Europe). A wrong elevation shifts temperatures by 0.65 °C per 100 m
and can change which model cell is selected.

## Cost per request

Reads are HTTP range requests against the bucket; tempus fetches only the bytes
a series needs and keeps file metadata (headers, lookup tables) in memory.
Measured against the bucket (us-west-2):

- **One weather day** for one location: tens of kB.
- **30 years for bioclim**, cold: about 2.6 s, about 90 range requests and
  about 1.2 MB per variable. With the metadata already in memory: about 1 s.

Nothing is mirrored to disk; the only persistent state is the existing result
cache (see [Caching model](caching.md)). The knobs are under
[`providers.reanalysis`](../reference/configuration.md#providers-reanalysis-archive).

## Attribution chain

The values are produced by ECMWF (IFS) and by the Copernicus Climate Change
Service (C3S, ERA5 and ERA5-Land, run by ECMWF on behalf of the EU) and
regridded and distributed by Open-Meteo. All three require attribution, and
every weather, aggregate and bioclim feature carries it
(`domain.ReanalysisLicense()`):

```json
"license": {
  "name": "CC-BY 4.0",
  "url": "https://open-meteo.com/en/license",
  "attribution": "Weather data: ECMWF IFS (CC-BY 4.0) and Copernicus Climate Change Service ERA5/ERA5-Land (contains modified Copernicus Climate Change Service information), processed and distributed by Open-Meteo.com (CC-BY 4.0 open data)"
}
```

The aggregate and bioclim features append their method source (and, for
bioclim, the reference period) to that attribution. See
[Attribution requirement](attribution.md) for how the block is enforced.
