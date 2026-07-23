package domain

import (
	"fmt"
	"strconv"
	"time"
)

// QueryRequest is a validated feature-query request. Instant is UTC, truncated
// to the hour. Providers is an optional filter (empty ⇒ all registered).
type QueryRequest struct {
	Coordinate Coordinate
	Instant    time.Time
	Providers  []string
}

// ValidationError is a client input error → HTTP 400, not retryable.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// Field name constants used in ValidationError — kept here so test assertions
// on Field values never drift from the production strings.
const (
	fieldDatetime = "datetime"
)

// datetimeLayouts are tried in order. RFC3339 (with offset) first; the
// offset-less forms are interpreted as UTC per the service contract.
var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

// ParseQueryRequest validates raw string inputs and builds a QueryRequest.
// Future instants are allowed here: astronomy providers (sun/moon) compute for
// any date. Providers that cannot serve the future (e.g. weather) reject it
// themselves and report it in the response envelope.
func ParseQueryRequest(lat, lon, datetime string, providers []string) (QueryRequest, error) {
	latF, err := strconv.ParseFloat(lat, 64)
	if err != nil || latF < -90 || latF > 90 {
		return QueryRequest{}, ValidationError{"lat", "must be a number in [-90,90]"}
	}
	lonF, err := strconv.ParseFloat(lon, 64)
	if err != nil || lonF < -180 || lonF > 180 {
		return QueryRequest{}, ValidationError{"lon", "must be a number in [-180,180]"}
	}

	instant, ok := parseInstant(datetime)
	if !ok {
		return QueryRequest{}, ValidationError{fieldDatetime, "must be RFC3339 or YYYY-MM-DDTHH:MM[:SS] (UTC assumed)"}
	}
	instant = instant.UTC().Truncate(time.Hour)

	return QueryRequest{
		Coordinate: Coordinate{Lat: latF, Lon: lonF},
		Instant:    instant,
		Providers:  providers,
	}, nil
}

func parseInstant(s string) (time.Time, bool) {
	for _, layout := range datetimeLayouts {
		// Offset-less layouts parse in UTC because we pass time.UTC via ParseInLocation.
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
