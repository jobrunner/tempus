// Resolving "who is the client" is its own concern from bucket accounting, and
// the security-sensitive half: get it wrong and either every client shares one
// bucket (proxy not trusted) or anyone can mint a fresh one per request (header
// trusted blindly).
package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// parseCIDRs parses the trusted-proxy list, returning the parsed networks and
// any entries that failed (so the caller can warn — a silently dropped CIDR
// would quietly disable X-Forwarded-For handling and rate-limit the proxy
// itself, i.e. everyone behind it as one client).
func parseCIDRs(cidrs []string) (nets []*net.IPNet, invalid []string) {
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, n)
		} else {
			invalid = append(invalid, c)
		}
	}
	return nets, invalid
}

func ipInAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP resolves the request's client IP for rate limiting. By default it is
// the direct peer (RemoteAddr). X-Forwarded-For is consulted ONLY when the
// direct peer is itself a trusted proxy; even then the client is the RIGHT-most
// entry that is not a trusted proxy — never the left-most, which a client can
// spoof (proxies append to XFF rather than overwrite it, so anything to the left
// of the first trusted hop is attacker-controlled).
func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr // RemoteAddr without a port (unusual) — use as-is
	}
	peer := net.ParseIP(host)
	if peer == nil || len(trusted) == 0 || !ipInAny(peer, trusted) {
		return host
	}
	// Direct peer is a trusted proxy: walk XFF right-to-left, skipping further
	// trusted hops; the first non-trusted address is the real client.
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		parsed := net.ParseIP(ip)
		if parsed == nil || ipInAny(parsed, trusted) {
			continue
		}
		return ip
	}
	// All XFF entries are trusted proxies (or none present) — fall back to peer.
	return host
}
