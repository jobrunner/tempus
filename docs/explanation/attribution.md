# Attribution requirement

Every `Feature` in a tempus response carries a `license` block:

```json
"license": {
  "name": "Open-Meteo",
  "url": "https://open-meteo.com",
  "attribution": "Weather data by Open-Meteo.com"
}
```

All three fields are **required**. The service enforces this at the port boundary:
a `FeatureProvider` that returns a `Feature` with a missing or empty license field
is a contract violation detected at startup (if the license is statically known)
or at runtime (if the provider omits it).

## Why mandatory attribution?

Many open data sources require attribution as a condition of use (Open-Meteo, for
example, requires attribution on its free tier). Without the `license` block baked
into every feature, the downstream application must know which provider produced
which feature and look up the attribution separately. This is error-prone and easy
to forget.

By making attribution a first-class field on `Feature` — not a side channel — the
client always has everything it needs to display a correct attribution string,
regardless of which providers responded.

## Design contract

The `License` type in the domain:

```go
type License struct {
    Name        string `json:"name"`
    URL         string `json:"url"`
    Attribution string `json:"attribution"`
}
```

All three strings must be non-empty. A `FeatureProvider` declares its static
license via `License() domain.License`. The caching decorator preserves the
license through the cache layer, so cached features carry the same attribution
as live ones.
