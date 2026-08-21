package app

import (
	"time"

	"github.com/jobrunner/tempus/internal/adapters/aggregate"
	"github.com/jobrunner/tempus/internal/adapters/astronomy"
	"github.com/jobrunner/tempus/internal/adapters/bioclim"
	"github.com/jobrunner/tempus/internal/adapters/openmeteo"
	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/config"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// buildRegistry decides which providers this configuration runs. It is separate
// from New because "which providers exist and under what conditions" is its own
// question, told once, rather than four conditionals inside the composition root.
func buildRegistry(cfg *config.Config, cache output.Cache, clk output.Clock) *application.Registry {
	registry := application.NewRegistry()

	if cfg.Providers.OpenMeteo.Enabled {
		registry.Register(cachedWeatherProvider(cfg, cache, clk))
	}

	// Sun and moon are pure computations: no external call, no cache, and they
	// work for any date (past or future).
	registry.Register(astronomy.NewSun())
	registry.Register(astronomy.NewMoon())

	// Weather aggregates (antecedent precipitation, day extrema, GDD). Fetches a
	// time range from Open-Meteo; registered without the caching decorator
	// because its output also depends on the per-request gddBase override, which
	// the cache key does not capture.
	if cfg.Providers.Aggregate.Enabled && cfg.Providers.OpenMeteo.Enabled {
		registry.Register(aggregate.New(aggregate.Options{
			ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
			ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
			Timeout:         cfg.Providers.OpenMeteo.Timeout,
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			Clock:           clk,
		}))
	}

	// Bioclim (19 BIO variables + Köppen-Geiger) from ERA5 monthly normals.
	// Time-independent for a location+period, so it caches per coordinate itself
	// (its own cache key includes the reference period, ignoring the instant).
	if cfg.Providers.Bioclim.Enabled && cfg.Providers.OpenMeteo.Enabled {
		registry.Register(bioclim.New(bioclim.Options{
			ArchiveBaseURL: cfg.Providers.OpenMeteo.ArchiveBaseURL,
			// The 30-year daily fetch is large; allow more time than a single-hour
			// call. The query timeout still governs overall via context.
			Timeout: 60 * time.Second,
			Cache:   cache,
		}))
	}

	return registry
}

// cachedWeatherProvider is the Open-Meteo provider behind the caching decorator.
func cachedWeatherProvider(cfg *config.Config, cache output.Cache, clk output.Clock) output.FeatureProvider {
	om := openmeteo.New(openmeteo.Options{
		ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
		ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
		Timeout:         cfg.Providers.OpenMeteo.Timeout,
		ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
		Clock:           clk,
	})
	return application.NewCachingProvider(om, cache, clk, application.CachingOptions{
		Version:         "1",
		ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
		MatureTTL:       365 * 24 * time.Hour,
		ImmatureTTL:     time.Hour,
		LatLonPrecision: 2,
	})
}
