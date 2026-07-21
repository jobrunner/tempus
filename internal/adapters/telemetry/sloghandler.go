package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// SpanContextHandler is a slog.Handler decorator that injects the current span's
// trace_id and span_id into every record carrying a context, so logs correlate
// with traces (e.g. in Grafana/Tempo).
type SpanContextHandler struct {
	inner slog.Handler
}

// NewSpanContextHandler wraps an existing handler.
func NewSpanContextHandler(inner slog.Handler) *SpanContextHandler {
	return &SpanContextHandler{inner: inner}
}

func (h *SpanContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *SpanContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *SpanContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SpanContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *SpanContextHandler) WithGroup(name string) slog.Handler {
	return &SpanContextHandler{inner: h.inner.WithGroup(name)}
}
