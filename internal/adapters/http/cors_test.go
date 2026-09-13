package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newCORSTestServer builds a server whose CORS middleware allows the given
// origins. Requests must go through Handler(), not Router(): CORS wraps the
// router from the outside so that preflights reach it at all (see corsHandler).
func newCORSTestServer(t *testing.T, origins ...string) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, stubBatchService{}, stubProviders{}, stubHealth{}, fixedClock{}, logger,
		Options{CORSAllowedOrigins: origins})
}

func doCORS(srv *Server, method, path, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestCORS_AllowedOriginGetsHeaders(t *testing.T) {
	srv := newCORSTestServer(t, "https://a.test")
	rr := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://a.test")

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://a.test" {
		t.Errorf("Allow-Origin = %q, want the requesting origin echoed back", got)
	}
	if got := rr.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, must contain Origin so caches don't serve one origin's response to another", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 — CORS must not change the normal response", rr.Code)
	}
}

func TestCORS_DisallowedOriginGetsNoHeaders(t *testing.T) {
	srv := newCORSTestServer(t, "https://a.test")
	rr := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://evil.test")

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty for a non-allowed origin", got)
	}
	// The request itself still succeeds; it is the browser that blocks the read.
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestCORS_WildcardMatchesSubdomainsOnly(t *testing.T) {
	srv := newCORSTestServer(t, "https://*.example.test")

	if got := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://sub.example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "https://sub.example.test" {
		t.Errorf("subdomain: Allow-Origin = %q, want it allowed", got)
	}
	if got := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("bare domain: Allow-Origin = %q, want empty — *.example.test covers subdomains only", got)
	}
	if got := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://evil-example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("suffix lookalike: Allow-Origin = %q, want empty", got)
	}
}

// The load-bearing test: POST /api/v1/query/batch sends Content-Type:
// application/json, so every browser sends an OPTIONS preflight first. gorilla/
// mux does NOT run r.Use middleware for a method that matches no route — such a
// request goes to the MethodNotAllowedHandler, outside the chain. If CORS were
// registered via r.Use, this preflight would answer 405 with no CORS headers and
// the batch endpoint would be unusable from a browser.
func TestCORS_PreflightOnBatchPostSucceeds(t *testing.T) {
	srv := newCORSTestServer(t, "https://a.test")
	rr := doCORS(srv, http.MethodOptions, "/api/v1/query/batch", "https://a.test")

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://a.test" {
		t.Errorf("preflight Allow-Origin = %q, want the origin echoed", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Errorf("Allow-Methods = %q, must contain POST — the batch endpoint is a POST", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("Allow-Headers = %q, must contain Content-Type — batch posts JSON", got)
	}
}

func TestCORS_DisabledLeavesResponsesUntouched(t *testing.T) {
	srv := newCORSTestServer(t) // no origins configured
	rr := doCORS(srv, http.MethodGet, "/api/v1/providers", "https://a.test")

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want no CORS headers when unconfigured", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
