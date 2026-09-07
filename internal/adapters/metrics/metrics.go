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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/jobrunner/tempus/internal/config"
)

// defaultMetricsPath is the Prometheus scrape path used when none is configured.
const defaultMetricsPath = "/metrics"

// Server wraps a Prometheus MeterProvider and a lightweight HTTP server that
// serves the /metrics scrape endpoint.
type Server struct {
	provider *sdkmetric.MeterProvider
	httpSrv  *http.Server
}

// New builds a Prometheus-backed MeterProvider and wires the scrape server.
// Call Start to begin serving and Shutdown to drain. Each Server owns its own
// prometheus.Registry rather than the package-global default registerer, so
// multiple Servers (e.g. across tests in one process) never collide over
// metric names.
func New(cfg config.MetricsConfig) (*Server, error) {
	reg := prometheus.NewRegistry()
	// client_golang's Go/process collectors self-register onto the package-global
	// DefaultRegisterer only via their own init()-time convenience wrapper; a
	// dedicated registry gets neither for free, so register them explicitly to
	// keep the documented go_goroutines/process_* output.
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(reg))
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	path := cfg.Path
	if path == "" {
		path = defaultMetricsPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

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
