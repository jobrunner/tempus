# Bioclimatic variables & Köppen-Geiger

The bioclim provider (`kind: "bioclim"`) turns weather into **climate**: the 19
WorldClim bioclimatic variables (BIO1–BIO19) and the Köppen-Geiger class, so an
occurrence record can be characterised by the long-term climate of its place —
in the same variables the species-distribution/ecology community uses (WorldClim
/ CHELSA), making the enrichment comparable to those datasets.

## How it is computed

tempus reads the daily series of the reanalysis archive (ECMWF IFS from 2017,
ERA5-Land and ERA5 before — see [Data source](data-source.md)) for the
reference period, corrected to the request's elevation, aggregates it to **12-month climate normals** (mean daily min/max/mean
temperature per calendar month; mean monthly precipitation total), then:

- **BIO1–BIO19** via the standard WorldClim/ANUCLIM definitions (quarters are
  the 12 overlapping runs of three consecutive months).
- **Köppen-Geiger** via the decision tree of Beck et al. (2018), from the same
  monthly temperature and precipitation normals.

## Köppen-Geiger rules

tempus follows Beck et al. (2018), *Sci. Data* 5:180214, **Table 2** and its
Methods text, with the inequalities exactly as written there. Where this differs
from the older Kottek et al. (2006) key that many maps use:

- **Half-years by temperature.** Summer is whichever of April–September and
  October–March is *warmer*, not the hemisphere's nominal summer. This matters
  near the equator.
- **No `As`.** The tropical sub-types are only `Af`, `Am` and `Aw`; `Aw` covers
  a dry season in either half-year.
- **s vs. w.** When both the dry-summer and the dry-winter criterion hold, the
  wetter half-year decides: `s` if winter gets more precipitation, else `w`.
- **Strict thresholds.** E is `Thot ≤ 10 °C`; `Tmon10` counts months
  *above* 10 °C; the B threshold switches only at *more than* 70 % seasonal
  precipitation; `d` applies only when neither `a` nor `b` does.
- **C/D at 0 °C** (Kottek uses −3 °C), and B takes precedence over E.

The sourced criteria, the reference stations (WMO 1991–2020 normals) that pin
the classifier, and the earlier deviations are documented in the
[Köppen-Geiger reference dossier](../research/koppen-reference.md).

The values are **time-independent** for a location and period, so the provider
**caches per coordinate + elevation + period** with a long TTL (one year). The
first query for a location triggers a ~30-year read (about 2.6 s cold, about
1 s once the archive's file metadata is in memory); subsequent queries (e.g. many records at the
same locality) are served from cache.

!!! note "Reanalysis-based, not identical to WorldClim"
    The *definitions* match WorldClim, but the *values* come from the
    reanalysis (mostly ERA5/ERA5-Land for 30-year normals), so they
    are close to — not bit-identical with — WorldClim (interpolated station data)
    or CHELSA. Attribution states this explicitly.

## Köppen-Geiger boundary flag

The Köppen main-class boundaries are knife-edge isotherms — the C/D
(temperate/cold) boundary is the **mean temperature of the coldest month = 0 °C**.
Near such a boundary, the reanalysis and a high-resolution station/downscaled Köppen raster
(what a GeoTIFF map is built from) routinely differ by ~1–1.5 °C in winter, which
is enough to flip the **main class** (e.g. Cfb ↔ Dfb). This is the usual reason a
raster map and tempus disagree at one point even for the same period — neither is
wrong; they are different data at a boundary.

So the `koppen` object also returns:

- **`coldestMonthMeanC`** — the value that decides the C/D boundary, so you can
  see how close to 0 °C the point is.
- **`borderline`** + **`adjacent`** — set when the point is within ~1.5 °C of a
  main-class temperature boundary, naming the class a slightly colder/warmer
  dataset would give. A location reading `Cfb` with `borderline: true` and
  `adjacent.code: Dfb` explains a Dfb tile in a station-based raster.

## Reference period (and historical records)

BIO variables are 30-year climate normals. A find in 1954 should be described by
a normal contemporaneous with the find, not by today's climate. So the period is
**auto-selected from the datetime's year**:

- Find ≥ 1991 → 1991–2020; 1961–1990 → that period; ~1954 → 1940–1969 (the
  1931–1960 normal clamped to the ERA5 floor), etc.
- Clamped to the **ERA5 floor of 1940** (earliest available), so pre-1940 finds
  use 1940–1969, reported transparently in `referencePeriod`.
- Override with `?refPeriod=YYYY-YYYY` (e.g. `1970-2000` for a direct WorldClim
  comparison).

This is why the same Berlin coordinate can classify as **Cfb** for a 2010 find
but **Dfb** for a 1954 find — the mid-century normal was colder.

## The variables

| | Temperature (°C, except noted) | | Precipitation (mm, except noted) |
|---|---|---|---|
| BIO1 | Annual mean temperature | BIO12 | Annual precipitation |
| BIO2 | Mean diurnal range | BIO13 | Precipitation of wettest month |
| BIO3 | Isothermality (%) | BIO14 | Precipitation of driest month |
| BIO4 | Temperature seasonality (std×100) | BIO15 | Precipitation seasonality (CV %) |
| BIO5 | Max temp of warmest month | BIO16 | Precipitation of wettest quarter |
| BIO6 | Min temp of coldest month | BIO17 | Precipitation of driest quarter |
| BIO7 | Temperature annual range | BIO18 | Precipitation of warmest quarter |
| BIO8 | Mean temp of wettest quarter | BIO19 | Precipitation of coldest quarter |
| BIO9 | Mean temp of driest quarter | | |
| BIO10 | Mean temp of warmest quarter | | |
| BIO11 | Mean temp of coldest quarter | | |

## Data source and limits

Computed from the **reanalysis archive (ECMWF IFS, Copernicus ERA5 and
ERA5-Land)**, read from Open-Meteo's open-data bucket — no API and no quota
(see [Data source](data-source.md)). A 30-year read for a new location costs
about 90 range requests and about 1.2 MB per variable; a bucket failure that
survives the retries is reported as a retryable provider status. Caching per
coordinate, elevation and period keeps repeat queries cheap. Because the
temperatures are corrected to the request elevation, two records at the same
coordinate but different heights get different normals.
