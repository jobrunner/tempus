# Multi-arch (linux/amd64, linux/arm64) image for tempus.
# Build:  docker buildx build --platform linux/amd64,linux/arm64 -t tempus:dev .
#
# All base images are pinned by immutable digest (no floating tags, no :latest).
# The runtime pulls in NO apk packages: CA roots are copied from the builder and
# the IANA tz database is embedded into the binary (-tags timetzdata), so there
# is nothing unpinnable fetched at build time.

# ---- builder: runs natively on $BUILDPLATFORM, cross-compiles to the target ----
FROM --platform=$BUILDPLATFORM golang:1.25.12-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS builder

# Use the toolchain shipped in the image; never fetch one over the network.
ENV GOTOOLCHAIN=local
WORKDIR /build

# Automatic platform args provided by buildx.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_DATE=unknown

# Dependencies first (go.mod/go.sum are exact-pinned) for layer caching.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# CGO off -> fully static binary. timetzdata embeds the tz database so the
# runtime needs no tzdata package. trimpath + -s -w for a reproducible, lean build.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -tags timetzdata -trimpath \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_DATE}" \
      -o /out/tempus ./cmd/tempus

# ---- runtime: Alpine, per-arch image resolved from the pinned manifest list ----
FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

# CA roots for outbound HTTPS (e.g. Open-Meteo), taken from the builder image.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Non-root runtime user.
RUN addgroup -S app && adduser -S app -G app && mkdir -p /app && chown app:app /app
WORKDIR /app
COPY --from=builder /out/tempus /app/tempus
USER app

ENV TEMPUS_SERVER_HOST=0.0.0.0 \
    TEMPUS_SERVER_PORT=8080 \
    TEMPUS_LOGGING_FORMAT=json
EXPOSE 8080

# busybox wget ships in the Alpine base; /health/live needs no dependencies.
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q --spider http://localhost:8080/health/live || exit 1

ENTRYPOINT ["/app/tempus"]
