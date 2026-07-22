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
