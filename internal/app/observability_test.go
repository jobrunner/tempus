package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/metrics"
	"github.com/jobrunner/tempus/internal/adapters/omhttp"
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

type fakeTestClock struct{ now time.Time }

func (f *fakeTestClock) Now() time.Time { return f.now }

// registerBudgetGauges must expose both the spent and limit gauges, reading
// live values from the budget on each scrape.
func TestRegisterBudgetGauges_ExposesSpentAndLimit(t *testing.T) {
	srv, err := metrics.New(config.MetricsConfig{Enabled: true, Port: 0, Path: "/metrics"})
	if err != nil {
		t.Fatalf("metrics.New: %v", err)
	}

	clk := &fakeTestClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	budget := omhttp.NewBudget(100, clk)
	budget.TryReserve(37)

	if err := registerBudgetGauges(srv.Provider().Meter("tempus/omhttp"), budget); err != nil {
		t.Fatalf("registerBudgetGauges: %v", err)
	}

	body := scrape(t, srv.Handler())
	for _, want := range []string{
		"tempus_openmeteo_daily_budget_spent",
		"tempus_openmeteo_daily_budget_limit",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing %q\n---\n%s", want, body)
		}
	}
}

// App.New, with metrics enabled and Open-Meteo enabled, must register the
// budget gauges without erroring.
func TestApp_WithMetricsEnabled_RegistersBudgetGauges(t *testing.T) {
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
