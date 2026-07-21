# No future datetimes

tempus rejects any query whose `datetime` parameter is in the future with
`400 Bad Request`:

```json
{"error": "invalid_request", "message": "datetime must not be in the future"}
```

## Rationale

tempus is designed for **retrospective queries** — retrieving what conditions
were like at a place and time in the past. This is the core use-case: looking
up the weather for a hike you took last Saturday, or validating the conditions
at the site of a past event.

Accepting future datetimes would require:

1. **Forecast providers** — fundamentally different data, with uncertainty bounds
   and model-revision semantics that the response envelope does not currently model.
2. **Invalidation logic** — cached future data becomes stale as forecasts update.
3. **Complex retry semantics** — a "not yet available" status is different from
   "provider down".

Keeping the boundary at "now" keeps the caching model simple (past data is
immutable once archived) and the error contract clear (an invalid datetime is
always a client error, not a provider error).

## The boundary

The service compares the requested datetime against the current wall-clock time
(injected via the `Clock` output port, which is mockable in tests). A datetime
equal to "now" (within a small grace window) may be accepted by some providers
and rejected by others; in practice, the service treats "now" as future and
rejects it, directing clients to query once the moment has clearly passed.
