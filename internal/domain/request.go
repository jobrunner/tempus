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
	Timezone   *time.Location
	TimezoneID string
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

// datetimeLayouts are tried in order. RFC3339 (with offset) first; the
// offset-less forms are interpreted as UTC per the service contract.
var datetimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

// ParseQueryRequest validates raw string inputs and builds a QueryRequest.
// now is injected (from the Clock port) so "future" is testable.
func ParseQueryRequest(lat, lon, datetime, tzID string, providers []string, now time.Time) (QueryRequest, error) {
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
		return QueryRequest{}, ValidationError{"datetime", "must be RFC3339 or YYYY-MM-DDTHH:MM[:SS] (UTC assumed)"}
	}
	instant = instant.UTC().Truncate(time.Hour)
	if instant.After(now.UTC()) {
		return QueryRequest{}, ValidationError{"datetime", "must not be in the future"}
	}

	loc, err := time.LoadLocation(tzID)
	if err != nil {
		return QueryRequest{}, ValidationError{"timezone", "must be a valid IANA timezone id"}
	}

	return QueryRequest{
		Coordinate: Coordinate{Lat: latF, Lon: lonF},
		Instant:    instant,
		Timezone:   loc,
		TimezoneID: tzID,
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
