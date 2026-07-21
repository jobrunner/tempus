# tempus

**tempus** is a coordinate + time feature-query service. Given a location and a
point in time, it fetches weather and other environmental data from one or more
registered providers, assembles a GeoJSON feature collection, and returns it with
per-feature mandatory attribution.

## Why tempus?

- **Always HTTP 200 on valid input** — per-provider errors are encoded in the
  response envelope, not the status code. An idempotent client retries transparently.
- **Mandatory attribution** — every `Feature` carries a `license` block (`name`,
  `url`, `attribution`). Serving a feature without a complete license is a contract
  violation that the service enforces at the port boundary.
- **No future queries** — the service only returns data for datetimes that are not
  in the future; future datetimes are rejected with `400 Bad Request`.

## Quick links

| Section | What's inside |
|---|---|
| [Tutorials](tutorials/index.md) | Getting started — run the service, make your first query |
| [How-to guides](how-to/index.md) | Practical recipes: run locally, query past weather, configure providers |
| [Reference](reference/http-api.md) | Full HTTP API contract, configuration keys, observability |
| [Explanation](explanation/architecture.md) | Design decisions, retry semantics, caching model |
