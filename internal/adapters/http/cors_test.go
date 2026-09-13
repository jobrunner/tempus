package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
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

// doCORS issues a normal (non-preflight) GET against a representative endpoint;
// preflights go through doPreflight.
func doCORS(srv *Server, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

// doPreflight sends what a browser actually sends before a cross-origin POST:
// OPTIONS with both Origin and Access-Control-Request-Method.
func doPreflight(srv *Server, path, origin, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodOptions, path, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestCORS_AllowedOriginGetsHeaders(t *testing.T) {
	srv := newCORSTestServer(t, "https://a.test")
	rr := doCORS(srv, "https://a.test")

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
	rr := doCORS(srv, "https://evil.test")

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

	if got := doCORS(srv, "https://sub.example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "https://sub.example.test" {
		t.Errorf("subdomain: Allow-Origin = %q, want it allowed", got)
	}
	if got := doCORS(srv, "https://example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("bare domain: Allow-Origin = %q, want empty — *.example.test covers subdomains only", got)
	}
	if got := doCORS(srv, "https://evil-example.test").
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
	rr := doPreflight(srv, "/api/v1/query/batch", "https://a.test", http.MethodPost)

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
	rr := doCORS(srv, "https://a.test")

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want no CORS headers when unconfigured", got)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

// Copilot review finding: a scheme-qualified wildcard must not leak across
// schemes or ports. "https://*.example.test" is a promise about HTTPS on the
// default port, not about plaintext HTTP or an arbitrary port.
func TestCORS_WildcardRespectsSchemeAndPort(t *testing.T) {
	srv := newCORSTestServer(t, "https://*.example.test")

	for _, origin := range []string{
		"http://sub.example.test",       // wrong scheme
		"https://sub.example.test:8443", // unexpected port
	} {
		if got := doCORS(srv, origin).
			Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q: Allow-Origin = %q, want empty", origin, got)
		}
	}
	if got := doCORS(srv, "https://sub.example.test").
		Header().Get("Access-Control-Allow-Origin"); got != "https://sub.example.test" {
		t.Errorf("matching origin: Allow-Origin = %q, want it allowed", got)
	}
}

// Copilot review finding: only a real preflight (Origin + the
// Access-Control-Request-Method header) may be short-circuited with 204. A bare
// OPTIONS must keep reaching the router, as it did before CORS existed.
func TestCORS_BareOptionsStillReachesTheRouter(t *testing.T) {
	srv := newCORSTestServer(t, "https://a.test")

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/query", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code == http.StatusNoContent {
		t.Errorf("bare OPTIONS answered 204 by the CORS layer; it should fall through to the router (405)")
	}
}

// TestCORSPreflightForEveryWritingRoute derives its cases from the route table,
// so an endpoint added tomorrow is covered without touching this test. It gates
// both ways CORS silently breaks: a middleware registered via router.Use (mux
// bypasses it for the unmatched OPTIONS, so no headers come back) and an
// Access-Control-Allow-Methods list that has drifted from the routes. Neither
// shows up in a unit test, in curl, or in the same-origin frontend — the usual
// discovery path is an integrator's bug report.
//
// Driving Handler() rather than Router() is the whole point: the bare router has
// no CORS layer, so the same test against Router() would prove nothing.
func TestCORSPreflightForEveryWritingRoute(t *testing.T) {
	const origin = "https://app.example.test"
	srv := newCORSTestServer(t, origin)

	type op struct{ method, path string }
	var ops []op
	err := srv.Router().Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		// Matcher-only routes have no template, and path-var routes cannot be
		// preflighted literally; both are skipped (same idiom as contract_test).
		tmpl, _ := route.GetPathTemplate()
		if tmpl == "" || strings.Contains(tmpl, "{") {
			return nil
		}
		methods, _ := route.GetMethods()
		for _, m := range methods {
			// GET/HEAD are "simple requests" — no preflight, nothing to pin.
			if m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions {
				ops = append(ops, op{m, tmpl})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	if len(ops) == 0 {
		t.Skip("no writing routes registered yet — nothing to preflight")
	}

	for _, o := range ops {
		t.Run(o.method+" "+o.path, func(t *testing.T) {
			rr := doPreflight(srv, o.path, origin, o.method)

			if rr.Code != http.StatusNoContent {
				t.Errorf("preflight status = %d, want 204", rr.Code)
			}
			if got := rr.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("Allow-Origin = %q, want %q", got, origin)
			}
			if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, o.method) {
				t.Errorf("Allow-Methods = %q, must contain %s — the browser blocks the real request otherwise",
					got, o.method)
			}
		})
	}
}

// An unusable pattern must be reported at startup and must not silently enable
// a half-configured CORS layer. Parsing once here (rather than per request) is
// also what keeps a typo from looking like "configured but never matching".
func TestCORS_UnusableOriginPatternIsReportedAndSkipped(t *testing.T) {
	var logged strings.Builder
	logger := slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv := NewServer(":0", stubFeatures{}, stubBatchService{}, stubProviders{}, stubHealth{}, fixedClock{}, logger,
		Options{CORSAllowedOrigins: []string{"example.test", "https://good.test"}}) // first lacks a scheme

	if !strings.Contains(logged.String(), "example.test") {
		t.Errorf("startup log should name the unusable pattern, got: %s", logged.String())
	}
	// The usable one still works …
	if got := doCORS(srv, "https://good.test").Header().Get("Access-Control-Allow-Origin"); got != "https://good.test" {
		t.Errorf("Allow-Origin = %q, want the valid pattern to still apply", got)
	}
	// … and the broken one grants nothing.
	if got := doCORS(srv, "https://example.test").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty — the pattern was unusable", got)
	}
}

// With every configured pattern unusable, CORS must stay off entirely rather
// than run an allow-list that can never match.
func TestCORS_AllPatternsUnusableDisablesCORS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := NewServer(":0", stubFeatures{}, stubBatchService{}, stubProviders{}, stubHealth{}, fixedClock{}, logger,
		Options{CORSAllowedOrigins: []string{"nonsense", "also://bad/path"}})

	rr := doPreflight(srv, "/api/v1/query/batch", "https://a.test", http.MethodPost)
	if rr.Code == http.StatusNoContent {
		t.Error("preflight was short-circuited although no usable origin is configured")
	}
}
