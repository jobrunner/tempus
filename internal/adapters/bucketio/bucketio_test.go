package bucketio

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
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

func TestDefaultBase_KeepsIdleConnectionsForParallelReads(t *testing.T) {
	tr, ok := defaultBase(32).(*http.Transport)
	if !ok {
		t.Fatal("default base must be an *http.Transport")
	}
	if tr.MaxIdleConnsPerHost != 32 || tr.MaxIdleConns < 32 {
		t.Errorf("idle per host = %d, total = %d; want 32 and >= 32", tr.MaxIdleConnsPerHost, tr.MaxIdleConns)
	}
	if tr == http.DefaultTransport {
		t.Error("the shared default transport must not be modified")
	}
	if d := defaultBase(0).(*http.Transport); d.MaxIdleConnsPerHost != http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost {
		t.Errorf("zero must keep Go's default, got %d", d.MaxIdleConnsPerHost)
	}
}
