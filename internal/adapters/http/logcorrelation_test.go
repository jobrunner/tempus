package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	// Test-only: the production logger decoration, so these tests prove the
	// whole chain (otelmux span → request ctx → log call → trace_id in the
	// line), not a stand-in. depguard exempts _test.go from layer rules.
	"github.com/jobrunner/tempus/internal/adapters/telemetry"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/input"
)

// logBuffer is a goroutine-safe sink: handlers log from the request goroutine
// while the test reads afterwards.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// line returns the raw JSON line whose msg equals msg, failing if there is not
// exactly one.
func (b *logBuffer) line(t *testing.T, msg string) string {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var found []string
	for _, l := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		var m struct {
			Msg string `json:"msg"`
		}
		if json.Unmarshal([]byte(l), &m) == nil && m.Msg == msg {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %q log line, got %d; log:\n%s", msg, len(found), b.buf.String())
	}
	return found[0]
}

func field(t *testing.T, line, key string) any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	return m[key]
}

// tracedServer builds the server the way the composition root does: a sampling
// TracerProvider and the production slog decoration over a JSON handler.
func tracedServer(t *testing.T, features input.FeatureService, batch input.BatchService) (*Server, *logBuffer) {
	t.Helper()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	logs := &logBuffer{}
	logger := slog.New(telemetry.NewSpanContextHandler(
		slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	srv := NewServer(":0", features, batch, stubProviders{}, stubHealth{}, fixedClock{}, logger,
		Options{TracerProvider: tp})
	return srv, logs
}

type panickingFeatures struct{}

func (panickingFeatures) Query(context.Context, domain.QueryRequest) (domain.QueryResult, error) {
	panic("provider exploded")
}

// The one log line an operator needs most — a recovered panic — must carry the
// trace_id the client saw in X-Trace-Id, or the 500 cannot be tied to its trace.
func TestLogCorrelation_PanicRecoveredCarriesTraceID(t *testing.T) {
	srv, logs := tracedServer(t, panickingFeatures{}, stubBatchService{})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet,
		"/api/v1/query?lat=49.79&lon=9.93&datetime=2025-06-15T13:00:00Z", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	traceID := rr.Header().Get("X-Trace-Id")
	if traceID == "" {
		t.Fatal("X-Trace-Id missing")
	}
	line := logs.line(t, "panic recovered")
	if got := field(t, line, "trace_id"); got != traceID {
		t.Errorf("panic log trace_id = %v, want %s; line: %s", got, traceID, line)
	}
	if got := field(t, line, "span_id"); got == nil {
		t.Errorf("panic log span_id missing; line: %s", line)
	}
}

// The access log is decorated by the handler now; a hand-rolled trace_id on top
// would emit the key twice in one JSON object, which log backends resolve
// arbitrarily (or reject).
func TestLogCorrelation_AccessLogHasTraceIDExactlyOnce(t *testing.T) {
	srv, logs := tracedServer(t, stubFeatures{}, stubBatchService{})

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil))

	traceID := rr.Header().Get("X-Trace-Id")
	line := logs.line(t, "request")
	if n := strings.Count(line, `"trace_id"`); n != 1 {
		t.Errorf("trace_id appears %d times in the access log, want 1; line: %s", n, line)
	}
	if got := field(t, line, "trace_id"); got != traceID {
		t.Errorf("access log trace_id = %v, want %s", got, traceID)
	}
}

// failingBatch fails after the NDJSON headers went out, so the handler can only
// log — that log is the sole trace of the broken stream on the server side.
type failingBatch struct{}

func (failingBatch) QueryBatch(context.Context, []input.BatchPoint, func(input.BatchItem) error) error {
	return errors.New("backend gone")
}

func TestLogCorrelation_BatchStreamAbortCarriesTraceID(t *testing.T) {
	srv, logs := tracedServer(t, stubFeatures{}, failingBatch{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/batch", strings.NewReader(batchBody(1)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/x-ndjson")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	traceID := rr.Header().Get("X-Trace-Id")
	if traceID == "" {
		t.Fatal("X-Trace-Id missing")
	}
	line := logs.line(t, "batch stream aborted")
	if got := field(t, line, "trace_id"); got != traceID {
		t.Errorf("batch abort log trace_id = %v, want %s; line: %s", got, traceID, line)
	}
}
