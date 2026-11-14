package omhttp

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// roundTripperFunc adapts a function to http.RoundTripper for tests.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// getJSONCase is one GetJSON scenario.
type getJSONCase struct {
	name string
	// handler serves the upstream; nil means the server is closed before
	// the call, so the request fails at the network level.
	handler   http.HandlerFunc
	transport http.RoundTripper
	wantErr   bool
	wantClass output.ErrorClass
	wantRetry time.Duration
	wantMsg   string
	wantSame  *output.ProviderError
}

func TestGetJSON(t *testing.T) {
	const fallback = 42 * time.Second
	budget := output.NewTransientError(ErrBudgetExhausted, 3*time.Hour)

	tests := []getJSONCase{
		{
			name: "200 decodes",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"value":3}`))
			},
		},
		{
			name: "429 honors integer Retry-After",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantErr: true, wantClass: output.ClassTransient, wantRetry: 7 * time.Second,
		},
		{
			name: "429 without Retry-After uses fallback",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantErr: true, wantClass: output.ClassTransient, wantRetry: fallback,
		},
		{
			name: "negative Retry-After uses fallback",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "-5")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantErr: true, wantClass: output.ClassTransient, wantRetry: fallback,
		},
		{
			name: "HTTP-date Retry-After uses fallback",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "Wed, 21 Oct 2026 07:28:00 GMT")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantErr: true, wantClass: output.ClassTransient, wantRetry: fallback,
		},
		{
			name: "503 is transient",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			wantErr: true, wantClass: output.ClassTransient, wantRetry: fallback,
			wantMsg: "open-meteo status 503",
		},
		{
			name: "404 is permanent and carries the body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("no such endpoint"))
			},
			wantErr: true, wantClass: output.ClassPermanent,
			wantMsg: "open-meteo status 404: no such endpoint",
		},
		{
			name: "broken JSON is permanent",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"value":`))
			},
			wantErr: true, wantClass: output.ClassPermanent,
		},
		{
			name:    "network error is transient 30s",
			wantErr: true, wantClass: output.ClassTransient, wantRetry: 30 * time.Second,
		},
		{
			name: "classified transport error passes through",
			transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, budget
			}),
			wantErr: true, wantSame: &budget,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			if tc.handler == nil {
				srv.Close()
			} else {
				defer srv.Close()
			}
			client := &http.Client{Timeout: 2 * time.Second, Transport: tc.transport}

			var dst struct {
				Value int `json:"value"`
			}
			err := GetJSON(context.Background(), client, srv.URL, &dst, fallback)
			checkGetJSON(t, tc, err, dst.Value)
		})
	}
}

// checkGetJSON asserts one GetJSON outcome against its case.
func checkGetJSON(t *testing.T, tc getJSONCase, err error, value int) {
	t.Helper()
	if !tc.wantErr {
		if err != nil {
			t.Fatalf("GetJSON err = %v, want nil", err)
		}
		if value != 3 {
			t.Errorf("decoded value = %d, want 3", value)
		}
		return
	}
	pe, ok := output.AsProviderError(err)
	if !ok {
		t.Fatalf("GetJSON err = %v, want output.ProviderError", err)
	}
	if tc.wantSame != nil {
		if pe.Class != tc.wantSame.Class || pe.RetryAfter != tc.wantSame.RetryAfter ||
			!errors.Is(pe, ErrBudgetExhausted) {
			t.Errorf("err = %+v, want unchanged %+v", pe, *tc.wantSame)
		}
		return
	}
	if pe.Class != tc.wantClass {
		t.Errorf("Class = %d, want %d", pe.Class, tc.wantClass)
	}
	if pe.Retryable != (tc.wantClass == output.ClassTransient) {
		t.Errorf("Retryable = %v for class %d", pe.Retryable, pe.Class)
	}
	if pe.RetryAfter != tc.wantRetry {
		t.Errorf("RetryAfter = %v, want %v", pe.RetryAfter, tc.wantRetry)
	}
	if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
		t.Errorf("err = %q, want it to contain %q", err, tc.wantMsg)
	}
}

func TestRetryAfter(t *testing.T) {
	const fallback = 60 * time.Second
	tests := []struct {
		header string
		want   time.Duration
	}{
		{"", fallback},
		{"0", 0},
		{"7", 7 * time.Second},
		{"-5", fallback},
		{"1.5", fallback},
		{"abc", fallback},
		{"Wed, 21 Oct 2026 07:28:00 GMT", fallback},
		// The largest whole-second Duration, and one past it: multiplying the
		// latter by time.Second would overflow into a negative delay.
		{"9223372036", 9223372036 * time.Second},
		{"9223372037", fallback},
	}
	for _, tc := range tests {
		resp := &http.Response{Header: http.Header{}}
		if tc.header != "" {
			resp.Header.Set("Retry-After", tc.header)
		}
		if got := RetryAfter(resp, fallback); got != tc.want {
			t.Errorf("RetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestMaxRetryAfterSecondsIsLargestWholeSecondDuration(t *testing.T) {
	if want := int64(math.MaxInt64 / time.Second); maxRetryAfterSeconds != want {
		t.Fatalf("maxRetryAfterSeconds = %d, want %d", maxRetryAfterSeconds, want)
	}
}
