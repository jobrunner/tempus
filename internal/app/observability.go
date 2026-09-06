package app

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"

	httpapi "github.com/jobrunner/tempus/internal/adapters/http"
	"github.com/jobrunner/tempus/internal/adapters/metrics"
	"github.com/jobrunner/tempus/internal/adapters/omhttp"
	"github.com/jobrunner/tempus/internal/adapters/telemetry"
)

// metricsShutdownTimeout bounds the metrics server's graceful shutdown.
const metricsShutdownTimeout = 5 * time.Second

// omhttpMeterName is the instrumentation scope for the daily-budget gauges.
const omhttpMeterName = "tempus/omhttp"

// wireObservability sets up tracing and the metrics server as configured and
// returns the server options carrying the tracer. Both are optional: with
// tracing disabled the options carry no TracerProvider at all, and the HTTP
// server skips the tracing middleware when it is nil (see setupRoutes). budget
// is nil when Open-Meteo is disabled; the budget gauges are then skipped.
// Anything that needs closing is appended to the app's closers, which is why
// this is a method rather than a free function.
func (a *App) wireObservability(version string, budget *omhttp.Budget) (httpapi.Options, error) {
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
		if err := a.startMetricsServer(budget); err != nil {
			return httpapi.Options{}, err
		}
	}
	return opts, nil
}

// startMetricsServer runs the metrics endpoint in the background and registers
// its shutdown. A serve error is logged, not returned: the metrics endpoint
// failing must not take the service down with it.
func (a *App) startMetricsServer(budget *omhttp.Budget) error {
	srv, err := metrics.New(a.cfg.Metrics)
	if err != nil {
		return err
	}
	if budget != nil {
		if err := registerBudgetGauges(srv.Provider().Meter(omhttpMeterName), budget); err != nil {
			return err
		}
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

// registerBudgetGauges exposes the daily Open-Meteo call budget as two
// observable gauges so operators can see remaining budget without deriving it
// from logs.
func registerBudgetGauges(meter metric.Meter, budget *omhttp.Budget) error {
	spent, err := meter.Int64ObservableGauge(
		"tempus.openmeteo.daily_budget.spent",
		metric.WithDescription("Weighted Open-Meteo calls spent from today's daily budget."),
		metric.WithUnit("{weighted_call}"),
	)
	if err != nil {
		return err
	}
	limit, err := meter.Int64ObservableGauge(
		"tempus.openmeteo.daily_budget.limit",
		metric.WithDescription("Configured daily weighted-call budget for Open-Meteo."),
		metric.WithUnit("{weighted_call}"),
	)
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(spent, int64(budget.Spent()))
		o.ObserveInt64(limit, int64(budget.Limit()))
		return nil
	}, spent, limit)
	return err
}
