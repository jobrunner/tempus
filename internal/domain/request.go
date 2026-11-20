package domain

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// QueryRequest is a validated feature-query request. Instant is UTC, truncated
// to the hour. Providers is an optional filter (empty ⇒ all registered).
// GDDBaseCelsius optionally overrides the growing-degree-day base temperature
// used by the aggregate provider (nil ⇒ provider default).
type QueryRequest struct {
	Coordinate Coordinate
	// Elevation is the height of the location in metres above sea level
	// (orthometric, as from a DEM or a GNSS receiver's MSL output — not the
	// ellipsoidal height). Required: the reanalysis cell selection and the
	// lapse-rate correction depend on it.
	Elevation      float64
	Instant        time.Time
	Providers      []string
	GDDBaseCelsius *float64
	// RefPeriod optionally overrides the bioclim reference period ("YYYY-YYYY";
	// empty ⇒ auto-selected contemporaneous normal).
	RefPeriod string
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
	fieldDatetime  = "datetime"
	fieldGDDBase   = "gddBase"
	fieldElevation = "elevation"

	// Elevation bounds in metres: below the Dead Sea shore, above Everest.
	minElevation = -500
	maxElevation = 9000
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
func ParseQueryRequest(lat, lon, elevation, datetime, gddBase string, providers []string) (QueryRequest, error) {
	latF, err := strconv.ParseFloat(lat, 64)
	if err != nil || latF < -90 || latF > 90 {
		return QueryRequest{}, ValidationError{"lat", "must be a number in [-90,90]"}
	}
	lonF, err := strconv.ParseFloat(lon, 64)
	if err != nil || lonF < -180 || lonF > 180 {
		return QueryRequest{}, ValidationError{"lon", "must be a number in [-180,180]"}
	}

	elev, err := parseElevation(elevation)
	if err != nil {
		return QueryRequest{}, err
	}

	instant, ok := parseInstant(datetime)
	if !ok {
		return QueryRequest{}, ValidationError{fieldDatetime, "must be RFC3339 or YYYY-MM-DDTHH:MM[:SS] (UTC assumed)"}
	}
	instant = instant.UTC().Truncate(time.Hour)

	gdd, err := parseGDDBase(gddBase)
	if err != nil {
		return QueryRequest{}, err
	}

	return QueryRequest{
		Coordinate:     Coordinate{Lat: latF, Lon: lonF},
		Elevation:      elev,
		Instant:        instant,
		Providers:      providers,
		GDDBaseCelsius: gdd,
	}, nil
}

// parseElevation parses the required elevation in metres above sea level.
func parseElevation(s string) (float64, error) {
	elev, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(elev) || elev < minElevation || elev > maxElevation {
		return 0, ValidationError{fieldElevation, "is required: metres above sea level in [-500,9000]"}
	}
	return elev, nil
}

// parseGDDBase parses the optional gddBase override. Empty ⇒ nil (use default).
func parseGDDBase(s string) (*float64, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < -50 || v > 50 {
		return nil, ValidationError{fieldGDDBase, "must be a number in [-50,50] °C"}
	}
	return &v, nil
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
