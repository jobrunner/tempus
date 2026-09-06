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
	srv := newContractTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	body := rr.Body.String()
	for _, want := range []string{
		`id="tabSingle"`, `id="tabBatch"`, `id="batchPanel"`,
		`id="batchInput"`, `id="batchProgress"`, `id="batchRunBtn"`,
		`id="batchAbortBtn"`, `id="batchExportBtn"`, `id="batchTable"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("frontend page is missing %s", want)
		}
	}
}
