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
// retrying (and, with tp, traced) transport, its month blocks cached per
// model cell, and the service that selects, corrects and mixes the model
// series. Nil when the archive is disabled.
func buildReanalysis(cfg *config.Config, cache output.Cache, clk output.Clock, tp trace.TracerProvider) output.Reanalysis {
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
	store := omstore.New(omstore.Options{
		BaseURL:     r.BaseURL,
		HTTPClient:  client,
		MaxParallel: r.MaxParallel,
		ListTTL:     r.ListTTL,
	})
	return reanalysis.NewService(application.NewCachingSeriesReader(store, cache, clk, application.SeriesCacheOptions{
		Version:     "1",
		Maturity:    r.Maturity,
		MatureTTL:   365 * 24 * time.Hour,
		ImmatureTTL: time.Hour,
		MaxSpan:     400 * 24 * time.Hour,
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

// resultMaturityMargin delays "final" for results past the maturity the
// series cache uses. An archive block is never served once one of its hours
// has turned final, but a result computed from it just before that moment is
// written moments after; the margin keeps such a result on the short TTL. It
// only has to exceed one query (query.timeout), not a fill.
const resultMaturityMargin = 10 * time.Minute

// cachingOptions: hours older than maturity are final and cached for a year;
// younger ones (the archive's newest IFS hours are still revised) for an hour.
// Version 3 keys on the exact point; version 2 entries (0.01° keys) carried
// the geometry of whichever nearby point asked first.
func cachingOptions(maturity time.Duration, keyParams func(domain.QueryRequest) string) application.CachingOptions {
	return application.CachingOptions{
		Version:      "3",
		ArchiveDelay: maturity + resultMaturityMargin,
		MatureTTL:    365 * 24 * time.Hour,
		ImmatureTTL:  time.Hour,
		KeyParams:    keyParams,
	}
}
