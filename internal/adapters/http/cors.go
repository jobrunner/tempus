package httpapi

import (
	"net/http"
	"strings"
)

// corsMaxAgeSeconds is how long a browser may cache a preflight result.
const corsMaxAgeSeconds = "86400" // 24 hours

// wrapCORS returns h unchanged when no origins are configured, so a service
// without CORS keeps byte-identical responses and pays nothing per request.
func (s *Server) wrapCORS(h http.Handler) http.Handler {
	if len(s.corsAllowedOrigins) == 0 {
		return h
	}
	return s.corsHandler(h)
}

// corsHandler wraps the whole router — deliberately NOT registered via
// router.Use. gorilla/mux only runs Use-middleware for requests that match a
// route, and an OPTIONS preflight against a GET-or-POST-only route matches
// nothing: it goes straight to the MethodNotAllowedHandler, outside the
// middleware chain. Registered as Use-middleware, CORS would therefore answer
// every preflight with a bare 405 and no headers, which breaks
// POST /api/v1/query/batch from a browser (its application/json body always
// triggers a preflight). Wrapping the router sees every request instead.
func (s *Server) corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Vary is set for any cross-origin request, allowed or not:
			// the response body/headers depend on Origin, so a shared cache
			// must not hand one origin's response to another.
			w.Header().Add("Vary", "Origin")
			if s.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", corsMaxAgeSeconds)
			}
		}

		// Only a real preflight is short-circuited: OPTIONS carrying both Origin
		// and Access-Control-Request-Method. A bare OPTIONS (no Origin, or a
		// probe without the request-method header) keeps falling through to the
		// router exactly as it did before CORS existed — otherwise enabling CORS
		// would silently turn every OPTIONS into a 204.
		//
		// Answering 204 regardless of whether the origin is *allowed* is
		// deliberate: without the Allow-Origin header above the browser rejects
		// the response anyway, and a uniform answer avoids leaking which origins
		// are configured.
		if r.Method == http.MethodOptions && origin != "" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed reports whether origin matches any configured pattern.
func (s *Server) isOriginAllowed(origin string) bool {
	for _, pattern := range s.corsAllowedOrigins {
		if matchOrigin(origin, pattern) {
			return true
		}
	}
	return false
}

// matchOrigin matches an origin against one pattern: either exactly, or as a
// "*.example.com" wildcard covering subdomains (but not the bare domain).
//
// Only the host label is wildcarded. Scheme and port must still match exactly,
// so "https://*.example.com" does NOT admit "http://sub.example.com" (plaintext)
// or "https://sub.example.com:8443" (a different service on the same host) —
// an origin is the scheme/host/port triple, and widening it silently would hand
// responses to servers the operator never listed.
func matchOrigin(origin, pattern string) bool {
	if origin == pattern {
		return true
	}

	oScheme, oHost, oPort := splitOrigin(origin)
	pScheme, pHost, pPort := splitOrigin(pattern)
	if oScheme != pScheme || oPort != pPort {
		return false
	}
	if !strings.HasPrefix(pHost, "*.") {
		return false
	}
	suffix := pHost[1:] // "*.example.com" -> ".example.com"
	// len > len(suffix) keeps "example.com" itself out, and requiring the dot
	// keeps "evil-example.com" out.
	return strings.HasSuffix(oHost, suffix) && len(oHost) > len(suffix)
}

// splitOrigin breaks an origin (or a wildcard pattern) into scheme, host and
// port. A pattern written without a scheme yields an empty scheme, which then
// only matches an equally scheme-less origin — browsers always send one, so
// such a pattern matches nothing and is better rejected than quietly widened.
func splitOrigin(origin string) (scheme, host, port string) {
	rest := origin
	if idx := strings.Index(rest, "://"); idx != -1 {
		scheme, rest = rest[:idx], rest[idx+3:]
	}
	if idx := strings.Index(rest, "/"); idx != -1 {
		rest = rest[:idx]
	}
	if idx := strings.LastIndex(rest, ":"); idx != -1 {
		rest, port = rest[:idx], rest[idx+1:]
	}
	return scheme, rest, port
}
