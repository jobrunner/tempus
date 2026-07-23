package domain

import (
	"errors"
	"testing"
	"time"
)

func TestParseQueryRequest_OK_AssumesUTCAndTruncatesHour(t *testing.T) {
	req, err := ParseQueryRequest("49.79", "9.93", "2025-06-15T13:45:00", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !req.Instant.Equal(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("Instant = %v, want 2025-06-15T13:00:00Z", req.Instant)
	}
}

// Future instants are now accepted by the parser: astronomy providers serve any
// date, and weather rejects the future itself (reported in the envelope).
func TestParseQueryRequest_FutureAccepted(t *testing.T) {
	req, err := ParseQueryRequest("0", "0", "2099-01-01T13:00:00Z", nil)
	if err != nil {
		t.Fatalf("future instant should parse, got error: %v", err)
	}
	if !req.Instant.Equal(time.Date(2099, 1, 1, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("Instant = %v, want 2099-01-01T13:00:00Z", req.Instant)
	}
}

func TestParseQueryRequest_BadInputs(t *testing.T) {
	cases := []struct{ name, lat, lon, dt, field string }{
		{"lat-range", "91", "0", "2020-01-01T00:00:00Z", "lat"},
		{"lon-range", "0", "181", "2020-01-01T00:00:00Z", "lon"},
		{"bad-datetime", "0", "0", "not-a-date", "datetime"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseQueryRequest(c.lat, c.lon, c.dt, nil)
			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("want ValidationError on %q, got %v", c.field, err)
			}
		})
	}
}
