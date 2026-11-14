package omhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// networkRetryAfter is the retry hint for a request that never got a response.
const networkRetryAfter = 30 * time.Second

// GetJSON issues a GET to url and decodes the JSON body into dst, classifying
// every failure as an output.ProviderError the Open-Meteo adapters return as is:
//   - an error from client.Do that already is a ProviderError (e.g. the
//     transport's budget exhaustion) passes through unchanged; any other is
//     transient with a 30s hint;
//   - 429 and 5xx are transient, hinted by RetryAfter(resp, fallback);
//   - any other 4xx is permanent and carries the first 512 bytes of the body;
//   - an undecodable body is permanent.
func GetJSON(ctx context.Context, client *http.Client, url string, dst any, fallback time.Duration) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return output.NewPermanentError(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		if pe, ok := output.AsProviderError(err); ok {
			return pe
		}
		return output.NewTransientError(err, networkRetryAfter)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return output.NewTransientError(fmt.Errorf("open-meteo status %d", resp.StatusCode), RetryAfter(resp, fallback))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return output.NewPermanentError(fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, b))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return output.NewPermanentError(err)
	}
	return nil
}

// RetryAfter reads the response's Retry-After header so the envelope's hint
// reflects what Open-Meteo actually asked for. Only the delay-seconds form
// (a non-negative integer) is honored; an absent, negative, fractional or
// HTTP-date value yields fallback.
func RetryAfter(resp *http.Response, fallback time.Duration) time.Duration {
	if d, ok := retryAfterSeconds(resp); ok {
		return d
	}
	return fallback
}

// maxRetryAfterSeconds is the largest whole-second value a time.Duration holds;
// anything above overflows to a negative delay when multiplied by time.Second.
const maxRetryAfterSeconds = math.MaxInt64 / int64(time.Second)

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
