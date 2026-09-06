package app

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/adapters/aggregate"
	"github.com/jobrunner/tempus/internal/adapters/astronomy"
	"github.com/jobrunner/tempus/internal/adapters/bioclim"
	"github.com/jobrunner/tempus/internal/adapters/omhttp"
	"github.com/jobrunner/tempus/internal/adapters/openmeteo"
	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/config"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// omClients are the per-adapter HTTP clients sharing one rate limiter and one
// daily budget. Each client carries its own cost weight and timeout.
type omClients struct {
	weather   *http.Client
	aggregate *http.Client
	bioclim   *http.Client
}

// buildOpenMeteoClients wires the shared throttling transport. The limiter and
// budget are shared across all Open-Meteo traffic; weights differ per call
// type because Open-Meteo weighs long archive ranges as multiple calls.
func buildOpenMeteoClients(cfg *config.Config, clk output.Clock) omClients {
	om := cfg.Providers.OpenMeteo
	// Burst 10 keeps a single interactive query (≤4 calls) latency-free while
	// the sustained rate stays under the free-tier minute limit.
	limiter := rate.NewLimiter(rate.Limit(float64(om.RatePerMinute)/60.0), 10)
	budget := omhttp.NewBudget(om.DailyBudget, clk)
	client := func(weight int, timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: omhttp.NewTransport(omhttp.Options{
				Limiter:       limiter,
				Budget:        budget,
				Weight:        weight,
				RetryAttempts: om.RetryAttempts,
			}),
		}
	}
	return omClients{
		weather:   client(om.Weights.Weather, om.Timeout),
		aggregate: client(om.Weights.Aggregate, om.Timeout),
		// The 30-year daily fetch is large; allow more time than a single-hour
		// call (matches the previous hardcoded bioclim timeout).
		bioclim: client(om.Weights.Bioclim, 60*time.Second),
	}
}

// buildRegistry decides which providers this configuration runs. It is separate
// from New because "which providers exist and under what conditions" is its own
// question, told once, rather than four conditionals inside the composition root.
func buildRegistry(cfg *config.Config, cache output.Cache, clk output.Clock) *application.Registry {
	registry := application.NewRegistry()
	clients := buildOpenMeteoClients(cfg, clk)

	if cfg.Providers.OpenMeteo.Enabled {
		registry.Register(cachedWeatherProvider(cfg, cache, clk, clients.weather))
	}

	// Sun and moon are pure computations: no external call, no cache, and they
	// work for any date (past or future).
	registry.Register(astronomy.NewSun())
	registry.Register(astronomy.NewMoon())

	// Weather aggregates: cached like the weather provider; the per-request
	// gddBase override is part of the cache key via KeyParams.
	if cfg.Providers.Aggregate.Enabled && cfg.Providers.OpenMeteo.Enabled {
		agg := aggregate.New(aggregate.Options{
			ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
			ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
			Timeout:         cfg.Providers.OpenMeteo.Timeout,
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			Clock:           clk,
			HTTPClient:      clients.aggregate,
		})
		registry.Register(application.NewCachingProvider(agg, cache, clk, application.CachingOptions{
			Version:         "1",
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			MatureTTL:       365 * 24 * time.Hour,
			ImmatureTTL:     time.Hour,
			LatLonPrecision: 2,
			KeyParams: func(req domain.QueryRequest) string {
				if req.GDDBaseCelsius == nil {
					return ""
				}
				return fmt.Sprintf("gdd=%.2f", *req.GDDBaseCelsius)
			},
		}))
	}

	// Bioclim (19 BIO variables + Köppen-Geiger) from ERA5 monthly normals.
	// Time-independent for a location+period, so it caches per coordinate itself
	// (its own cache key includes the reference period, ignoring the instant).
	if cfg.Providers.Bioclim.Enabled && cfg.Providers.OpenMeteo.Enabled {
		registry.Register(bioclim.New(bioclim.Options{
			ArchiveBaseURL: cfg.Providers.OpenMeteo.ArchiveBaseURL,
			// The 30-year daily fetch is large; allow more time than a single-hour
			// call. The query timeout still governs overall via context. The
			// Timeout field is superseded by clients.bioclim's own timeout once
			// HTTPClient is set, but is kept as the adapter's documented default.
			Timeout:    60 * time.Second,
			Cache:      cache,
			HTTPClient: clients.bioclim,
		}))
	}

	return registry
}

// cachedWeatherProvider is the Open-Meteo provider behind the caching decorator.
func cachedWeatherProvider(cfg *config.Config, cache output.Cache, clk output.Clock, client *http.Client) output.FeatureProvider {
	om := openmeteo.New(openmeteo.Options{
		ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
		ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
		Timeout:         cfg.Providers.OpenMeteo.Timeout,
		ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
		Clock:           clk,
		HTTPClient:      client,
	})
	return application.NewCachingProvider(om, cache, clk, application.CachingOptions{
		Version:         "1",
		ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
		MatureTTL:       365 * 24 * time.Hour,
		ImmatureTTL:     time.Hour,
		LatLonPrecision: 2,
	})
}
