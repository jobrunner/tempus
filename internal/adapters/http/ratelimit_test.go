package httpapi

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newRateLimitedServer(t *testing.T, limits RateLimit) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, stubBatchService{}, stubProviders{}, stubHealth{}, fixedClock{}, logger,
		Options{RateLimit: limits})
}

// getFrom issues a GET as if it came from remoteAddr. Forwarded-header handling
// is covered directly against clientIP in TestClientIP_TrustBoundary.
func getFrom(srv *Server, path, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

// Burst 2 means the third request within the same instant is refused.
func TestRateLimit_RefusesBeyondBurst(t *testing.T) {
	srv := newRateLimitedServer(t, RateLimit{Enabled: true, Rate: 0.0001, Burst: 2})

	for i := 1; i <= 2; i++ {
		if rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1234"); rr.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (still within burst)", i, rr.Code)
		}
	}
	rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1234")
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("third request: status = %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got == "" {
		t.Error("a 429 must carry Retry-After so a client knows when to come back")
	}
}

// Buckets are per client IP: one noisy client must not spend another's budget.
func TestRateLimit_IsPerClientIP(t *testing.T) {
	srv := newRateLimitedServer(t, RateLimit{Enabled: true, Rate: 0.0001, Burst: 1})

	if rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1"); rr.Code != http.StatusOK {
		t.Fatalf("first client: status = %d, want 200", rr.Code)
	}
	if rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("first client again: status = %d, want 429", rr.Code)
	}
	if rr := getFrom(srv, "/api/v1/providers", "10.0.0.2:1"); rr.Code != http.StatusOK {
		t.Errorf("second client: status = %d, want 200 — buckets must be per IP", rr.Code)
	}
}

// Health and readiness probes must never be rate-limited: a load spike would
// otherwise make the orchestrator kill a container that is merely busy.
func TestRateLimit_NeverAppliesToProbes(t *testing.T) {
	srv := newRateLimitedServer(t, RateLimit{Enabled: true, Rate: 0.0001, Burst: 1})

	// Spend the bucket on the API surface first.
	getFrom(srv, "/api/v1/providers", "10.0.0.1:1")
	if rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("precondition: API should be limited now, got %d", rr.Code)
	}

	for _, p := range []string{"/health", "/health/live", "/health/ready"} {
		if rr := getFrom(srv, p, "10.0.0.1:1"); rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 — probes are never rate-limited", p, rr.Code)
		}
	}
}

func TestRateLimit_DisabledLetsEverythingThrough(t *testing.T) {
	srv := newRateLimitedServer(t, RateLimit{}) // disabled

	for i := 0; i < 5; i++ {
		if rr := getFrom(srv, "/api/v1/providers", "10.0.0.1:1"); rr.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 when the limiter is off", i, rr.Code)
		}
	}
}

// The security-critical part: X-Forwarded-For is only believed when the direct
// peer is a trusted proxy, and then the client is the RIGHT-most non-trusted
// entry. A client can append anything to the left of XFF, so trusting the
// left-most value would let anyone mint a fresh bucket per request.
func TestClientIP_TrustBoundary(t *testing.T) {
	trusted, _ := parseCIDRs([]string{"10.0.0.0/8"})

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"no proxies configured: always the peer", "203.0.113.7:443", "1.2.3.4", "203.0.113.7"},
		{"untrusted peer: XFF ignored", "198.51.100.9:80", "1.2.3.4", "198.51.100.9"},
		{"trusted peer: right-most non-trusted wins", "10.1.2.3:80", "9.9.9.9, 203.0.113.7", "203.0.113.7"},
		{"spoofed left entries are skipped", "10.1.2.3:80", "1.1.1.1, 2.2.2.2, 203.0.113.7", "203.0.113.7"},
		{"further trusted hops are skipped", "10.1.2.3:80", "203.0.113.7, 10.4.5.6", "203.0.113.7"},
		{"all hops trusted: fall back to peer", "10.1.2.3:80", "10.4.5.6, 10.7.8.9", "10.1.2.3"},
		{"trusted peer without XFF", "10.1.2.3:80", "", "10.1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			use := trusted
			if tc.name == "no proxies configured: always the peer" {
				use = nil
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := clientIP(req, use); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

// An unparseable CIDR must be reported rather than dropped: silently ignoring
// it would disable X-Forwarded-For handling and rate-limit the proxy itself.
func TestParseCIDRs_ReportsInvalidEntries(t *testing.T) {
	nets, invalid := parseCIDRs([]string{"10.0.0.0/8", " ", "not-a-cidr", "192.168.0.0/16"})

	if len(nets) != 2 {
		t.Errorf("parsed %d networks, want 2", len(nets))
	}
	if len(invalid) != 1 || invalid[0] != "not-a-cidr" {
		t.Errorf("invalid = %v, want [not-a-cidr]", invalid)
	}
	if !ipInAny(net.ParseIP("10.1.2.3"), nets) {
		t.Error("10.1.2.3 should be inside 10.0.0.0/8")
	}
	if ipInAny(net.ParseIP("203.0.113.7"), nets) {
		t.Error("203.0.113.7 must not match any configured network")
	}
}

// Idle buckets must not accumulate forever — the sweep is what keeps a
// long-running instance from growing a bucket per client IP seen since boot.
func TestRateLimiter_EvictsIdleBuckets(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	l := newIPRateLimiter(100, 10)
	l.now = func() time.Time { return now }

	l.allow("10.0.0.1")
	l.allow("10.0.0.2")
	if got := l.size(); got != 2 {
		t.Fatalf("buckets = %d, want 2", got)
	}

	// Past the TTL, a later access sweeps the idle ones.
	now = now.Add(2 * l.ttl)
	l.allow("10.0.0.3")
	if got := l.size(); got != 1 {
		t.Errorf("buckets after sweep = %d, want 1 (only the fresh one)", got)
	}
}
