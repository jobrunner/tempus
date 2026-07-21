# ---- builder ----
FROM golang:1.23-alpine AS builder
WORKDIR /build
ARG VERSION=dev
ARG BUILD_DATE=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
      -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_DATE}" \
      -o tempus ./cmd/tempus

# ---- runtime ----
FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app && mkdir -p /app && chown app:app /app
WORKDIR /app
COPY --from=builder /build/tempus /app/tempus
USER app
ENV TEMPUS_SERVER_HOST=0.0.0.0 \
    TEMPUS_SERVER_PORT=8080 \
    TEMPUS_LOGGING_FORMAT=json
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q --spider http://localhost:8080/health/live || exit 1
ENTRYPOINT ["/app/tempus"]
