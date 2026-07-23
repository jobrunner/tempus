# Future datetimes

A query's `datetime` **may** be in the future. Whether the future is served
depends on the provider — it is decided per provider, not by rejecting the
whole request:

- **Astronomy providers** (`sun`, `moon`) are pure computations and work for any
  date, past or future. Predicting next month's moon phase or a future
  sunrise is a first-class use-case.
- **The weather provider** (`open-meteo`) serves only past/present hours. For a
  future instant it reports a **non-retryable error** in `providers[]`
  (`status: "error"`, `retryable: false`) and contributes no feature. The
  request still returns `200 OK`.

```json
{
  "features": [ { "properties": { "kind": "sun" } }, { "properties": { "kind": "moon" } } ],
  "providers": [
    {"id": "open-meteo", "kind": "weather", "status": "error", "retryable": false},
    {"id": "sun", "kind": "sun", "status": "ok"},
    {"id": "moon", "kind": "moon", "status": "ok"}
  ]
}
```

## Rationale

Rejecting the entire request for a future datetime would deny the astronomy
features, which are perfectly well-defined for any date. Encoding the weather
provider's limitation as a per-provider status instead keeps the idempotent
contract intact: the client sees exactly which providers could and could not
serve the requested instant, and the weather limitation is non-retryable (it
will never become available by retrying).

For weather specifically, the past/present boundary keeps the caching model
simple (archived data is immutable) and avoids modelling forecast uncertainty,
which the response envelope does not currently represent.

## The boundary

The weather provider compares the requested datetime against the current
wall-clock time (injected via the `Clock` output port, which is mockable in
tests). Astronomy providers apply no such boundary.
