package app

import (
	"context"
	"time"

	httpapi "github.com/jobrunner/tempus/internal/adapters/http"
	"github.com/jobrunner/tempus/internal/adapters/metrics"
	"github.com/jobrunner/tempus/internal/adapters/telemetry"
)

// metricsShutdownTimeout bounds the metrics server's graceful shutdown.
const metricsShutdownTimeout = 5 * time.Second

// wireObservability sets up tracing and the metrics server as configured and
// returns the server options carrying the tracer. Both are optional: with
// tracing disabled the options carry no TracerProvider at all, and the HTTP
// server skips the tracing middleware when it is nil (see setupRoutes).
// Anything that needs closing is appended to the app's closers, which is why
// this is a method rather than a free function.
func (a *App) wireObservability(version string) (httpapi.Options, error) {
	opts := httpapi.Options{ServiceName: "tempus", Version: version}
	opts.Batch = httpapi.BatchLimits{
		MaxPoints:     a.cfg.Query.Batch.MaxPoints,
		MaxSyncPoints: a.cfg.Query.Batch.MaxSyncPoints,
	}

	if a.cfg.Tracing.Enabled {
		tp, shutdown, err := telemetry.NewTracerProvider(context.Background(), a.cfg.Tracing, "tempus")
		if err != nil {
			return httpapi.Options{}, err
		}
		opts.TracerProvider = tp
		a.closers = append(a.closers, func() error { return shutdown(context.Background()) })
	}

	if a.cfg.Metrics.Enabled {
		if err := a.startMetricsServer(); err != nil {
			return httpapi.Options{}, err
		}
	}
	return opts, nil
}

// startMetricsServer runs the metrics endpoint in the background and registers
// its shutdown. A serve error is logged, not returned: the metrics endpoint
// failing must not take the service down with it.
func (a *App) startMetricsServer() error {
	srv, err := metrics.New(a.cfg.Metrics)
	if err != nil {
		return err
	}
	go func() {
		if err := srv.Start(); err != nil {
			a.logger.Error("metrics server error", "error", err)
		}
	}()
	a.closers = append(a.closers, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
		defer cancel()
		return srv.Shutdown(ctx)
	})
	return nil
}
