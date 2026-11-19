package omhttp

import (
	"math"
	"net/http"
	"testing"
	"time"
)

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

func TestRetryAfter_NilResponseYieldsFallback(t *testing.T) {
	if got := RetryAfter(nil, time.Minute); got != time.Minute {
		t.Errorf("RetryAfter(nil) = %v, want 1m", got)
	}
}

func TestMaxRetryAfterSecondsIsLargestWholeSecondDuration(t *testing.T) {
	if want := int64(math.MaxInt64 / time.Second); maxRetryAfterSeconds != want {
		t.Fatalf("maxRetryAfterSeconds = %d, want %d", maxRetryAfterSeconds, want)
	}
}
