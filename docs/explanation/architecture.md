# Architecture

## Hexagonal ports and adapters

tempus follows the hexagonal (ports and adapters) architecture pattern:

```
          ┌────────────────────────────────────────┐
          │                 domain                 │
          │   Feature, License, QueryResult,       │
          │   ProviderStatus (pure Go, no deps)    │
          └────────────┬───────────────┬───────────┘
                       │               │
          input ports  │               │  output ports
          (driving)    │               │  (driven)
                       ▼               ▼
          ┌────────────────┐   ┌───────────────────┐
          │ FeatureService │   │  FeatureProvider   │
          │ ProviderLister │   │  Reanalysis        │
          │ HealthChecker  │   │  Clock, Cache      │
          └────┬───────────┘   └──────────┬────────┘
               │                          │
               ▼                          ▼
     ┌──────────────────┐      ┌──────────────────────┐
     │  HTTP adapter    │      │  Provider adapters   │
     │  (gorilla/mux)   │      │  (open-meteo, …)     │
     └──────────────────┘      └──────────────────────┘
```

The weather (`open-meteo`), `aggregate` and `bioclim` providers do not talk to
the network themselves. They read point series through the `Reanalysis` output
port (`internal/ports/output/reanalysis.go`), implemented by
`internal/adapters/reanalysis` (cell selection, elevation correction, model
mixing) on top of `internal/adapters/omstore` (HTTP range reads from the
open-data bucket) and `internal/adapters/omfile` (the OM file decoder). See
[Data source](data-source.md).

The domain package has no framework or infrastructure dependencies. The
application layer (`internal/application/`) composes providers, applies the
caching decorator, enforces the no-future rule, and assembles the response
envelope.

## Relationship to Ortus-style queries

tempus draws on the Ortus feature-query pattern: given a coordinate, return a
collection of GeoJSON features enriched with domain data. The key difference is
that Ortus answers "which polygon does this point fall in?" (point-in-polygon
against pre-packaged local data), while tempus **reads or computes** data at query
time — from the reanalysis archive or by pure computation — with caching. Both share the same response
envelope shape and the same mandatory attribution contract.

## Provider abstraction

Every data source implements the `FeatureProvider` port:

```go
type FeatureProvider interface {
    ID() string
    Kind() string
    Fetch(ctx context.Context, req QueryRequest) (domain.ProviderResult, error)
    License() domain.License
}
```

The application layer calls each registered provider, wraps them in a caching
decorator (transparent to the provider), and folds their results into the
`QueryResult` envelope. A provider returning an error causes its entry in
`providers[]` to carry `status: unavailable` rather than propagating an HTTP 500.
