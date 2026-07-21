package httpapi

import (
	_ "embed"
	"html"
	"net/http"
	"strings"
)

//go:embed index.html
var indexHTML string

// renderFrontend substitutes the build version into the footer placeholder once,
// at server construction. The version is HTML-escaped (it comes from a trusted
// -ldflags value, but escaping keeps the template injection-safe regardless).
func renderFrontend(version string) []byte {
	return []byte(strings.Replace(indexHTML, "__TEMPUS_VERSION__", html.EscapeString(version), 1))
}

// handleIndex serves the pre-rendered weather query frontend. The page is built
// once in NewServer (the version is constant for the server's lifetime), so each
// request only writes the cached bytes.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.frontendPage)
}
