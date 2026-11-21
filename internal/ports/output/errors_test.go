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

func TestProviderError_MessageAndUnwrap(t *testing.T) {
	cause := errors.New("bucket down")
	pe := NewPermanentError(cause)
	if !errors.Is(pe, cause) {
		t.Error("ProviderError must unwrap to its cause")
	}
	if msg := pe.Error(); msg != "provider error (class 2): bucket down" {
		t.Errorf("Error() = %q", msg)
	}
}
