package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// OTelTracer is an adapter that implements the output.Tracer port over a real
// go.opentelemetry.io/otel/trace.Tracer. All domain/application code stays
// telemetry-free by depending only on the port.
type OTelTracer struct {
	tracer trace.Tracer
}

// NewOTelTracer wraps a real OTel tracer.
func NewOTelTracer(tp trace.TracerProvider, name string) *OTelTracer {
	return &OTelTracer{tracer: tp.Tracer(name)}
}

// Start implements output.Tracer.
func (t *OTelTracer) Start(ctx context.Context, name string, opts ...output.StartSpanOption) (context.Context, output.Span) {
	var cfg output.StartSpanConfig
	for _, o := range opts {
		o(&cfg)
	}

	var otelOpts []trace.SpanStartOption
	if len(cfg.Attributes()) > 0 {
		otelOpts = append(otelOpts, trace.WithAttributes(toOTelAttrs(cfg.Attributes())...))
	}

	ctx, span := t.tracer.Start(ctx, name, otelOpts...)
	return ctx, &otelSpan{span: span}
}

// otelSpan adapts a real trace.Span to the output.Span port.
type otelSpan struct {
	span trace.Span
}

func (s *otelSpan) SetAttributes(attrs ...output.Attribute) {
	s.span.SetAttributes(toOTelAttrs(attrs)...)
}

func (s *otelSpan) AddEvent(name string, attrs ...output.Attribute) {
	s.span.AddEvent(name, trace.WithAttributes(toOTelAttrs(attrs)...))
}

func (s *otelSpan) RecordError(err error) {
	s.span.RecordError(err)
}

func (s *otelSpan) SetStatus(code output.StatusCode, description string) {
	s.span.SetStatus(toOTelStatusCode(code), description)
}

func (s *otelSpan) End() {
	s.span.End()
}

// toOTelAttrs converts port attributes to OTel key-value pairs.
func toOTelAttrs(attrs []output.Attribute) []attribute.KeyValue {
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		kvs = append(kvs, toOTelAttr(a))
	}
	return kvs
}

func toOTelAttr(a output.Attribute) attribute.KeyValue {
	k := attribute.Key(a.Key)
	switch v := a.Value.(type) {
	case string:
		return k.String(v)
	case bool:
		return k.Bool(v)
	case int:
		return k.Int(v)
	case int64:
		return k.Int64(v)
	case float64:
		return k.Float64(v)
	default:
		return k.String(fmt.Sprintf("%v", v))
	}
}

func toOTelStatusCode(code output.StatusCode) otelcodes.Code {
	switch code {
	case output.StatusOK:
		return otelcodes.Ok
	case output.StatusError:
		return otelcodes.Error
	default:
		return otelcodes.Unset
	}
}

// Compile-time assertion that OTelTracer satisfies the port.
var _ output.Tracer = (*OTelTracer)(nil)
