# Attribution requirement

Every `Feature` in a tempus response carries a `license` block:

```json
"license": {
  "name": "Open-Meteo",
  "url": "https://open-meteo.com",
  "attribution": "Weather data by Open-Meteo.com"
}
```

All three fields are **required**, and the service enforces it at the port
boundary rather than trusting providers.

`FeatureService` validates `Feature.License` on every result that crosses the
`FeatureProvider` port, and on every feature a `FeatureDeriver` produces. A block
missing `name`, `url` or `attribution` — or carrying only whitespace — is treated
as a **permanent** provider fault:

- the feature is **not** served,
- the provider appears in `providers[]` with `"status": "error"` and
  `"retryable": false`, and an `error` message naming the missing fields,
- the response is still HTTP 200, like every other per-provider fault here.

Non-retryable is deliberate: retrying returns the same empty block, so asking the
client to try again would only postpone the problem.

For a deriver, one incomplete block rejects that deriver's whole batch. A
partially attributed batch is not a meaningful thing to publish.

!!! note "No startup check"

    Validation happens at request time only. There is no startup pass that
    inspects each provider's statically declared `Attribution()`, so a provider
    whose licence is wrong is discovered on the first request that uses it, not
    at boot. Earlier versions of this page claimed such a startup check existed;
    it never did.

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
