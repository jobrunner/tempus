# Sun and moon (astronomy)

tempus computes the position of the sun and moon — plus rise/set, twilight and
lunar phase — directly from the queried coordinate and instant. These are
**pure-computation providers**: they make no external request, need no cache,
and work for any date (past or future). The mathematics lives in the domain
layer (`internal/domain/solar.go`, `internal/domain/lunar.go`) and is unit
tested against published reference values.

## Sun (`kind: "sun"`)

Computed via the NOAA solar-position approximation (Spencer's Fourier series
for declination and the equation of time).

| Property | Meaning |
|---|---|
| `elevationDeg` | Sun elevation above the horizon |
| `azimuthDeg` | Azimuth, clockwise from true north |
| `zenithDeg` | Solar zenith angle (90 − elevation) |
| `sunrise`, `sunset` | Event times (RFC3339 UTC), or `null` on polar day/night |
| `solarNoon`, `solarNoonElevationDeg` | Time and elevation of the daily maximum |
| `dayLengthMinutes` | Daylight duration between sunrise and sunset |
| `twilight` | Civil (−6°), nautical (−12°), astronomical (−18°) dawn/dusk |
| `lightPhase` | Bilingual regime: day / twilight (each band) / night |

The twilight bands and photoperiod (`dayLengthMinutes`) are the values commonly
used in marine biology to describe the light regime (e.g. diel vertical
migration cues).

Attribution: **NOAA Solar Calculator** — "Sonnenstand berechnet
(NOAA-Näherung), tempus".

## Moon (`kind: "moon"`)

Computed via Jean Meeus, *Astronomical Algorithms* (2nd ed., ch. 47), using the
periodic-term tables for lunar longitude, latitude and distance. Positions are
**geocentric** (no topocentric parallax correction), accurate to roughly an
arcminute — more than sufficient for the intended use.

| Property | Meaning |
|---|---|
| `elevationDeg`, `azimuthDeg` | Moon position (azimuth clockwise from north) |
| `distanceKm` | Earth–moon centre distance |
| `illuminationPct` | Illuminated fraction of the disc |
| `phaseAngleDeg` | Phase angle (0 = full, 180 = new) |
| `ageDays` | Days since the last new moon |
| `phase` | Bilingual phase name (Neumond … abnehmende Sichel) |
| `moonrise`, `moonset` | Event times (RFC3339 UTC), or `null` when no crossing |

Rise and set are found by scanning the moon's altitude across the local day
(see [Which day?](#which-day)) and
interpolating the crossing of the standard altitude (+0.125°, combining mean
parallax, refraction and the lunar semidiameter).

Attribution: **Meeus, Astronomical Algorithms** — "Mondstand berechnet nach
Meeus (geozentrische Näherung), tempus".

## Which day?

Sunrise, sunset, solar noon, twilight and moonrise/moonset belong to the day
the instant falls into **at the coordinate**, not to its UTC calendar day: the
solar day whose solar noon is nearest to the instant (at most twelve hours
away). That day runs from halfway after the previous solar noon to halfway
before the next — about 24 hours, but not exactly, since solar noons drift by
up to half a minute a day — so consecutive local days follow each other
without gaps. The moon is searched over the same span.

Far from Greenwich the two differ. In Thailand (longitude 100°E, UTC+7) an
instant at 06:30 local time on 19 May is 23:30 UTC on 18 May; the events
returned are those of the 19 May morning — sunrise just before the instant —
not those around 18 May's UTC noon. Westwards, an evening at 18:30 local in
Mexico (00:30 UTC the next day) gets that evening's sunset. Near Greenwich the
day is the same as the UTC day for most of it; only instants within the
minutes between local midnight and UTC midnight can switch to the neighbour.

The instant need not lie between the events: at 02:00 local time it belongs to
the day whose dawn is still ahead, at 23:00 to the one whose dusk is past. Do
not derive day or night from the events; use `lightPhase` (or `elevationDeg`),
which are exact for the instant and not tied to any day, as is `azimuthDeg`.

## Time zones

All event times are returned in **UTC** (RFC3339). The frontend converts them
to the selected local time zone for display; the API contract stays UTC-only.
