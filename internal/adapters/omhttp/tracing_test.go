package omhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// tracedSetup is a recording TracerProvider plus a "parent" span standing in
// for the inbound request span that otelmux opens in production.
type tracedSetup struct {
	rec    *tracetest.SpanRecorder
	tp     *sdktrace.TracerProvider
	ctx    context.Context
	parent trace.Span
}

func newTracedSetup(t *testing.T) *tracedSetup {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, parent := tp.Tracer("test").Start(context.Background(), "inbound")
	return &tracedSetup{rec: rec, tp: tp, ctx: ctx, parent: parent}
}

// finish ends the parent span and returns it plus every client span.
func (s *tracedSetup) finish(t *testing.T) (sdktrace.ReadOnlySpan, []sdktrace.ReadOnlySpan) {
	t.Helper()
	s.parent.End()
	var parent sdktrace.ReadOnlySpan
	var clients []sdktrace.ReadOnlySpan
	for _, sp := range s.rec.Ended() {
		switch {
		case sp.Name() == "inbound":
			parent = sp
		case sp.SpanKind() == trace.SpanKindClient:
			clients = append(clients, sp)
		}
	}
	if parent == nil {
		t.Fatal("parent span not recorded")
	}
	return parent, clients
}

// get issues one traced GET and closes the body; only the error matters here.
func (s *tracedSetup) get(ctx context.Context, t *testing.T, opts Options, url string) error {
	t.Helper()
	opts.TracerProvider = s.tp
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := newClient(t, opts).Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func statusAttr(sp sdktrace.ReadOnlySpan) (int64, bool) {
	for _, kv := range sp.Attributes() {
		if kv.Key == "http.response.status_code" {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

func eventsNamed(sp sdktrace.ReadOnlySpan, name string) []sdktrace.Event {
	var out []sdktrace.Event
	for _, e := range sp.Events() {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}

func attrOf(attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestTransport_TracesOneClientSpanPerCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTracedSetup(t)
	if err := s.get(s.ctx, t, Options{RetryAttempts: 2}, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parent, clients := s.finish(t)
	if len(clients) != 1 {
		t.Fatalf("client spans = %d, want 1", len(clients))
	}
	if got, ok := statusAttr(clients[0]); !ok || got != http.StatusOK {
		t.Errorf("http.response.status_code = %d (present %v), want 200", got, ok)
	}
	if clients[0].Parent().SpanID() != parent.SpanContext().SpanID() {
		t.Error("client span must be a child of the inbound span")
	}
}

func TestTransport_TracesEachRetryAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTracedSetup(t)
	if err := s.get(s.ctx, t, Options{RetryAttempts: 1}, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parent, clients := s.finish(t)
	if len(clients) != 2 {
		t.Fatalf("client spans = %d, want 2 (one per attempt)", len(clients))
	}
	retries := eventsNamed(parent, eventRetry)
	if len(retries) != 1 {
		t.Fatalf("retry events = %d, want 1", len(retries))
	}
	attrs := retries[0].Attributes
	if v, ok := attrOf(attrs, "retry.attempt"); !ok || v.AsInt64() != 1 {
		t.Errorf("retry.attempt = %v (present %v), want 1", v.AsInt64(), ok)
	}
	if v, ok := attrOf(attrs, "http.response.status_code"); !ok || v.AsInt64() != http.StatusServiceUnavailable {
		t.Errorf("status = %v (present %v), want 503", v.AsInt64(), ok)
	}
	if v, ok := attrOf(attrs, "retry.wait_ms"); !ok || v.AsInt64() != 0 {
		t.Errorf("retry.wait_ms = %v (present %v), want 0 (Retry-After: 0)", v.AsInt64(), ok)
	}
}

func TestTransport_RetryEventCarriesNetworkError(t *testing.T) {
	s := newTracedSetup(t)
	failing := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})
	opts := Options{RetryAttempts: 1, Base: failing}
	// Cancel during the backoff wait so the test stays fast; the event is
	// recorded before the wait starts.
	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()
	if err := s.get(ctx, t, opts, "http://open-meteo.invalid/"); err == nil {
		t.Fatal("want an error")
	}

	parent, _ := s.finish(t)
	retries := eventsNamed(parent, eventRetry)
	if len(retries) != 1 {
		t.Fatalf("retry events = %d, want 1", len(retries))
	}
	if v, ok := attrOf(retries[0].Attributes, "error.message"); !ok || v.AsString() != "connection refused" {
		t.Errorf("error.message = %q (present %v), want the network error", v.AsString(), ok)
	}
	if v, ok := attrOf(retries[0].Attributes, "retry.wait_ms"); !ok || v.AsInt64() != 2000 {
		t.Errorf("retry.wait_ms = %d (present %v), want 2000 (first backoff)", v.AsInt64(), ok)
	}
}

func TestTransport_BudgetExhaustedRecordsEventWithoutClientSpan(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTracedSetup(t)
	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	opts := Options{Budget: NewBudget(0, clk), Weight: 1}
	err := s.get(output.WithBatchOrigin(s.ctx), t, opts, srv.URL)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}

	parent, clients := s.finish(t)
	if len(clients) != 0 || calls.Load() != 0 {
		t.Fatalf("client spans = %d, upstream calls = %d; want none", len(clients), calls.Load())
	}
	events := eventsNamed(parent, eventBudgetExhausted)
	if len(events) != 1 {
		t.Fatalf("budget events = %d, want 1", len(events))
	}
	if v, ok := attrOf(events[0].Attributes, "budget.weight"); !ok || v.AsInt64() != 1 {
		t.Errorf("budget.weight = %d (present %v), want 1", v.AsInt64(), ok)
	}
}

func TestTransport_NilTracerProviderRecordsNoSpans(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newTracedSetup(t)
	req, _ := http.NewRequestWithContext(s.ctx, http.MethodGet, srv.URL, nil)
	resp, err := newClient(t, Options{}).Do(req) // TracerProvider left nil
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if _, clients := s.finish(t); len(clients) != 0 {
		t.Fatalf("client spans = %d, want 0 without a TracerProvider", len(clients))
	}
	if _, ok := NewTransport(Options{}).base.(*http.Transport); !ok {
		t.Error("nil TracerProvider must keep the bare base transport")
	}
}

// Open-Meteo is a third party: our trace IDs must not leak to it.
func TestTransport_DoesNotPropagateTraceContext(t *testing.T) {
	var leaked atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"traceparent", "tracestate", "baggage"} {
			if v := r.Header.Get(h); v != "" {
				leaked.Store(h + ": " + v)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Production installs a W3C propagator globally (telemetry.NewTracerProvider);
	// without it here the test would pass even if the transport used the global.
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	s := newTracedSetup(t)
	if err := s.get(s.ctx, t, Options{}, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v := leaked.Load(); v != nil {
		t.Fatalf("trace context leaked upstream: %v", v)
	}
	if _, clients := s.finish(t); len(clients) != 1 {
		t.Fatalf("client spans = %d, want 1 (the span exists, only propagation is off)", len(clients))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
