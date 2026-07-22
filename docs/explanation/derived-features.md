# Derived Features

tempus can compute *derived* features from already-fetched provider data
without issuing additional external requests.

## Dew Point (Taupunkt)

The dew point is derived from the weather feature's `temperature2m` (°C) and
`relativeHumidity2m` (%) using the **Magnus-Tetens formula** with
Sonntag-1990 coefficients (a = 17.62, b = 243.12 °C):

```
γ = ln(RH/100) + a·T / (b + T)
Td = b·γ / (a − γ)
```

The result appears as a separate GeoJSON feature with `kind: "dewpoint"` and
a `dewPoint2m` property (°C, rounded to one decimal place).  The feature's
`license.attribution` traces back to the source weather provider so
attribution is always preserved.

If the weather provider has not yet returned data (e.g. archive delay), the
dew-point entry in `providers[]` has `status: "unavailable"` and
`retryable: true`.  If the required fields are present but invalid, the status
is `error` with `retryable: false`.

### Dew-point comfort classification

The dew-point feature also carries a **bilingual comfort classification** in
`comfort.de` / `comfort.en`, based on the standard meteorological dew-point
comfort scale:

| Dew point (°C) | Deutsch | English |
|---|---|---|
| ≤ 5 | sehr trocken | very dry |
| ≤ 10 | trocken | dry |
| ≤ 13 | sehr angenehm | very comfortable |
| ≤ 16 | angenehm | comfortable |
| ≤ 18 | leicht schwül | slightly humid |
| ≤ 21 | schwül | humid |
| ≤ 24 | sehr schwül | very humid |
| > 24 | drückend | oppressive |

Source: `comfortSource` property — "Taupunkt-Komfortskala (gängige
meteorologische Einteilung)"; see also
<https://en.wikipedia.org/wiki/Dew_point#Relationship_to_human_comfort>.

## Weather Code Descriptions (WMO Code Table 4677)

Each weather feature (kind `"weather"`) returned by the Open-Meteo provider
carries a numeric `weatherCode` (WMO Code Table 4677 subset). The feature now
also includes:

- **`weatherCodeDescription`** — a bilingual object `{"de": "...", "en": "..."}`
  with a human-readable description of the code (e.g. `{"de": "Bedeckt",
  "en": "Overcast"}` for code 3).  Present only for codes that appear in the
  Open-Meteo subset of WMO Code Table 4677; absent for unknown codes.
- **`weatherCodeSource`** — a string citing the origin:
  "WMO Code Table 4677 (WW – present weather); weather-interpretation codes as
  used by Open-Meteo" (URL: <https://open-meteo.com/en/docs>).
