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
| `growingDegreeDays` | `base5`, `base10`, `custom`, `since`, `days`, `method` | Accumulated GDD (°C·d) at fixed bases 5 and 10, plus the per-request base |
| `arrheniusThermalTime` | `value`, `referenceTempC`, `activationEnergyEv`, `days`, `method` | Species-agnostic, base-free thermal-time site index |

## Growing-degree-days

GDD accumulates heat available for ectotherm/plant development. tempus uses the
**average method** with a lower floor and no upper cutoff:

```
GDD = Σ max(0, (Tmax_day + Tmin_day) / 2 − base)
```

- **base5 and base10 are always computed.** Base 5 °C (cool-temperate) and
  10 °C (warm-season/general insect) are the two standard bases and are returned
  on every request. This deliberately prevents clients from silently changing a
  single configurable base over time and creating inconsistent series.
- A **per-request base** can be added with the `gddBase` query parameter
  (°C, range [-50, 50]) — e.g. `?gddBase=7` for a species with a specific
  developmental threshold. It appears under `custom` **in addition to** base5/base10.
- **Accumulation window** runs from the start of the growing year to the
  instant: **1 January** in the northern hemisphere, **1 July** in the southern
  hemisphere (so the season does not begin in the middle of the southern
  summer). The `since` field reports the start date used.

## Arrhenius thermal time (base-free site index)

GDD needs an arbitrary base temperature and is linear above it. The
`arrheniusThermalTime` index is **base-free** and **species-agnostic**: it
integrates the Boltzmann-Arrhenius reaction-rate response of temperature, which
has no threshold and captures the whole temperature range.

Per day it uses the 2-point rate mean of Tmin and Tmax, with the rate normalised
so a day at the reference temperature contributes 1.0:

```
w(T) = exp[ (E/k) · (1/Tref − 1/T) ]        (temperatures in Kelvin)
index = Σ_days ½·(w(Tmin) + w(Tmax))
```

- **E = 0.65 eV** — the Metabolic Theory of Ecology activation energy for
  metabolic/development rates (Brown et al. 2004; Gillooly et al. 2001), a
  universal (species-agnostic) value.
- **Tref = 20 °C** — the result is in **20 °C-equivalent days**. Crucially, Tref
  is *only a normalising constant*: it multiplies the whole sum by a fixed
  factor and never changes what is counted. So values computed at different
  references are interconvertible, and comparisons/ratios are Tref-independent —
  unlike a GDD base, this does **not** re-introduce the base-temperature problem.

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
