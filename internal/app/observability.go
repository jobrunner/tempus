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
// returns the server options carrying the tracer. Both are optional; when
// tracing is disabled the options keep the no-op tracer, so downstream code
// never has to nil-check. Anything that needs closing is appended to the app's
// closers, which is why this is a method rather than a free function.
func (a *App) wireObservability(version string) (httpapi.Options, error) {
	opts := httpapi.Options{ServiceName: "tempus", Version: version}

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
