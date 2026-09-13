package httpapi

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// bucketTTL is how long an idle client bucket is kept before the inline sweep
// evicts it.
const bucketTTL = 10 * time.Minute

// ipRateLimiter is a per-client-IP token-bucket limiter. It is opt-in
// (server.rate_limit.enabled) — meant for a tempus exposed directly on a public
// IP, without a rate-limiting gateway in front.
//
// Memory is bounded by an inline sweep: idle buckets are evicted on access once
// per TTL, so there is no background goroutine to own or shut down.
type ipRateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*ipBucket
	rate      rate.Limit
	burst     int
	ttl       time.Duration
	lastSweep time.Time
	now       func() time.Time // injectable for tests
}

type ipBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(r float64, burst int) *ipRateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &ipRateLimiter{
		buckets: make(map[string]*ipBucket),
		rate:    rate.Limit(r),
		burst:   burst,
		ttl:     bucketTTL,
		now:     time.Now,
	}
}

// allow reports whether a request from ip may proceed now.
func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) > l.ttl {
		for k, b := range l.buckets {
			if now.Sub(b.lastSeen) > l.ttl {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.buckets[ip]
	if !ok {
		b = &ipBucket{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[ip] = b
	}
	b.lastSeen = now
	return b.limiter.Allow()
}

// size reports the number of live buckets (used by the eviction test).
func (l *ipRateLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// initRateLimit prepares the limiter when enabled. Invalid trusted-proxy CIDRs
// are logged rather than dropped in silence: with the list empty the middleware
// falls back to the direct peer, which behind a proxy is the proxy itself —
// every client would then share one bucket.
func (s *Server) initRateLimit(cfg RateLimit) {
	if !cfg.Enabled {
		return
	}
	nets, invalid := parseCIDRs(cfg.TrustedProxies)
	if len(invalid) > 0 {
		s.logger.Warn("ignoring unparseable trusted-proxy CIDRs; X-Forwarded-For will not be trusted for them",
			"invalid", invalid)
	}
	s.trustedProxies = nets
	s.rateLimiter = newIPRateLimiter(cfg.Rate, cfg.Burst)
}

// rateLimitMiddleware enforces the per-IP limit. It is mounted on the /api/v1
// subrouter only: health and readiness probes must answer even under load, or
// an orchestrator kills a container that is merely busy.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.rateLimiter.allow(clientIP(r, s.trustedProxies)) {
			w.Header().Set("Retry-After", "1")
			s.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
