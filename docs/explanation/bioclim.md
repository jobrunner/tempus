# Bioclimatic variables & Köppen-Geiger

The bioclim provider (`kind: "bioclim"`) turns weather into **climate**: the 19
WorldClim bioclimatic variables (BIO1–BIO19) and the Köppen-Geiger class, so an
occurrence record can be characterised by the long-term climate of its place —
in the same variables the species-distribution/ecology community uses (WorldClim
/ CHELSA), making the enrichment comparable to those datasets.

## How it is computed

tempus fetches the daily ERA5 archive (Open-Meteo) for the reference period,
aggregates it to **12-month climate normals** (mean daily min/max/mean
temperature per calendar month; mean monthly precipitation total), then:

- **BIO1–BIO19** via the standard WorldClim/ANUCLIM definitions (quarters are
  the 12 overlapping runs of three consecutive months).
- **Köppen-Geiger** via the decision tree of Beck et al. (2018), from the same
  monthly temperature and precipitation normals.

The values are **time-independent** for a location and period, so the provider
**caches per coordinate + period** with a long TTL. The first query for a
location triggers a ~30-year fetch; subsequent queries (e.g. many records at the
same locality) are served from cache.

!!! note "ERA5-based, not identical to WorldClim"
    The *definitions* match WorldClim, but the *values* come from ERA5, so they
    are close to — not bit-identical with — WorldClim (interpolated station data)
    or CHELSA. Attribution states this explicitly.

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

Computed from the **Open-Meteo archive (ERA5, Copernicus/ECMWF)**. The 30-year
fetch is large; repeated bursts for many distinct coordinates can hit Open-Meteo
rate limits (HTTP 429), reported as a retryable provider status. Per-coordinate
caching keeps repeat queries cheap.
