package httpapi

import (
	"net/http"

	"github.com/gorilla/mux"
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
				// Answer with the method the ROUTER accepts for this path rather
				// than a hand-written list. A literal "GET, POST, OPTIONS" is
				// correct exactly until someone adds a DELETE route: nothing
				// fails at build time, and the endpoint is simply unusable from
				// a browser. Deriving it from the route table makes that drift
				// impossible.
				if m := r.Header.Get("Access-Control-Request-Method"); m != "" && s.routeAllowsMethod(r, m) {
					w.Header().Set("Access-Control-Allow-Methods", m+", OPTIONS")
				}
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

// routeAllowsMethod asks the router whether method+path would match a route.
// mux reports a path that exists under a different method via
// RouteMatch.MatchErr == ErrMethodMismatch, so a method the service does not
// serve is never advertised as allowed.
func (s *Server) routeAllowsMethod(r *http.Request, method string) bool {
	probe := r.Clone(r.Context())
	probe.Method = method
	var match mux.RouteMatch
	return s.router.Match(probe, &match) && match.MatchErr == nil
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
