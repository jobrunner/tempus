// Package omhttp provides the HTTP transport for the open-data bucket the
// Open-Meteo store reads from: transparent retry of 429/5xx/network failures
// honoring Retry-After, and one client span per attempt. The bucket is plain
// public object storage with no quota, so the transport neither rate-limits
// nor budgets. Adapters receive it as a plain *http.Client, so no adapter
// imports this package (composition happens in internal/app).
package omhttp

import (
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// eventRetry is the span event recorded on the caller's span (the inbound
// request span) for each backoff wait between per-attempt client spans.
const eventRetry = "open-meteo.retry"

// Options configures the transport.
type Options struct {
	// RetryAttempts is how many times a failed attempt is retried (0 = none).
	RetryAttempts int
	Base          http.RoundTripper
	// TracerProvider, when non-nil, wraps Base so each actual attempt becomes
	// its own client span under the caller's span. Nil disables that tracing.
	TracerProvider trace.TracerProvider
}

// Transport retries GET requests on 429/5xx/network failures, honoring
// Retry-After. Retries assume idempotent, body-less requests — every bucket
// read is a plain (ranged) GET; a request with a body is never retried.
type Transport struct {
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
		// nothing: the bucket is a third party and gets no trace context.
		base = otelhttp.NewTransport(base,
			otelhttp.WithTracerProvider(opts.TracerProvider),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator()),
		)
	}
	return &Transport{attempts: opts.RetryAttempts, base: base}
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
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
