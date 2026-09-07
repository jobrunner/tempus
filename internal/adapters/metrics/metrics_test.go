package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/config"
)

// scrape performs a GET against the server's handler and returns the body.
func scrape(t *testing.T, srv *Server) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", rr.Code)
	}
	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatalf("read scrape body: %v", err)
	}
	return string(body)
}

// New must preserve the documented standard Go runtime metrics
// (go_goroutines, process_cpu_seconds_total, ...) even though it registers
// its OTel exporter on a private prometheus.Registry rather than the
// package-global default registerer.
func TestMetricsServer_ExposesStandardRuntimeMetrics(t *testing.T) {
	srv, err := New(config.MetricsConfig{Enabled: true, Port: 0, Path: "/metrics"})
	if err != nil {
		t.Fatalf("metrics.New: %v", err)
	}
	body := scrape(t, srv)
	for _, want := range []string{"go_goroutines", "process_"} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing %q (standard collectors not registered on the private registry)\n---\n%s", want, body)
		}
	}
}

// Two independent Server instances (e.g. across tests in one process) must
// not collide over metric names, even though each now registers its own
// standard-collector set on its own private registry.
func TestMetricsServer_MultipleInstancesDoNotCollide(t *testing.T) {
	srv1, err := New(config.MetricsConfig{Enabled: true, Port: 0, Path: "/metrics"})
	if err != nil {
		t.Fatalf("metrics.New (1): %v", err)
	}
	srv2, err := New(config.MetricsConfig{Enabled: true, Port: 0, Path: "/metrics"})
	if err != nil {
		t.Fatalf("metrics.New (2): %v", err)
	}
	for i, srv := range []*Server{srv1, srv2} {
		_ = scrape(t, srv) // scrape() itself asserts HTTP 200 (Gather() didn't error)
		_ = i
	}
}

func TestMetricsServer_ServesMetricsEndpoint(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    0, // picked by OS, but we test via httptest
		Path:    "/metrics",
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("metrics.New: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Use the handler directly via httptest to avoid binding a real port.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 at /metrics, got %d", rr.Code)
	}
}

func TestMetricsServer_Shutdown(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled: true,
		Port:    0,
		Path:    "/metrics",
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("metrics.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
