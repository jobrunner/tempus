package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	boltcache "github.com/jobrunner/tempus/internal/adapters/cache/bolt"
	memcache "github.com/jobrunner/tempus/internal/adapters/cache/memory"
	"github.com/jobrunner/tempus/internal/adapters/clock"
	"github.com/jobrunner/tempus/internal/adapters/dewpoint"
	httpapi "github.com/jobrunner/tempus/internal/adapters/http"
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
	// background runs for as long as Run's context lives.
	background []func(context.Context)
}

const cacheTypeMemory = "memory"

type readyAlways struct{}

func (readyAlways) Ready(context.Context) bool { return true }

// New wires adapters into ports.
func New(cfg *config.Config, logger *slog.Logger, version string) (*App, error) {
	a := &App{cfg: cfg, logger: logger}

	cache, closer, err := buildCache(cfg.Cache)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		a.closers = append(a.closers, closer)
	}

	tp, err := a.startTracing()
	if err != nil {
		return nil, err
	}

	clk := clock.System{}
	archive, keepListed := buildReanalysis(cfg, cache, clk, tp)
	if keepListed != nil {
		a.background = append(a.background, keepListed)
	}
	registry := buildRegistry(cfg, cache, clk, archive)

	serverOpts, err := a.wireObservability(version, tp)
	if err != nil {
		return nil, err
	}

	derivers := []output.FeatureDeriver{dewpoint.New()}
	features := application.NewFeatureService(registry, derivers, logger, cfg.Query.Timeout)
	batch := application.NewBatchService(features, cfg.Query.Batch.Concurrency)
	addr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	a.server = httpapi.NewServer(addr, features, batch, registry, readyAlways{}, clk, logger, serverOpts)
	return a, nil
}

func buildCache(cfg config.CacheConfig) (output.Cache, func() error, error) {
	switch cfg.Type {
	case cacheTypeMemory:
		return memcache.New(cfg.MaxBytes), nil, nil
	default: // "disk" or anything else
		c, err := boltcache.Open(cfg.Path, boltcache.Options{MaxBytes: cfg.MaxBytes})
		if err != nil {
			return nil, nil, err
		}
		return c, c.Close, nil
	}
}

// Handler exposes the router for tests.
func (a *App) Handler() http.Handler { return a.server.Handler() }

// Run starts the server and shuts down gracefully on ctx cancellation.
func (a *App) Run(ctx context.Context) error {
	defer func() {
		for _, c := range a.closers {
			_ = c()
		}
	}()

	// Background tasks end when Run does, on every path — also when the
	// server fails to start while the caller's context lives on.
	bgCtx, stopBackground := context.WithCancel(ctx)
	defer stopBackground()
	for _, task := range a.background {
		go task(bgCtx)
	}

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
		return a.server.Shutdown(shutCtx)
	}
}
