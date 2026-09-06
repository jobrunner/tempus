package omhttp

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// ErrBudgetExhausted is returned (without any upstream call) when a
// batch-originated request finds today's weighted call budget spent.
var ErrBudgetExhausted = errors.New("open-meteo daily budget exhausted; retry tomorrow")

// Options configures the transport. Limiter and Budget are shared across all
// Open-Meteo adapters; Weight is the per-call cost this client charges.
type Options struct {
	Limiter       *rate.Limiter
	Budget        *Budget
	Weight        int
	RetryAttempts int
	Base          http.RoundTripper
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
		return nil, ErrBudgetExhausted
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
		wait := retryDelay(resp, attempt)
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
		}
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(wait):
		}
	}
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
	if resp != nil {
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return 2 * time.Second << attempt
}
