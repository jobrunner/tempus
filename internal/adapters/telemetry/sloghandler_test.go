package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// logLine logs one record through a SpanContextHandler over a JSON handler and
// decodes it, so tests can assert on the emitted keys.
func logLine(ctx context.Context, t *testing.T, wrap func(slog.Handler) slog.Handler) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	h := wrap(NewSpanContextHandler(slog.NewJSONHandler(&buf, nil)))
	slog.New(h).InfoContext(ctx, "hello")
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", buf.String(), err)
	}
	return got
}

func identity(h slog.Handler) slog.Handler { return h }

func TestSpanContextHandler_AddsTraceAndSpanID(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	got := logLine(ctx, t, identity)
	sc := span.SpanContext()
	if got["trace_id"] != sc.TraceID().String() {
		t.Errorf("trace_id = %v, want %s", got["trace_id"], sc.TraceID())
	}
	if got["span_id"] != sc.SpanID().String() {
		t.Errorf("span_id = %v, want %s", got["span_id"], sc.SpanID())
	}
}

func TestSpanContextHandler_NoSpanNoIDs(t *testing.T) {
	got := logLine(context.Background(), t, identity)
	if _, ok := got["trace_id"]; ok {
		t.Errorf("trace_id present without a span: %v", got)
	}
	if got["msg"] != "hello" {
		t.Errorf("msg = %v, want hello (record must still reach the inner handler)", got["msg"])
	}
}

// WithAttrs/WithGroup must keep the decorator in front of the derived handler,
// or span IDs would silently vanish from loggers built with logger.With(...).
func TestSpanContextHandler_DerivedHandlersKeepDecorating(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	got := logLine(ctx, t, func(h slog.Handler) slog.Handler {
		return h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	})
	if got["k"] != "v" || got["trace_id"] == nil {
		t.Errorf("WithAttrs: got %v, want k=v and a trace_id", got)
	}

	got = logLine(ctx, t, func(h slog.Handler) slog.Handler { return h.WithGroup("g") })
	group, _ := got["g"].(map[string]any)
	if group == nil || group["trace_id"] == nil {
		t.Errorf("WithGroup: got %v, want trace_id inside group g", got)
	}
}

func TestSpanContextHandler_EnabledDelegates(t *testing.T) {
	inner := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := NewSpanContextHandler(inner)
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info must be disabled when the inner handler's level is warn")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("error must be enabled when the inner handler's level is warn")
	}
}
