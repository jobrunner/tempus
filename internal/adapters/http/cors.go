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

		// Preflights never reach a handler. Answering 204 regardless of whether
		// the origin is allowed is deliberate: without the Allow-Origin header
		// above, the browser rejects the request anyway, and a uniform answer
		// avoids leaking which origins are configured.
		if r.Method == http.MethodOptions {
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
func matchOrigin(origin, pattern string) bool {
	if origin == pattern {
		return true
	}

	// Wildcard patterns may be written with or without a scheme
	// ("https://*.example.com" or "*.example.com"); compare hosts either way.
	patternHost := extractHost(pattern)
	if !strings.HasPrefix(patternHost, "*.") {
		return false
	}
	suffix := patternHost[1:] // "*.example.com" -> ".example.com"
	originHost := extractHost(origin)
	// len > len(suffix) keeps "example.com" itself out, and requiring the dot
	// keeps "evil-example.com" out.
	return strings.HasSuffix(originHost, suffix) && len(originHost) > len(suffix)
}

// extractHost strips scheme, port and path from an origin or pattern.
func extractHost(origin string) string {
	host := origin
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}
