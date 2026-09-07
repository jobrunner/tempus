package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFrontend_ContainsBatchTab pins the batch UI's load-bearing element ids;
// renaming them breaks the tab wiring silently, so the smoke test names them.
func TestFrontend_ContainsBatchTab(t *testing.T) {
	body := fetchIndexBody(t)
	for _, want := range []string{
		`id="tabSingle"`, `id="tabBatch"`, `id="batchPanel"`,
		`id="batchInput"`, `id="batchProgress"`, `id="batchRunBtn"`,
		`id="batchAbortBtn"`, `id="batchExportBtn"`, `id="batchTable"`,
		`id="batchProvidersSummary"`, `id="batchProvidersPanel"`,
		`id="batchCsvUploadBtn"`, `id="batchFile"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("frontend page is missing %s", want)
		}
	}
}

// TestFrontend_BatchIIFEDefinesOwnEscHelper guards against the class of bug
// found in review round 1: the batch tab's code lives in its own IIFE
// `(function initBatch() { ... })()`, a sibling of the pre-existing IIFE that
// declares `function esc(str)`. Function declarations are scoped to their
// enclosing function and are not exported to the global object, so a batch
// helper calling the *other* IIFE's `esc` would throw ReferenceError at
// runtime on the first streamed row — a failure no HTML/JS parser or the id
// smoke test above catches. This test brace-matches `initBatch`'s own body
// and asserts it declares its own `esc`, so deleting that local helper (or
// reintroducing a call to an out-of-scope one) fails loudly here instead of
// silently in the browser.
func TestFrontend_BatchIIFEDefinesOwnEscHelper(t *testing.T) {
	body := fetchIndexBody(t)

	marker := "function initBatch() {"
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("could not find %q in served page", marker)
	}
	openBrace := start + strings.Index(body[start:], "{")

	// Brace-count from initBatch's opening '{' to its matching closing '}'.
	depth := 0
	end := -1
	for i := openBrace; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		t.Fatalf("could not find matching closing brace for %q", marker)
	}

	batchBody := body[openBrace:end]
	if !strings.Contains(batchBody, "function esc(") {
		t.Error("initBatch's own body does not declare `function esc(...)` — " +
			"the batch code either lost its local esc helper or now relies on " +
			"an out-of-scope one from a sibling IIFE, which throws at runtime " +
			"(ReferenceError: esc is not defined) on the first streamed row")
	}
}

// fetchIndexBody GETs "/" through the router and returns the response body,
// shared by the frontend smoke tests above.
func fetchIndexBody(t *testing.T) string {
	t.Helper()
	srv := newContractTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	return rr.Body.String()
}
