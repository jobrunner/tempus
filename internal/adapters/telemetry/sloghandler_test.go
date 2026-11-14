package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"go.opentelemetry.io/otel/trace"
)

// spanCtx returns a context carrying a fixed, valid (remote) span context so
// the expected IDs are known up front.
func spanCtx(t *testing.T) (context.Context, trace.SpanContext) {
	t.Helper()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x0a, 0x1b, 0x2c, 0x3d, 0x4e, 0x5f, 0x60, 0x71, 0x82, 0x93, 0xa4, 0xb5, 0xc6, 0xd7, 0xe8, 0xf9},
		SpanID:     trace.SpanID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		TraceFlags: trace.FlagsSampled,
	})
	if !sc.IsValid() {
		t.Fatal("test span context is not valid")
	}
	return trace.ContextWithSpanContext(context.Background(), sc), sc
}

// jsonLogger builds the production decoration (SpanContextHandler over a JSON
// handler) writing into buf.
func jsonLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(NewSpanContextHandler(slog.NewJSONHandler(buf, nil)))
}

// lastLine decodes the single JSON record in buf.
func lastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("want exactly one log line, got %d: %q", len(lines), buf.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatalf("decode %q: %v", lines[0], err)
	}
	return m
}

func TestSpanContextHandler_AddsTraceAndSpanID(t *testing.T) {
	var buf bytes.Buffer
	ctx, sc := spanCtx(t)

	jsonLogger(&buf).InfoContext(ctx, "hello", "k", "v")

	m := lastLine(t, &buf)
	if m["trace_id"] != sc.TraceID().String() {
		t.Errorf("trace_id = %v, want %s", m["trace_id"], sc.TraceID())
	}
	if m["span_id"] != sc.SpanID().String() {
		t.Errorf("span_id = %v, want %s", m["span_id"], sc.SpanID())
	}
	if m["k"] != "v" {
		t.Errorf("record attr k = %v, want v", m["k"])
	}
}

// Without a valid span there is nothing to correlate, so the keys must be
// absent rather than present-but-zero (an all-zero trace_id would match every
// other uncorrelated line in a log backend).
func TestSpanContextHandler_NoValidSpan_NoKeys(t *testing.T) {
	tests := map[string]context.Context{
		"no span":      context.Background(),
		"invalid span": trace.ContextWithSpanContext(context.Background(), trace.SpanContext{}),
	}
	for name, ctx := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			jsonLogger(&buf).InfoContext(ctx, "hello")

			m := lastLine(t, &buf)
			for _, k := range []string{"trace_id", "span_id"} {
				if _, ok := m[k]; ok {
					t.Errorf("%s present (%v), want absent", k, m[k])
				}
			}
		})
	}
}

func TestSpanContextHandler_EnabledDelegatesLevel(t *testing.T) {
	h := NewSpanContextHandler(slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(Info) = true, want false below the inner Warn level")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("Enabled(Warn) = false, want true")
	}
	// Derived handlers must keep delegating, too.
	if h.WithAttrs([]slog.Attr{slog.String("a", "b")}).Enabled(ctx, slog.LevelInfo) {
		t.Error("WithAttrs handler: Enabled(Info) = true, want false")
	}
	if h.WithGroup("g").Enabled(ctx, slog.LevelInfo) {
		t.Error("WithGroup handler: Enabled(Info) = true, want false")
	}
}

// logger.With(...) is how request-scoped loggers are built; the decoration must
// survive it.
func TestSpanContextHandler_WithAttrsKeepsDecoration(t *testing.T) {
	var buf bytes.Buffer
	ctx, sc := spanCtx(t)

	jsonLogger(&buf).With("component", "http").InfoContext(ctx, "hello")

	m := lastLine(t, &buf)
	if m["component"] != "http" {
		t.Errorf("component = %v, want http", m["component"])
	}
	if m["trace_id"] != sc.TraceID().String() {
		t.Errorf("trace_id = %v, want %s", m["trace_id"], sc.TraceID())
	}
}

// trace_id/span_id are correlation keys a log backend looks up at the top level
// (Grafana's derived field, Loki's JSON parser). If a caller opened a group,
// they must NOT end up as grp.trace_id — the group still applies to everything
// else, exactly as slog promises.
func TestSpanContextHandler_WithGroupKeepsTraceIDsTopLevel(t *testing.T) {
	var buf bytes.Buffer
	ctx, sc := spanCtx(t)

	jsonLogger(&buf).
		With("pre", 1).
		WithGroup("req").
		With("method", "GET").
		WithGroup("inner").
		InfoContext(ctx, "hello", "k", "v")

	m := lastLine(t, &buf)
	if m["trace_id"] != sc.TraceID().String() {
		t.Errorf("top-level trace_id = %v, want %s (line: %s)", m["trace_id"], sc.TraceID(), buf.String())
	}
	if m["span_id"] != sc.SpanID().String() {
		t.Errorf("top-level span_id = %v, want %s (line: %s)", m["span_id"], sc.SpanID(), buf.String())
	}
	if m["pre"] != float64(1) {
		t.Errorf("pre = %v, want 1 at top level", m["pre"])
	}
	req, _ := m["req"].(map[string]any)
	if req == nil || req["method"] != "GET" {
		t.Fatalf("req group = %v, want method=GET inside it", m["req"])
	}
	inner, _ := req["inner"].(map[string]any)
	if inner == nil || inner["k"] != "v" {
		t.Errorf("req.inner group = %v, want k=v inside it", req["inner"])
	}
	if _, ok := inner["trace_id"]; ok {
		t.Error("trace_id leaked into the open group")
	}
}

// The handler must stay a correct slog.Handler (groups, empty groups, attrs
// from With, zero time, …) — slogtest is the stdlib's conformance suite.
func TestSpanContextHandler_Conformance(t *testing.T) {
	var buf bytes.Buffer
	h := NewSpanContextHandler(slog.NewJSONHandler(&buf, nil))
	results := func() []map[string]any {
		var ms []map[string]any
		for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(line, &m); err != nil {
				t.Fatal(err)
			}
			ms = append(ms, m)
		}
		return ms
	}
	if err := slogtest.TestHandler(h, results); err != nil {
		t.Error(err)
	}
}
