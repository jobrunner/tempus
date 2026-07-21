# Retry and error semantics

## Always HTTP 200 on valid input

For any request with valid parameters (lat, lon, a non-future datetime, and a
valid timezone), tempus returns HTTP `200 OK`. Per-provider errors are encoded
**inside** the response body in `providers[].status`, not as HTTP error codes.

This is an intentional design decision that enables **idempotent clients**:

- The client can retry the whole request without special-casing provider errors.
- Retries are safe because the query is deterministic: the same inputs produce
  the same (or better) results as more providers recover.
- Logging and tracing pipelines can record a single status code (`200`) and
  inspect the provider statuses for detail.

## Provider status values

| `status` | Meaning |
|---|---|
| `ok` | Provider returned data successfully |
| `unavailable` | Provider is temporarily down or rate-limiting; the client should retry |
| `error` | Provider returned an unexpected error; retry unlikely to help |

## The `retryable` flag

When `status` is `unavailable`, `retryable: true` signals that a later retry
**should** produce a result. The optional `retryAfter` field carries an RFC 3339
timestamp as a hint for when to retry. In the absence of `retryAfter`, clients
should apply exponential back-off starting from a few seconds.

When `status` is `error`, `retryable` will be `false` (or absent). Retrying will
not help and may be wasteful.

## Client retry recipe

```
repeat:
  result = GET /api/v1/query?...
  for each provider in result.providers:
    if provider.retryable:
      wait until max(now + backoff, provider.retryAfter)
      goto repeat
  return result   # all providers either ok or permanently errored
```

Because the endpoint is idempotent (same inputs → same or better outputs), this
loop is safe to run client-side without any server-side session state.
