package httpapi

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"go.yaml.in/yaml/v3"
)

//go:embed openapi.yaml
var openAPIYAML embed.FS

var (
	openAPIJSON     []byte
	openAPIJSONOnce sync.Once
	openAPIJSONErr  error
)

// getOpenAPIJSON returns the spec as JSON (converted + cached on first use).
// The contract test consumes this — the exact bytes served at /openapi.json.
func getOpenAPIJSON() ([]byte, error) {
	openAPIJSONOnce.Do(func() {
		openAPIJSON, openAPIJSONErr = convertOpenAPIToJSON()
	})
	return openAPIJSON, openAPIJSONErr
}

func convertOpenAPIToJSON() ([]byte, error) {
	data, err := openAPIYAML.ReadFile("openapi.yaml")
	if err != nil {
		return nil, err
	}
	var spec any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return json.MarshalIndent(convertYAMLToJSON(spec), "", "  ")
}

// convertYAMLToJSON turns yaml.v3's map/interface trees into JSON-serializable
// maps (string keys). yaml.v3 already yields map[string]interface{}, but the
// recursion keeps this robust across nested docs.
func convertYAMLToJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = convertYAMLToJSON(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			if ks, ok := k.(string); ok {
				m[ks] = convertYAMLToJSON(val)
			}
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = convertYAMLToJSON(val)
		}
		return s
	default:
		return v
	}
}

// handleOpenAPI serves the embedded spec as JSON.
func (s *Server) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	body, err := getOpenAPIJSON()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "failed to load OpenAPI specification")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// Swagger UI is loaded from unpkg, pinned to one exact swagger-ui-dist release
// and guarded by Subresource Integrity, so neither a new upstream release nor
// a tampered CDN file changes what /docs runs. A bump changes the version AND
// both hashes (they are per-release); recompute each with
//
//	curl -sL https://unpkg.com/swagger-ui-dist@<version>/<file> |
//	  openssl dgst -sha384 -binary | openssl base64 -A
//
// Pick a release that has been out for about two weeks: SRI freezes whatever
// the pinned version contains, so a just-published (possibly bad) release
// should get time to be yanked first. Check that unpkg and jsDelivr serve
// identical hashes.
const (
	swaggerUIVersion      = "5.34.1"
	swaggerUICSSIntegrity = "sha384-4c3vsE5f+trhFyHrFrLM1fFGUn531NIaC7dcqEWIJdoRQJ5CkJbSuSCwXLpaS1KF"
	swaggerUIJSIntegrity  = "sha384-XFYN47VzPiDY6qPtxYKg0nqUeW/upo20xf1j2SUE7gT/i35FJB4cuS/71Iegzk8Z"
)

// swaggerUIPage is a minimal Swagger UI page pointing at /openapi.json.
// (Swap for your preferred renderer; kept tiny here on purpose.) It is
// formatted rather than concatenated: gremlins mutates every `+` in a const
// expression, and coverage cannot reach a const, so each one would count as an
// uncovered mutant.
var swaggerUIPage = fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>tempus API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@%[1]s/swagger-ui.css" integrity="%[2]s" crossorigin="anonymous"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@%[1]s/swagger-ui-bundle.js" integrity="%[3]s" crossorigin="anonymous"></script>
<script>window.onload=()=>SwaggerUIBundle({url:"/openapi.json",dom_id:"#swagger-ui"})</script>
</body></html>`, swaggerUIVersion, swaggerUICSSIntegrity, swaggerUIJSIntegrity)

// handleSwaggerUI serves the Swagger UI page.
func (s *Server) handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}
