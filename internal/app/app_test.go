package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jobrunner/tempus/internal/config"
)

func TestApp_QueryEndToEndWithMemoryCache(t *testing.T) {
	cfg, _ := config.Load("")
	cfg.Cache.Type = "memory"
	cfg.Cache.Path = filepath.Join(t.TempDir(), "c.bolt")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	a, err := New(cfg, logger)
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
