package domain_test

import (
	"strings"
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
)

func TestParseOrigin_Valid(t *testing.T) {
	cases := []struct {
		in                 string
		scheme, host, port string
	}{
		{"https://example.com", "https", "example.com", ""},
		{"http://localhost:8080", "http", "localhost", "8080"},
		{"https://sub.example.com:8443", "https", "sub.example.com", "8443"},
		{"http://[::1]:8080", "http", "[::1]", "8080"},
		{"http://[::1]", "http", "[::1]", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := domain.ParseOrigin(tc.in)
			if err != nil {
				t.Fatalf("ParseOrigin: %v", err)
			}
			if got.Scheme != tc.scheme || got.Host != tc.host || got.Port != tc.port {
				t.Errorf("got %+v, want scheme=%q host=%q port=%q", got, tc.scheme, tc.host, tc.port)
			}
		})
	}
}

// Each rejection exists because accepting it would either widen the rule or
// leave one that can never match — both worse than an error at startup.
func TestParseOrigin_Rejects(t *testing.T) {
	cases := []struct {
		name, in, wantErrContains string
	}{
		{"opaque null origin", "null", "sandboxed"},
		{"no scheme", "example.com", "needs a scheme"},
		{"empty scheme", "://example.com", "needs a scheme"},
		{"path would silently widen the rule", "https://example.com/private", "must not contain a path"},
		{"no host", "https://", "needs a host"},
		{"port out of range", "https://example.com:99999", "invalid port"},
		{"non-numeric port", "https://example.com:http", "invalid port"},
		{"empty port", "https://example.com:", "invalid port"},
		{"junk after IPv6 literal", "http://[::1]x", "IPv6 literal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ParseOrigin(tc.in)
			if err == nil {
				t.Fatalf("ParseOrigin(%q) = nil error, want a rejection", tc.in)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error %q should mention %q so the operator can fix the entry", err, tc.wantErrContains)
			}
		})
	}
}

func TestParseOriginPattern_Rejects(t *testing.T) {
	for _, in := range []string{
		"https://sub*.example.com", // "*" is not a whole label
		"https://*.",               // suffix "." would match any host ending in a dot
		"https://*",                // no base host at all
		"*.example.com",            // no scheme: would match nothing a browser sends
	} {
		t.Run(in, func(t *testing.T) {
			if _, err := domain.ParseOriginPattern(in); err == nil {
				t.Errorf("ParseOriginPattern(%q) = nil error, want a rejection", in)
			}
		})
	}
}

func TestOriginPattern_Matches(t *testing.T) {
	cases := []struct {
		pattern, origin string
		want            bool
	}{
		// Exact patterns.
		{"https://example.com", "https://example.com", true},
		{"https://example.com", "http://example.com", false},       // scheme differs
		{"https://example.com", "https://example.com:8443", false}, // port differs
		{"https://example.com", "https://sub.example.com", false},

		// Wildcards cover subdomains, not the bare domain.
		{"https://*.example.com", "https://sub.example.com", true},
		{"https://*.example.com", "https://deep.sub.example.com", true},
		{"https://*.example.com", "https://example.com", false},
		{"https://*.example.com", "https://notexample.com", false},
		{"https://*.example.com", "http://sub.example.com", false},       // scheme
		{"https://*.example.com", "https://sub.example.com:8443", false}, // port

		// Ports are part of the identity in both directions.
		{"http://localhost:8080", "http://localhost:8080", true},
		{"http://localhost:8080", "http://localhost:3000", false},
		{"http://localhost:8080", "http://localhost", false},

		// A malformed origin header never matches.
		{"https://example.com", "not-an-origin", false},
		{"https://example.com", "null", false},
	}
	for _, tc := range cases {
		t.Run(tc.pattern+" vs "+tc.origin, func(t *testing.T) {
			p, err := domain.ParseOriginPattern(tc.pattern)
			if err != nil {
				t.Fatalf("ParseOriginPattern(%q): %v", tc.pattern, err)
			}
			if got := p.MatchesString(tc.origin); got != tc.want {
				t.Errorf("MatchesString(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}
