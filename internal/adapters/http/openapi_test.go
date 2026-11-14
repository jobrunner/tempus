package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIJSON_ServesSpec(t *testing.T) {
	rr := httptest.NewRecorder()
	testServer().Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if _, ok := spec["openapi"]; !ok {
		t.Error(`spec has no "openapi" key`)
	}
}

// TestSwaggerUI_PinnedWithSRI guards the /docs page's supply chain: both CDN
// assets load from an exact swagger-ui-dist version with a Subresource
// Integrity hash, so neither a new upstream release nor a tampered CDN file
// can change the code the page runs.
func TestSwaggerUI_PinnedWithSRI(t *testing.T) {
	rr := httptest.NewRecorder()
	testServer().Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	base := "https://unpkg.com/swagger-ui-dist@" + swaggerUIVersion + "/"
	for _, want := range []string{
		`href="` + base + `swagger-ui.css" integrity="` + swaggerUICSSIntegrity + `" crossorigin="anonymous"`,
		`src="` + base + `swagger-ui-bundle.js" integrity="` + swaggerUIJSIntegrity + `" crossorigin="anonymous"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/docs page is missing %s", want)
		}
	}
	for _, sri := range []string{swaggerUICSSIntegrity, swaggerUIJSIntegrity} {
		if !strings.HasPrefix(sri, "sha384-") {
			t.Errorf("integrity %q is not a sha384 hash", sri)
		}
	}
	if strings.Contains(body, "swagger-ui-dist/") {
		t.Error("/docs page still loads an unversioned swagger-ui-dist URL")
	}
}
