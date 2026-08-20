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
		// GetPathTemplate errors for matcher-only routes (no path template); skip those.
		tmpl, _ := route.GetPathTemplate()
		if tmpl == "" {
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

// TestFeaturePropertiesSchemasAreWired guards the defect class that let six
// *FeatureProperties schemas sit in the spec unreferenced: a schema that
// documents a feature kind must be reachable from Feature.properties (via the
// oneOf) and resolvable by the discriminator, and its kind enum must agree
// with the mapping key that points at it. Without this, adding a provider kind
// or a property schema silently produces prose no tooling can validate.
func TestFeaturePropertiesSchemasAreWired(t *testing.T) {
	specJSON, err := getOpenAPIJSON()
	if err != nil {
		t.Fatalf("getOpenAPIJSON: %v", err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties struct {
					Properties struct {
						OneOf []struct {
							Ref string `json:"$ref"`
						} `json:"oneOf"`
						Discriminator struct {
							PropertyName string            `json:"propertyName"`
							Mapping      map[string]string `json:"mapping"`
						} `json:"discriminator"`
					} `json:"properties"`
				} `json:"properties"`
				AllOf []struct {
					Properties struct {
						Kind struct {
							Enum []string `json:"enum"`
						} `json:"kind"`
					} `json:"properties"`
				} `json:"allOf"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	feature, ok := spec.Components.Schemas["Feature"]
	if !ok {
		t.Fatal("spec has no Feature schema")
	}
	props := feature.Properties.Properties
	if props.Discriminator.PropertyName != "kind" {
		t.Errorf("Feature.properties discriminator.propertyName = %q, want %q",
			props.Discriminator.PropertyName, "kind")
	}

	inOneOf := map[string]bool{}
	for _, r := range props.OneOf {
		inOneOf[strings.TrimPrefix(r.Ref, "#/components/schemas/")] = true
	}
	mappedBy := map[string]string{} // schema name → discriminator key
	for key, ref := range props.Discriminator.Mapping {
		mappedBy[strings.TrimPrefix(ref, "#/components/schemas/")] = key
	}

	var documented int
	for name, schema := range spec.Components.Schemas {
		if !strings.HasSuffix(name, "FeatureProperties") {
			continue
		}
		documented++
		if !inOneOf[name] {
			t.Errorf("schema %q is not reachable from Feature.properties.oneOf", name)
		}
		key, mapped := mappedBy[name]
		if !mapped {
			t.Errorf("schema %q is not in Feature.properties.discriminator.mapping", name)
			continue
		}
		// The kind enum lives in the second allOf member (the first is the base).
		var kinds []string
		for _, member := range schema.AllOf {
			kinds = append(kinds, member.Properties.Kind.Enum...)
		}
		if len(kinds) != 1 {
			t.Errorf("schema %q: want exactly one kind enum value, got %v", name, kinds)
			continue
		}
		if kinds[0] != key {
			t.Errorf("schema %q: kind enum is %q but the discriminator maps it under %q",
				name, kinds[0], key)
		}
	}
	if documented == 0 {
		t.Fatal("found no *FeatureProperties schemas — the assertions above proved nothing")
	}
	if len(inOneOf) != documented {
		t.Errorf("Feature.properties.oneOf has %d members but the spec defines %d *FeatureProperties schemas",
			len(inOneOf), documented)
	}
}

// newContractTestServer builds a Server wired with fakes — enough to register
// every route. The stubs from server_test.go in the same package are reused.
func newContractTestServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{})
}
