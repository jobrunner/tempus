package app

import (
	"context"
	"log/slog"

	"github.com/jobrunner/tempus/internal/config"
)

// App is the composition root. Fully wired in Task 16.
type App struct {
	cfg    *config.Config
	logger *slog.Logger
}

// New builds the application from config. Expanded in Task 16.
func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	return &App{cfg: cfg, logger: logger}, nil
}

// Run blocks until ctx is canceled. Expanded in Task 16.
func (a *App) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
