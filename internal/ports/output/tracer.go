package output

import "context"

// Tracer is the secondary port for distributed tracing. Adapters live in
// internal/adapters/telemetry. The default implementation is NoOpTracer, so the
// composition root can always inject a non-nil Tracer (no nil checks on the hot
// path).
type Tracer interface {
	// Start opens a new span and returns a context carrying it. The caller must
	// End() the span exactly once. If ctx already carries a span, the new span
	// becomes its child.
	Start(ctx context.Context, name string, opts ...StartSpanOption) (context.Context, Span)
}

// Span is an in-progress operation being traced.
type Span interface {
	SetAttributes(attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	RecordError(err error)
	SetStatus(code StatusCode, description string)
	End()
}

// StatusCode is the outcome of a span.
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// Attribute is a typed key/value pair on a span (kept minimal; extend as needed).
type Attribute struct {
	Key   string
	Value any
}

// StartSpanOption configures a span at creation (span kind, initial attributes…).
type StartSpanOption func(*StartSpanConfig)

// StartSpanConfig holds the resolved options for a new span. Adapters apply
// all options to a zero-value StartSpanConfig to extract the configuration.
type StartSpanConfig struct {
	attributes []Attribute
}

// Attributes returns the initial attributes collected from all applied options.
func (c *StartSpanConfig) Attributes() []Attribute { return c.attributes }

// WithAttributes sets initial attributes on the span.
func WithAttributes(attrs ...Attribute) StartSpanOption {
	return func(c *StartSpanConfig) { c.attributes = append(c.attributes, attrs...) }
}

// NoOpTracer is the zero-value Tracer; it discards everything.
type NoOpTracer struct{}

// Start implements Tracer.
func (NoOpTracer) Start(ctx context.Context, _ string, _ ...StartSpanOption) (context.Context, Span) {
	return ctx, noOpSpan{}
}

type noOpSpan struct{}

func (noOpSpan) SetAttributes(_ ...Attribute)      {}
func (noOpSpan) AddEvent(_ string, _ ...Attribute) {}
func (noOpSpan) RecordError(_ error)               {}
func (noOpSpan) SetStatus(_ StatusCode, _ string)  {}
func (noOpSpan) End()                              {}

// Compile-time assertion that NoOpTracer satisfies the port.
var _ Tracer = NoOpTracer{}
