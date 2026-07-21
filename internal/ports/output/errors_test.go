package output

import (
	"errors"
	"testing"
	"time"
)

func TestAsProviderError(t *testing.T) {
	pe := NewTransientError(errors.New("dial tcp: timeout"), 30*time.Second)
	got, ok := AsProviderError(pe)
	if !ok || got.Class != ClassTransient || !got.Retryable || got.RetryAfter != 30*time.Second {
		t.Fatalf("transient not classified correctly: %+v ok=%v", got, ok)
	}
	if _, ok := AsProviderError(errors.New("plain")); ok {
		t.Error("plain error must not classify as ProviderError")
	}
	if !NewNotYetAvailableError(time.Hour).Retryable {
		t.Error("not-yet-available must be retryable")
	}
	if NewPermanentError(errors.New("x")).Retryable {
		t.Error("permanent must not be retryable")
	}
}
