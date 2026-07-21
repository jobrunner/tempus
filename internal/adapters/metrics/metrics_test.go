package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/config"
)

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
