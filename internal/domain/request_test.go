package domain

import (
	"errors"
	"testing"
	"time"
)

func now() time.Time { return time.Date(2026, 7, 21, 12, 30, 0, 0, time.UTC) }

func TestParseQueryRequest_OK_AssumesUTCAndTruncatesHour(t *testing.T) {
	req, err := ParseQueryRequest("49.79", "9.93", "2025-06-15T13:45:00", nil, now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !req.Instant.Equal(time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)) {
		t.Errorf("Instant = %v, want 2025-06-15T13:00:00Z", req.Instant)
	}
}

func TestParseQueryRequest_FutureRejected(t *testing.T) {
	_, err := ParseQueryRequest("0", "0", "2026-07-21T13:00:00Z", nil, now())
	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "datetime" {
		t.Fatalf("want ValidationError on datetime (future), got %v", err)
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
			_, err := ParseQueryRequest(c.lat, c.lon, c.dt, nil, now())
			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("want ValidationError on %q, got %v", c.field, err)
			}
		})
	}
}
