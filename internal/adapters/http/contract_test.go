package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestRoutesMatchOpenAPISpec(t *testing.T) {
	srv := newContractTestServer(t)

	// 1. Registered /api/v1 operations (path relative to the /api/v1 prefix).
	routes := map[string]bool{}
	err := srv.Router().Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		// GetPathTemplate errors for matcher-only routes (no path); skip those.
		tmpl, tErr := route.GetPathTemplate()
		if tErr != nil {
			return nil
		}
		rel, ok := strings.CutPrefix(tmpl, "/api/v1")
		if !ok || rel == "" {
			return nil
		}
		methods, _ := route.GetMethods()
		for _, m := range methods {
			routes[strings.ToUpper(m)+" "+rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}

	// 2. Documented operations from the embedded spec (the exact bytes served at
	// /openapi.json), minus the root health endpoints and any operator-only paths
	// you intentionally leave undocumented.
	specJSON, err := getOpenAPIJSON()
	if err != nil {
		t.Fatalf("getOpenAPIJSON: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}
	httpMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}
	documented := map[string]bool{}
	for p, ops := range spec.Paths {
		if strings.HasPrefix(p, "/health") {
			continue
		}
		for op := range ops {
			if m := strings.ToUpper(op); httpMethods[m] {
				documented[m+" "+p] = true
			}
		}
	}

	// 3. Both directions.
	for r := range routes {
		if !documented[r] {
			t.Errorf("route %q (under /api/v1) is registered but NOT documented in openapi.yaml", r)
		}
	}
	for op := range documented {
		if !routes[op] {
			t.Errorf("openapi.yaml documents %q but no matching /api/v1 route is registered", op)
		}
	}
}

// newContractTestServer builds a Server wired with fakes — enough to register
// every route. The stubs from server_test.go in the same package are reused.
func newContractTestServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{})
}
