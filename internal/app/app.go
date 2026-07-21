package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	boltcache "github.com/jobrunner/tempus/internal/adapters/cache/bolt"
	memcache "github.com/jobrunner/tempus/internal/adapters/cache/memory"
	"github.com/jobrunner/tempus/internal/adapters/clock"
	httpapi "github.com/jobrunner/tempus/internal/adapters/http"
	"github.com/jobrunner/tempus/internal/adapters/openmeteo"
	"github.com/jobrunner/tempus/internal/application"
	"github.com/jobrunner/tempus/internal/config"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// App is the composition root: it owns adapters and the server lifecycle.
type App struct {
	cfg     *config.Config
	logger  *slog.Logger
	server  *httpapi.Server
	closers []func() error
}

type readyAlways struct{}

func (readyAlways) Ready(context.Context) bool { return true }

// New wires adapters into ports.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	a := &App{cfg: cfg, logger: logger}

	cache, closer, err := buildCache(cfg.Cache)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		a.closers = append(a.closers, closer)
	}

	clk := clock.System{}
	registry := application.NewRegistry()

	if cfg.Providers.OpenMeteo.Enabled {
		om := openmeteo.New(openmeteo.Options{
			ArchiveBaseURL:  cfg.Providers.OpenMeteo.ArchiveBaseURL,
			ForecastBaseURL: cfg.Providers.OpenMeteo.ForecastBaseURL,
			Timeout:         cfg.Providers.OpenMeteo.Timeout,
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			Clock:           clk,
		})
		cached := application.NewCachingProvider(om, cache, clk, application.CachingOptions{
			Version:         "1",
			ArchiveDelay:    cfg.Providers.OpenMeteo.ArchiveDelay,
			MatureTTL:       365 * 24 * time.Hour,
			ImmatureTTL:     time.Hour,
			LatLonPrecision: 2,
		})
		registry.Register(cached)
	}

	features := application.NewFeatureService(registry, logger, cfg.Query.Timeout)
	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	a.server = httpapi.NewServer(addr, features, registry, readyAlways{}, clk, logger, httpapi.Options{ServiceName: "tempus"})
	return a, nil
}

func buildCache(cfg config.CacheConfig) (output.Cache, func() error, error) {
	switch cfg.Type {
	case "memory":
		return memcache.New(), nil, nil
	default: // "disk" or anything else
		c, err := boltcache.Open(cfg.Path)
		if err != nil {
			return nil, nil, err
		}
		return c, c.Close, nil
	}
}

// Handler exposes the router for tests.
func (a *App) Handler() http.Handler { return a.server.Router() }

// Run starts the server and shuts down gracefully on ctx cancellation.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := a.server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), a.cfg.Server.ShutdownTimeout)
		defer cancel()
		err := a.server.Shutdown(shutCtx)
		for _, c := range a.closers {
			_ = c()
		}
		return err
	}
}
