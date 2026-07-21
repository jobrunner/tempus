# How to run the service

## Run from source

```bash
go build -o tempus ./cmd/tempus
./tempus
```

The binary reads configuration from environment variables (prefix `TEMPUS_`) and
optional config file. See [configure providers and cache](configure-providers.md)
for the full variable reference.

## Run with Docker

```bash
docker run --rm \
  -p 8080:8080 \
  -e TEMPUS_PROVIDERS_OPENMETEO_ENABLED=true \
  ghcr.io/jobrunner/tempus:latest
```

## Verify startup

```bash
# Liveness probe
curl -f http://localhost:8080/health/live

# Readiness probe (all providers initialised)
curl -f http://localhost:8080/health/ready
```

Both return HTTP 200 on success.

## Ports

| Port | Purpose |
|---|---|
| 8080 (default) | HTTP API (`TEMPUS_SERVER_PORT`) |
| 9090 (default) | Prometheus metrics (`TEMPUS_METRICS_PORT`) — when metrics enabled |

## Logging

By default tempus logs JSON to stdout. To switch to human-readable text:

```bash
TEMPUS_LOGGING_FORMAT=text ./tempus
```

Log level: `TEMPUS_LOGGING_LEVEL=debug|info|warn|error` (default: `info`).
