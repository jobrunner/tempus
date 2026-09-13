// Origin matching is its own concern from the middleware that applies it, and
// the part where a subtle mistake widens the policy silently.
package httpapi

import "strings"

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
