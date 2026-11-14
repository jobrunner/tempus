package omhttp

import (
	"errors"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// ErrBudgetExhausted is returned (without any upstream call) when a
// batch-originated request finds today's weighted call budget spent.
var ErrBudgetExhausted = errors.New("open-meteo daily budget exhausted; retry tomorrow")

// Span events recorded on the caller's span (the inbound request span) for the
// waits and rejections that happen around the per-attempt client spans.
const (
	eventRetry           = "open-meteo.retry"
	eventBudgetExhausted = "open-meteo.budget_exhausted"
)

// Options configures the transport. Limiter and Budget are shared across all
// Open-Meteo adapters; Weight is the per-call cost this client charges.
type Options struct {
	Limiter       *rate.Limiter
	Budget        *Budget
	Weight        int
	RetryAttempts int
	Base          http.RoundTripper
	// TracerProvider, when non-nil, wraps Base so each actual attempt becomes
	// its own client span under the caller's span. Nil disables that tracing.
	TracerProvider trace.TracerProvider
}

// Transport rate-limits, retries (429/5xx/network, honoring Retry-After), and
// budget-guards Open-Meteo GET requests. Retries assume idempotent, body-less
// requests — all Open-Meteo calls are plain GETs.
type Transport struct {
	limiter  *rate.Limiter
	budget   *Budget
	weight   int
	attempts int
	base     http.RoundTripper
}

// NewTransport builds the transport; a nil Base falls back to
// http.DefaultTransport.
func NewTransport(opts Options) *Transport {
	base := opts.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if opts.TracerProvider != nil {
		// Wrapping the base (not this Transport) gives one span per attempt, so
		// retries show up individually. The empty composite propagator injects
		// nothing: Open-Meteo is a third party and gets no trace context.
		base = otelhttp.NewTransport(base,
			otelhttp.WithTracerProvider(opts.TracerProvider),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		)
	}
	return &Transport{
		limiter:  opts.Limiter,
		budget:   opts.Budget,
		weight:   opts.Weight,
		attempts: opts.RetryAttempts,
		base:     base,
	}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.budget != nil && output.IsBatchOrigin(req.Context()) && !t.budget.TryReserve(t.weight) {
		trace.SpanFromContext(req.Context()).AddEvent(eventBudgetExhausted,
			trace.WithAttributes(attribute.Int("budget.weight", t.weight)))
		// The retry hint is "until UTC midnight", when the daily budget rolls
		// over — not a fabricated short backoff.
		return nil, output.NewTransientError(ErrBudgetExhausted, t.budget.UntilReset())
	}
	for attempt := 0; ; attempt++ {
		if t.limiter != nil {
			if err := t.limiter.Wait(req.Context()); err != nil {
				return nil, err
			}
		}
		resp, err := t.base.RoundTrip(req)
		if !shouldRetry(resp, err) || attempt >= t.attempts || req.Body != nil {
			return resp, err
		}
		if err := waitBeforeRetry(req, resp, err, attempt); err != nil {
			return nil, err
		}
	}
}

// waitBeforeRetry records a retry event, drains and closes a non-nil response
// body, then blocks until either the retry delay elapses or the request's
// context is done.
func waitBeforeRetry(req *http.Request, resp *http.Response, err error, attempt int) error {
	wait := retryDelay(resp, attempt)
	recordRetry(req, resp, err, attempt, wait)
	if resp != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}
	select {
	case <-req.Context().Done():
		return req.Context().Err()
	case <-time.After(wait):
		return nil
	}
}

// recordRetry adds a retry event to the caller's span (a no-op without one).
// attempt is the 1-based number of the retry about to be made.
func recordRetry(req *http.Request, resp *http.Response, err error, attempt int, wait time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.Int("retry.attempt", attempt+1),
		attribute.Int64("retry.wait_ms", wait.Milliseconds()),
	}
	if err != nil {
		attrs = append(attrs, attribute.String("error.message", err.Error()))
	} else {
		attrs = append(attrs, attribute.Int("http.response.status_code", resp.StatusCode))
	}
	trace.SpanFromContext(req.Context()).AddEvent(eventRetry, trace.WithAttributes(attrs...))
}

// shouldRetry: network errors and 429/5xx are worth retrying; 4xx are not.
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
}

// retryDelay prefers the server's Retry-After (seconds) and falls back to
// exponential backoff starting at 2s.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if d, ok := retryAfterSeconds(resp); ok {
		return d
	}
	return 2 * time.Second << attempt
}
