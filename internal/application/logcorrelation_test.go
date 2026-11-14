package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	// Test-only: the production logger decoration, so the test proves the log
	// line itself carries trace_id. depguard exempts _test.go from layer rules.
	"github.com/jobrunner/tempus/internal/adapters/telemetry"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// syncBuffer is safe for the concurrent provider goroutines that log.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// tracedQuery runs svc.Query inside a sampled span and returns that span's
// trace ID, so log lines can be checked against it.
func tracedQuery(t *testing.T, svc *FeatureService) string {
	t.Helper()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "query")
	defer span.End()
	if _, err := svc.Query(ctx, sampleReq()); err != nil {
		t.Fatalf("Query: %v", err)
	}
	return span.SpanContext().TraceID().String()
}

func traceIDOf(t *testing.T, logs *syncBuffer, msg string) any {
	t.Helper()
	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, l := range strings.Split(strings.TrimSpace(logs.buf.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == msg {
			return m["trace_id"]
		}
	}
	t.Fatalf("no %q log line; log:\n%s", msg, logs.buf.String())
	return nil
}

// Provider faults are logged from inside the per-provider goroutines; each of
// those lines must carry the request's trace_id, or a degraded response cannot
// be tied to the trace that shows which upstream call failed.
func TestFeatureService_ProviderFaultLogsCarryTraceID(t *testing.T) {
	incomplete := domain.License{Name: "x", Attribution: "y"} // no URL
	badFeat := domain.NewPointFeature(domain.Coordinate{Lat: 1, Lon: 2}, map[string]any{"v": 1.0}, incomplete)

	tests := []struct {
		name     string
		provider output.FeatureProvider
		derivers []output.FeatureDeriver
		msg      string
	}{
		{
			name:     "unclassified provider error",
			provider: failProvider{"raw", errors.New("boom")},
			msg:      "unclassified provider error",
		},
		{
			name:     "provider without attribution",
			provider: okProvider{id: "naughty", feat: badFeat},
			msg:      "provider returned a feature without complete attribution",
		},
		{
			name:     "deriver without attribution",
			derivers: []output.FeatureDeriver{fakeDeriver{id: "bad-deriver", feat: &badFeat}},
			msg:      "deriver returned a feature without complete attribution",
		},
		{
			name:     "unclassified deriver error",
			derivers: []output.FeatureDeriver{fakeDeriver{id: "raw-deriver", err: errors.New("boom")}},
			msg:      "unclassified provider error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := &syncBuffer{}
			logger := slog.New(telemetry.NewSpanContextHandler(slog.NewJSONHandler(logs, nil)))
			reg := NewRegistry()
			if tt.provider != nil {
				reg.Register(tt.provider)
			}
			svc := NewFeatureService(reg, tt.derivers, logger, 5*time.Second)

			want := tracedQuery(t, svc)

			if got := traceIDOf(t, logs, tt.msg); got != want {
				t.Errorf("trace_id = %v, want %s", got, want)
			}
		})
	}
}
