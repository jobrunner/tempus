// Package metrics provides a Prometheus-backed OpenTelemetry MeterProvider and
// an HTTP server that exposes the /metrics endpoint for scraping.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/jobrunner/tempus/internal/config"
)

// Server wraps a Prometheus MeterProvider and a lightweight HTTP server that
// serves the /metrics scrape endpoint.
type Server struct {
	provider *sdkmetric.MeterProvider
	httpSrv  *http.Server
}

// New builds a Prometheus-backed MeterProvider and wires the scrape server.
// Call Start to begin serving and Shutdown to drain.
func New(cfg config.MetricsConfig) (*Server, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	path := cfg.Path
	if path == "" {
		path = "/metrics"
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	addr := ":" + strconv.Itoa(cfg.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Server{
		provider: provider,
		httpSrv:  httpSrv,
	}, nil
}

// Provider returns the underlying MeterProvider for registering instruments.
func (s *Server) Provider() *sdkmetric.MeterProvider {
	return s.provider
}

// Handler returns the HTTP handler for the metrics endpoint. Useful in tests to
// exercise the endpoint without binding a real port.
func (s *Server) Handler() http.Handler {
	return s.httpSrv.Handler
}

// Start begins serving the metrics endpoint. It blocks until the server is
// closed; call it in a goroutine and use Shutdown to stop it.
func (s *Server) Start() error {
	if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown flushes the MeterProvider and stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	httpErr := s.httpSrv.Shutdown(ctx)
	providerErr := s.provider.Shutdown(ctx)
	if httpErr != nil {
		return httpErr
	}
	return providerErr
}
