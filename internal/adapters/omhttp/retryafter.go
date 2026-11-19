package omhttp

import (
	"net/http"
	"strconv"
	"time"
)

// RetryAfter reads the response's Retry-After header so a retry waits as long
// as the server actually asked for. Only the delay-seconds form (a
// non-negative integer) is honored; an absent, negative, fractional or
// HTTP-date value yields fallback.
func RetryAfter(resp *http.Response, fallback time.Duration) time.Duration {
	if d, ok := retryAfterSeconds(resp); ok {
		return d
	}
	return fallback
}

// maxRetryAfterSeconds is the largest whole-second value a time.Duration holds
// (math.MaxInt64 / int64(time.Second), asserted in the tests); anything above
// overflows to a negative delay when multiplied by time.Second. A literal, not
// the expression: gremlins mutates arithmetic in a const, coverage can never
// reach it, and each such mutant counts against the package's mutant coverage.
const maxRetryAfterSeconds = 9223372036

// retryAfterSeconds parses Retry-After as integer seconds in [0, maxRetryAfterSeconds].
func retryAfterSeconds(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}
	secs, err := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 64)
	if err != nil || secs < 0 || secs > maxRetryAfterSeconds {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}
