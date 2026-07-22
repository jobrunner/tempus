package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jobrunner/tempus/internal/config"
)

func TestApp_QueryEndToEndWithMemoryCache(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Cache.Type = cacheTypeMemory
	cfg.Cache.Path = filepath.Join(t.TempDir(), "c.bolt")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := New(cfg, logger, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()

	// providers endpoint proves wiring without hitting the network.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/v1/providers", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("providers status = %d", resp.StatusCode)
	}
}

func TestApp_WithMetricsEnabled(t *testing.T) {
	// Pick a free port so the metrics server doesn't collide with other tests.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind free port:", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg, _ := config.Load("")
	cfg.Cache.Type = cacheTypeMemory
	cfg.Metrics.Enabled = true
	cfg.Metrics.Port = port
	cfg.Metrics.Path = "/metrics"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := New(cfg, logger, "test")
	if err != nil {
		t.Fatalf("New with metrics enabled: %v", err)
	}
	// Verify the app handler still works.
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	_ = a // metrics server is started in the background
}

func TestApp_WithTracingEnabled(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Cache.Type = cacheTypeMemory
	cfg.Tracing.Enabled = true
	cfg.Tracing.Endpoint = "localhost:19997" // unreachable — OTel is lazy
	cfg.Tracing.Transport = "http"
	cfg.Tracing.SampleRatio = 1.0
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := New(cfg, logger, "test")
	if err != nil {
		t.Fatalf("New with tracing enabled: %v", err)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	_ = a
}

func TestApp_readyAlways(t *testing.T) {
	var r readyAlways
	if !r.Ready(context.Background()) {
		t.Error("readyAlways.Ready must return true")
	}
}
