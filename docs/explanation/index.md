# Explanation

Background reading: design decisions and the reasoning behind them.

| Page | Contents |
|---|---|
| [Architecture](architecture.md) | Hexagonal architecture, port/adapter design |
| [Retry and error semantics](retry-semantics.md) | Why always-200, per-provider status, retry contract |
| [Data source](data-source.md) | The reanalysis archive: what is still Open-Meteo, how a point value is made, attribution chain |
| [Attribution requirement](attribution.md) | Why every feature carries a mandatory license block |
| [Caching model](caching.md) | Data maturity, cache keying, expiry |
| [No future datetimes](no-future-datetimes.md) | Why future queries are rejected |
