package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// SpanContextHandler is a slog.Handler decorator that injects the current span's
// trace_id and span_id into every record carrying a context, so logs correlate
// with traces (e.g. in Grafana/Tempo). Only the *Context logging variants
// (InfoContext, ErrorContext, …) hand it a context; a plain Info cannot be
// correlated.
//
// trace_id/span_id always land at the top level of the record, even under
// WithGroup: they are lookup keys for log backends, and slog would otherwise
// qualify them as "<group>.trace_id". To get there, groups — and every attr
// added after the first group — are kept here and applied in Handle rather
// than pushed into the inner handler. Attrs added before any group still go
// straight to inner.WithAttrs, so the common logger.With(...) path keeps the
// inner handler's pre-formatting.
type SpanContextHandler struct {
	inner slog.Handler
	// goas holds, in order, the groups and the attrs added after the first
	// group. Empty unless WithGroup was called.
	goas []groupOrAttrs
}

// groupOrAttrs is one WithGroup (group != "") or WithAttrs (attrs) call.
type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

// NewSpanContextHandler wraps an existing handler.
func NewSpanContextHandler(inner slog.Handler) *SpanContextHandler {
	return &SpanContextHandler{inner: inner}
}

func (h *SpanContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *SpanContextHandler) Handle(ctx context.Context, r slog.Record) error {
	var traceAttrs []slog.Attr
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		traceAttrs = []slog.Attr{
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		}
	}
	if len(h.goas) == 0 {
		r.AddAttrs(traceAttrs...)
		return h.inner.Handle(ctx, r)
	}

	// Rebuild the record with the open groups applied by hand: the record's own
	// attrs are the innermost level, and each WithAttrs/WithGroup wraps it from
	// the inside out. The trace attrs are prepended outside all groups.
	nested := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		nested = append(nested, a)
		return true
	})
	for i := len(h.goas) - 1; i >= 0; i-- {
		g := h.goas[i]
		if g.group == "" {
			nested = append(append([]slog.Attr{}, g.attrs...), nested...)
			continue
		}
		// slog.Group with no attrs is elided by handlers, matching how a group
		// with nothing in it is dropped when slog applies it itself.
		nested = []slog.Attr{{Key: g.group, Value: slog.GroupValue(nested...)}}
	}
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	out.AddAttrs(traceAttrs...)
	out.AddAttrs(nested...)
	return h.inner.Handle(ctx, out)
}

func (h *SpanContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	if len(h.goas) == 0 {
		return &SpanContextHandler{inner: h.inner.WithAttrs(attrs)}
	}
	return h.with(groupOrAttrs{attrs: attrs})
}

func (h *SpanContextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.with(groupOrAttrs{group: name})
}

// with returns a copy of h with g appended; the slice is copied so sibling
// handlers derived from h never share a backing array.
func (h *SpanContextHandler) with(g groupOrAttrs) *SpanContextHandler {
	goas := make([]groupOrAttrs, 0, len(h.goas)+1)
	goas = append(goas, h.goas...)
	goas = append(goas, g)
	return &SpanContextHandler{inner: h.inner, goas: goas}
}
