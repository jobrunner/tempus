package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jobrunner/tempus/internal/adapters/metrics"
	"github.com/jobrunner/tempus/internal/config"
)

// scrape performs a GET against the metrics handler and returns the body.
func scrape(t *testing.T, handler http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", rr.Code)
	}
	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatalf("read scrape body: %v", err)
	}
	return string(body)
}

// The metrics endpoint must keep the standard Go runtime metrics that
// metrics.New's private-registry rewrite has to register explicitly
// (go_goroutines and process_* only self-register onto the global
// DefaultRegisterer via client_golang's package init()).
func TestMetricsServer_KeepsRuntimeMetrics(t *testing.T) {
	srv, err := metrics.New(config.MetricsConfig{Enabled: true, Port: 0, Path: "/metrics"})
	if err != nil {
		t.Fatalf("metrics.New: %v", err)
	}
	body := scrape(t, srv.Handler())
	for _, want := range []string{"go_goroutines", "process_"} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing %q\n---\n%s", want, body)
		}
	}
}

// App.New with metrics enabled must start the metrics server without erroring.
func TestApp_WithMetricsEnabled_Starts(t *testing.T) {
	cfg := testConfig(t)
	cfg.Metrics.Enabled = true
	cfg.Metrics.Port = 0
	cfg.Metrics.Path = "/metrics"

	a, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New with metrics enabled: %v", err)
	}
	t.Cleanup(func() {
		for _, c := range a.closers {
			_ = c()
		}
	})
}
