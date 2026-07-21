package output

import (
	"errors"
	"fmt"
	"time"
)

// ErrorClass classifies a provider failure for retry coding.
type ErrorClass int

const (
	ClassTransient       ErrorClass = iota // network/5xx/timeout/429 — retry soon
	ClassNotYetAvailable                   // past but not yet in the archive — retry later
	ClassPermanent                         // 4xx/contract violation — do not retry
)

// ProviderError is a classified failure a FeatureProvider may return.
type ProviderError struct {
	Class      ErrorClass
	Retryable  bool
	RetryAfter time.Duration // 0 = unknown
	Err        error
}

func (e ProviderError) Error() string {
	return fmt.Sprintf("provider error (class %d): %v", e.Class, e.Err)
}
func (e ProviderError) Unwrap() error { return e.Err }

// NewTransientError: source unreachable / temporary — retryable.
func NewTransientError(err error, retryAfter time.Duration) ProviderError {
	return ProviderError{Class: ClassTransient, Retryable: true, RetryAfter: retryAfter, Err: err}
}

// NewNotYetAvailableError: the datetime is valid+past but the source has no data
// for it yet (archive delay) — retryable once the data matures.
func NewNotYetAvailableError(retryAfter time.Duration) ProviderError {
	return ProviderError{Class: ClassNotYetAvailable, Retryable: true, RetryAfter: retryAfter,
		Err: errors.New("data not yet available for the requested time")}
}

// NewPermanentError: the source rejected the request in a non-recoverable way.
func NewPermanentError(err error) ProviderError {
	return ProviderError{Class: ClassPermanent, Retryable: false, Err: err}
}

// AsProviderError extracts a ProviderError from err, if present.
func AsProviderError(err error) (ProviderError, bool) {
	var pe ProviderError
	if errors.As(err, &pe) {
		return pe, true
	}
	return ProviderError{}, false
}
