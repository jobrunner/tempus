# Day/Night from Solar Position

## Why Not the Weather Provider's `is_day`?

Open-Meteo's Archive API (ERA5) returns `is_day=0` for **all hours** of
historical dates. This means querying e.g. `lat=49, lon=10,
datetime=1954-07-23T11:00Z` would show "Nacht" (night) at 11:00 midday
— clearly wrong.

## What tempus Does Instead

tempus computes day/night itself from the sun's **elevation angle** using
the [NOAA Spencer/NOAA approximation][noaa]:

1. The fractional day-of-year and UTC hour determine the **equation of time**
   and the sun's **declination**.
2. The **true solar time** at the queried longitude gives the **hour angle**.
3. The **solar elevation** (altitude above horizon) is derived from the standard
   spherical-astronomy formula.
4. Day is declared when the elevation exceeds **−0.833°** — the conventional
   sunrise/sunset threshold that accounts for atmospheric refraction and the
   sun's apparent radius.

This is a ~1° approximation, accurate enough to distinguish day from night for
any latitude and any date including historical dates back centuries.

## Response Fields

| Property      | Type    | Description                                      |
|---------------|---------|--------------------------------------------------|
| `isDay`       | boolean | `true` if sun is above −0.833° at the instant    |
| `isDaySource` | string  | Citation: "Tag/Nacht aus Sonnenstand berechnet (NOAA-Näherung), tempus" |

## Accuracy

The NOAA approximation is accurate to within roughly 1° of arc. Sunrise/sunset
timing is typically within 1–2 minutes of the precise ephemeris value. This is
well within acceptable limits for a day/night indicator.

[noaa]: https://www.esrl.noaa.gov/gmd/grad/solcalc/solareqns.PDF
