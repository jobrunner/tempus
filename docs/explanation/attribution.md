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

!!! note "What is validated where"

    Request-time validation covers **`Feature.License`** — the block attached to
    the data actually returned. It does not look at `Attribution()`, the static
    block a provider declares and `/api/v1/providers` publishes; that one is
    never checked at runtime and there is no startup pass that inspects it.
    Earlier versions of this page claimed such a startup check existed; it never
    did.

    The static block is instead covered by a fitness test
    (`TestEveryRegisteredProviderDeclaresACompleteLicense` in `internal/app`),
    which asserts that every provider the composition root registers declares a
    complete licence. A misconfigured provider is a programming error, and
    failing CI is more useful than failing to boot in production.

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
licence via `Attribution() domain.License` (see
`internal/ports/output/provider.go`).

Caches validate in both directions, so a cached feature is attributed exactly
like a live one. This applies to `application.CachingProvider` and to the
bioclim provider's own one-year cache, which is registered directly rather than
wrapped. A feature with an incomplete block is never written to the cache,
because a mature entry lives for up to a year and would keep failing validation
long after the provider itself was fixed. A cached entry that fails validation
on read — one written before this was enforced, say — is treated as a miss and
refetched, so a corrected provider recovers on the next request rather than
after the TTL.
