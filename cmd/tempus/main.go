package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jobrunner/tempus/internal/adapters/telemetry"
	"github.com/jobrunner/tempus/internal/app"
	"github.com/jobrunner/tempus/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger := setupLogger(cfg.Logging)
	slog.SetDefault(logger)

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("build app", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run", "error", err)
		os.Exit(1)
	}
}

func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	return slog.New(telemetry.NewSpanContextHandler(buildHandler(cfg, os.Stdout)))
}

func buildHandler(cfg config.LoggingConfig, w io.Writer) slog.Handler {
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.Level))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.NewTextHandler(w, opts)
	}
	return slog.NewJSONHandler(w, opts)
}
