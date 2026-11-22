package app

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/jobrunner/tempus/internal/adapters/aggregate"
	"github.com/jobrunner/tempus/internal/adapters/astronomy"
	"github.com/jobrunner/tempus/internal/adapters/bioclim"
	"github.com/jobrunner/tempus/internal/adapters/bucketio"
	"github.com/jobrunner/tempus/internal/adapters/omstore"
	"github.com/jobrunner/tempus/internal/adapters/reanalysis"
	"github.com/jobrunner/tempus/internal/adapters/weather"
	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/config"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// buildReanalysis wires the reanalysis archive: the open-data store behind a
// retrying (and, with tp, traced) transport, and the service that selects,
// corrects and mixes the model series. Nil when the archive is disabled.
func buildReanalysis(cfg *config.Config, tp trace.TracerProvider) output.Reanalysis {
	r := cfg.Providers.Reanalysis
	if !r.Enabled {
		return nil
	}
	client := &http.Client{
		Timeout: r.Timeout,
		// No limiter and no budget: the bucket has no quota. The transport
		// still retries 5xx/network failures and traces each attempt.
		Transport: bucketio.NewTransport(bucketio.Options{RetryAttempts: r.RetryAttempts, TracerProvider: tp}),
	}
	return reanalysis.NewService(omstore.New(omstore.Options{
		BaseURL:     r.BaseURL,
		HTTPClient:  client,
		MaxParallel: r.MaxParallel,
		ListTTL:     r.ListTTL,
	}))
}

// buildRegistry decides which providers this configuration runs. It is separate
// from New because "which providers exist and under what conditions" is its own
// question, told once, rather than four conditionals inside the composition root.
// archive is nil when the reanalysis archive is disabled; every provider that
// needs it goes away with it.
func buildRegistry(cfg *config.Config, cache output.Cache, clk output.Clock, archive output.Reanalysis) *application.Registry {
	registry := application.NewRegistry()

	// Sun and moon are pure computations: no external call, no cache, and they
	// work for any date (past or future).
	registry.Register(astronomy.NewSun())
	registry.Register(astronomy.NewMoon())

	if archive == nil {
		return registry
	}
	maturity := cfg.Providers.Reanalysis.Maturity

	registry.Register(application.NewCachingProvider(
		weather.New(weather.Options{Reanalysis: archive, Clock: clk}),
		cache, clk, cachingOptions(maturity, nil)))

	// Weather aggregates: the per-request gddBase override is part of the cache
	// key via KeyParams.
	if cfg.Providers.Aggregate.Enabled {
		agg := aggregate.New(aggregate.Options{Reanalysis: archive, Clock: clk})
		registry.Register(application.NewCachingProvider(agg, cache, clk, cachingOptions(maturity,
			func(req domain.QueryRequest) string {
				if req.GDDBaseCelsius == nil {
					return ""
				}
				return fmt.Sprintf("gdd=%.2f", *req.GDDBaseCelsius)
			})))
	}

	// Bioclim (19 BIO variables + Köppen-Geiger) from 30-year normals. Time-
	// independent for a location+period, so it caches per coordinate itself
	// (its own key includes the reference period and elevation, not the instant).
	if cfg.Providers.Bioclim.Enabled {
		registry.Register(bioclim.New(bioclim.Options{Reanalysis: archive, Cache: cache}))
	}

	return registry
}

// cachingOptions: hours older than maturity are final and cached for a year;
// younger ones (the archive's newest IFS hours are still revised) for an hour.
// Version 2 retires every entry computed from the Open-Meteo API.
func cachingOptions(maturity time.Duration, keyParams func(domain.QueryRequest) string) application.CachingOptions {
	return application.CachingOptions{
		Version:         "2",
		ArchiveDelay:    maturity,
		MatureTTL:       365 * 24 * time.Hour,
		ImmatureTTL:     time.Hour,
		LatLonPrecision: 2,
		KeyParams:       keyParams,
	}
}
