package telemetry

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// newInMemoryProvider builds a TracerProvider backed by an in-memory exporter
// so tests stay deterministic (no real network or OTLP backend needed).
func newInMemoryProvider(t *testing.T) (*trace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exp),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, exp
}

func TestOTelTracer_StartEndsSpan(t *testing.T) {
	tp, exp := newInMemoryProvider(t)
	tracer := NewOTelTracer(tp, "test")

	ctx, span := tracer.Start(context.Background(), "op.name")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "op.name" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "op.name")
	}
	_ = ctx
}

func TestOTelTracer_WithAttributes(t *testing.T) {
	tp, exp := newInMemoryProvider(t)
	tracer := NewOTelTracer(tp, "test")

	_, span := tracer.Start(context.Background(), "op",
		output.WithAttributes(
			output.Attribute{Key: "key", Value: "value"},
			output.Attribute{Key: "count", Value: 42},
		),
	)
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestOTelTracer_RecordError(t *testing.T) {
	tp, exp := newInMemoryProvider(t)
	tracer := NewOTelTracer(tp, "test")

	_, span := tracer.Start(context.Background(), "op")
	span.RecordError(errors.New("something went wrong"))
	span.SetStatus(output.StatusError, "failed")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if len(spans[0].Events) == 0 {
		t.Error("expected at least one event (exception) on span")
	}
}

func TestOTelTracer_SetStatusOK(t *testing.T) {
	tp, exp := newInMemoryProvider(t)
	tracer := NewOTelTracer(tp, "test")

	_, span := tracer.Start(context.Background(), "op")
	span.SetStatus(output.StatusOK, "")
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestOTelTracer_AddEvent(t *testing.T) {
	tp, exp := newInMemoryProvider(t)
	tracer := NewOTelTracer(tp, "test")

	_, span := tracer.Start(context.Background(), "op")
	span.AddEvent("cache-hit", output.Attribute{Key: "key", Value: "abc"})
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if len(spans[0].Events) == 0 {
		t.Error("expected event on span")
	}
}

func TestOTelTracer_SatisfiesPort(t *testing.T) {
	tp, _ := newInMemoryProvider(t)
	var _ output.Tracer = NewOTelTracer(tp, "test")
}
