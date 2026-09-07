package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// echoBatch is a minimal input.BatchService: valid points become ok items
// echoing the request, invalid ones become error items.
type echoBatch struct{}

func (echoBatch) QueryBatch(_ context.Context, points []input.BatchPoint, emit func(input.BatchItem) error) error {
	for _, p := range points {
		if p.Req == nil {
			if err := emit(input.BatchItem{ID: p.ID, Error: &input.BatchItemError{Message: p.ParseError}}); err != nil {
				return err
			}
			continue
		}
		res := domain.QueryResult{
			Query:     domain.QueryEcho{Coordinate: p.Req.Coordinate, Datetime: p.Req.Instant.UTC().Format("2006-01-02T15:04:05Z07:00")},
			Features:  []domain.Feature{},
			Providers: []domain.ProviderStatus{},
		}
		if err := emit(input.BatchItem{ID: p.ID, QueryResult: &res}); err != nil {
			return err
		}
	}
	return nil
}

func newBatchTestServer(t *testing.T, limits BatchLimits) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewServer(":0", stubFeatures{}, echoBatch{}, stubProviders{}, stubHealth{}, fixedClock{}, logger, Options{Batch: limits})
}

func postBatch(t *testing.T, srv *Server, body string, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	return rr
}

func TestHandleQueryBatch_SyncEnvelope(t *testing.T) {
	// 2 points (one with an id, one without) → 200, results in request order,
	// a missing id falls back to its index as a string, total == 2, and
	// processing_time_ms is populated (asserted below).
	body := `{"points":[
		{"id":"x","lat":49.79,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"},
		{"lat":47.42,"lon":10.98,"datetime":"2025-06-03T14:00:00Z"}]}`
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), body, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var env struct {
		Results          []json.RawMessage `json:"results"`
		Total            int               `json:"total"`
		ProcessingTimeMS int64             `json:"processing_time_ms"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Total != 2 || len(env.Results) != 2 {
		t.Fatalf("total/results = %d/%d, want 2/2", env.Total, len(env.Results))
	}
	// The test clock is fixed, so start and end read the same instant and the
	// envelope's processing_time_ms is deterministically zero rather than absent.
	if env.ProcessingTimeMS != 0 {
		t.Errorf("processing_time_ms = %d, want 0 (fixed clock)", env.ProcessingTimeMS)
	}
	var first struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(env.Results[0], &first)
	var second struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(env.Results[1], &second)
	if first.ID != "x" || second.ID != "1" {
		t.Errorf("ids = %q,%q; want x,1 (index fallback)", first.ID, second.ID)
	}
}

// batchBody builds a JSON body with n valid points.
func batchBody(n int) string {
	var b strings.Builder
	b.WriteString(`{"points":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"p%d","lat":49.79,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"}`, i)
	}
	b.WriteString(`]}`)
	return b.String()
}

func TestHandleQueryBatch_InvalidPointBecomesErrorItem(t *testing.T) {
	body := `{"points":[{"id":"bad","lat":999,"lon":9.95,"datetime":"2025-06-03T14:00:00Z"}]}`
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), body, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (per-item errors never abort)", rr.Code)
	}
	var env struct {
		Results []struct {
			ID    string `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Query json.RawMessage `json:"query"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	it := env.Results[0]
	if it.ID != "bad" || it.Error == nil || !strings.Contains(it.Error.Message, "lat") {
		t.Errorf("item = %+v, want id=bad with lat parse error", it)
	}
	if len(it.Query) != 0 {
		t.Error("error item must not carry a query echo")
	}
}

func TestHandleQueryBatch_EmptyPointsIs400(t *testing.T) {
	rr := postBatch(t, newBatchTestServer(t, BatchLimits{}), `{"points":[]}`, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleQueryBatch_TooManyPointsIs400(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 5, MaxSyncPoints: 5})
	rr := postBatch(t, srv, batchBody(6), "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleQueryBatch_SyncCapIs413WithNDJSONHint(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 10, MaxSyncPoints: 2})
	rr := postBatch(t, srv, batchBody(3), "")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "application/x-ndjson") {
		t.Errorf("413 message must hint at NDJSON streaming, got %s", rr.Body.String())
	}
}

func TestHandleQueryBatch_NDJSONStreamsAllPoints(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 10, MaxSyncPoints: 2})
	rr := postBatch(t, srv, batchBody(3), "application/x-ndjson")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (stream mode bypasses the sync cap)", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", ct)
	}
	var ids []string
	sc := bufio.NewScanner(bytes.NewReader(rr.Body.Bytes()))
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var it struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(sc.Bytes(), &it); err != nil {
			t.Fatalf("line not parseable: %v", err)
		}
		ids = append(ids, it.ID)
	}
	if len(ids) != 3 || ids[0] != "p0" || ids[2] != "p2" {
		t.Fatalf("ids = %v, want [p0 p1 p2] in input order", ids)
	}
}

func TestHandleQueryBatch_BodyTooLargeIs413(t *testing.T) {
	srv := newBatchTestServer(t, BatchLimits{MaxPoints: 5, MaxSyncPoints: 5})
	// Cap = 5*512 B + 64 KiB; ~80 KiB whitespace padding inside the JSON
	// blows it while staying syntactically pending.
	body := `{"points":[` + strings.Repeat(" ", 80*1024) + `]}`
	rr := postBatch(t, srv, body, "")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}
