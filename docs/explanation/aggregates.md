# Weather aggregates (time-window)

A single hourly value rarely explains a field observation on its own — the
*preceding* conditions and the season-to-date warmth often matter more. The
weather-aggregate provider (`kind: "aggregate"`) computes these from an
Open-Meteo archive range and is aimed at enriching occurrence records (e.g.
biological collection data) with the weather/climate context of a find.

Unlike a derived feature, it fetches a **time range** (not just the fund hour),
so it is a full provider. It is registered **without the cache** because its
output also depends on the per-request `gddBase` override, which the cache key
does not capture.

## What it returns

| Group | Property | Meaning |
|---|---|---|
| `precipitation` | `last24h`, `last72h`, `last120h` | Precipitation sums (mm) over the 24 h / 72 h / 5 d ending at the instant |
| `temperatureDay` | `min`, `max`, `mean` | The fund day's temperature extrema (°C, UTC day) |
| `growingDegreeDays` | `value`, `baseCelsius`, `since`, `days`, `method` | Accumulated GDD (°C·d) and how it was computed |

## Growing-degree-days

GDD accumulates heat available for ectotherm/plant development. tempus uses the
**average method** with a lower floor and no upper cutoff:

```
GDD = Σ max(0, (Tmax_day + Tmin_day) / 2 − base)
```

- **Base temperature** defaults to **10 °C** (a common all-purpose value for
  insects). Override it per request with the `gddBase` query parameter
  (°C, range [-50, 50]) — e.g. `?gddBase=7` for a species with a lower
  developmental threshold.
- **Accumulation window** runs from the start of the growing year to the
  instant: **1 January** in the northern hemisphere, **1 July** in the southern
  hemisphere (so the season does not begin in the middle of the southern
  summer). The `since` field reports the start date used.

## Windows and disciplines

The window choices mirror established practice: **24 / 72 h** are the standard
flood/antecedent-rainfall windows, **5 d (120 h)** is the hydrological
antecedent-precipitation window (SCS curve-number "antecedent moisture
condition"), and season-to-date **GDD** is the agrometeorology/phenology
standard for development timing.

## Data source and latency

Aggregates are computed from the **Open-Meteo archive (ERA5, Copernicus/ECMWF)**;
antecedent precipitation for very recent dates falls back to the forecast
endpoint. Because ERA5 has a few days' latency, aggregates for finds in the last
few days may have reduced coverage; for historical collection records (the
primary use-case) this does not apply. Future instants are rejected with a
non-retryable provider status.
