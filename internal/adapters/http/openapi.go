package httpapi

import (
	"embed"
	"encoding/json"
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

// handleSwaggerUI serves a minimal Swagger UI page pointing at /openapi.json.
// (Swap for your preferred renderer; kept tiny here on purpose.)
func (s *Server) handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	const page = `<!doctype html><html><head><meta charset="utf-8"><title>tempus API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>window.onload=()=>SwaggerUIBundle({url:"/openapi.json",dom_id:"#swagger-ui"})</script>
</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}
