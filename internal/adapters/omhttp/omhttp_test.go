package omhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/jobrunner/tempus/internal/ports/output"
)

func newClient(t *testing.T, opts Options) *http.Client {
	t.Helper()
	return &http.Client{Transport: NewTransport(opts), Timeout: 5 * time.Second}
}

func TestTransport_RetriesOn429ThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 3}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if calls.Load() != 3 {
		t.Fatalf("upstream calls = %d, want 3", calls.Load())
	}
}

func TestTransport_ExhaustedRetriesReturnLastResponse(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 2}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	if calls.Load() != 3 { // initial + 2 retries
		t.Fatalf("upstream calls = %d, want 3", calls.Load())
	}
}

func TestTransport_NoRetryOn400(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	resp, err := newClient(t, Options{RetryAttempts: 3}).Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1 (4xx must not retry)", calls.Load())
	}
}

func TestTransport_BudgetRejectsBatchOnly(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	budget := NewBudget(1, clk)
	client := newClient(t, Options{Budget: budget, Weight: 1})

	// Non-batch requests ignore the budget entirely.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if resp, err := client.Do(req); err != nil {
		t.Fatalf("non-batch request: %v", err)
	} else {
		resp.Body.Close()
	}

	// First batch request consumes the budget, second is rejected locally.
	bctx := output.WithBatchOrigin(req.Context())
	breq, _ := http.NewRequestWithContext(bctx, http.MethodGet, srv.URL, nil)
	if resp, err := client.Do(breq); err != nil {
		t.Fatalf("first batch request: %v", err)
	} else {
		resp.Body.Close()
	}
	before := calls.Load()
	breq2, _ := http.NewRequestWithContext(bctx, http.MethodGet, srv.URL, nil)
	resp2, err := client.Do(breq2)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	if resp2 != nil {
		resp2.Body.Close()
	}
	if calls.Load() != before {
		t.Fatal("budget rejection must not hit the upstream server")
	}
}

func TestTransport_RateLimiterPacesRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 1 token immediately (burst), then one per 50 ms.
	limiter := rate.NewLimiter(rate.Every(50*time.Millisecond), 1)
	client := newClient(t, Options{Limiter: limiter})
	start := time.Now()
	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		resp.Body.Close()
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("3 requests took %v, want >= 100ms (limiter must pace)", elapsed)
	}
}
