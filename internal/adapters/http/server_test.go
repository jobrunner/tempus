package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

type stubFeatures struct{ res domain.QueryResult }

func (s stubFeatures) Query(context.Context, domain.QueryRequest) (domain.QueryResult, error) {
	return s.res, nil
}

type stubProviders struct{}

func (stubProviders) Providers(context.Context) []input.ProviderInfo {
	return []input.ProviderInfo{{ID: "open-meteo", Kind: "weather", License: domain.License{Name: "CC-BY 4.0"}}}
}

type stubHealth struct{}

func (stubHealth) Ready(context.Context) bool { return true }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) }

func testServer() *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	res := domain.QueryResult{Features: []domain.Feature{}, Providers: []domain.ProviderStatus{{ID: "open-meteo", Status: "ok"}}}
	return NewServer(":0", stubFeatures{res}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{})
}

func TestHandleQuery_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body domain.QueryResult
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Providers) != 1 {
		t.Errorf("providers = %d, want 1", len(body.Providers))
	}
}

func TestHandleQuery_FutureIs400(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?lat=0&lon=0&datetime=2026-07-21T13:00:00Z", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleProviders_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestHandleIndex_OK(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	testServer().Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct == "" || !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html…", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "tempus") {
		t.Error("body does not contain 'tempus'")
	}
	if !strings.Contains(body, `id="lat"`) {
		t.Error("body does not contain lat input (id=\"lat\")")
	}
}
