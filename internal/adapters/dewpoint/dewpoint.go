// Package dewpoint derives the dew point from weather features using the
// Magnus-Tetens formula (Sonntag-1990 coefficients).
package dewpoint

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const deriverID = "dewpoint"

// Deriver implements output.FeatureDeriver for the dew-point calculation.
type Deriver struct{}

// New returns a new Deriver.
func New() *Deriver { return &Deriver{} }

// ID satisfies output.FeatureDeriver.
func (d *Deriver) ID() string { return deriverID }

// Kind satisfies output.FeatureDeriver.
func (d *Deriver) Kind() string { return deriverID }

// Attribution satisfies output.FeatureDeriver.
func (d *Deriver) Attribution() domain.License {
	return domain.License{
		Name:        "Magnus-Formel",
		URL:         "https://en.wikipedia.org/wiki/Dew_point",
		Attribution: "Taupunkt berechnet von tempus (Magnus-Tetens)",
	}
}

// Derive computes the dew point from a weather source feature.
// It returns output.NewNotYetAvailableError when no weather source is present
// (the weather provider may not have run yet), or output.NewPermanentError when
// the source feature exists but is missing required fields.
func (d *Deriver) Derive(_ context.Context, _ domain.QueryRequest, sources []domain.Feature) ([]domain.Feature, error) {
	src, found := findWeatherSource(sources)
	if !found {
		return nil, output.NewNotYetAvailableError(2 * time.Hour)
	}

	tempC, ok := coerceFloat(src.Properties["temperature2m"])
	if !ok {
		return nil, output.NewPermanentError(errors.New("cannot derive dew point: missing/invalid temperature or humidity"))
	}

	rhRaw, ok := coerceFloat(src.Properties["relativeHumidity2m"])
	if !ok {
		return nil, output.NewPermanentError(errors.New("cannot derive dew point: missing/invalid temperature or humidity"))
	}

	td, ok := domain.DewPointCelsius(tempC, rhRaw)
	if !ok {
		return nil, output.NewPermanentError(errors.New("cannot derive dew point: missing/invalid temperature or humidity"))
	}

	comfortDE, comfortEN := domain.DewPointComfort(td)

	feat := domain.Feature{
		Type:     "Feature",
		Geometry: src.Geometry,
		Properties: map[string]any{
			"provider":      deriverID,
			"kind":          deriverID,
			"observedAt":    src.Properties["observedAt"],
			"dewPoint2m":    math.Round(td*10) / 10,
			"units":         map[string]string{"dewPoint2m": "°C"},
			"basedOn":       src.Properties["provider"],
			"method":        "Magnus-Tetens (a=17.62, b=243.12)",
			"comfort":       map[string]string{"de": comfortDE, "en": comfortEN},
			"comfortSource": domain.DewPointComfortSource,
		},
		License: domain.License{
			Name:        src.License.Name,
			URL:         src.License.URL,
			Attribution: "Taupunkt (Magnus-Formel), berechnet von tempus aus: " + src.License.Attribution,
		},
	}
	return []domain.Feature{feat}, nil
}

// findWeatherSource returns the first feature that is a weather source: either
// its kind property equals "weather", or it carries both temperature2m and
// relativeHumidity2m.
func findWeatherSource(sources []domain.Feature) (domain.Feature, bool) {
	for _, f := range sources {
		if kind, ok := f.Properties["kind"]; ok && kind == "weather" {
			return f, true
		}
		_, hasTemp := f.Properties["temperature2m"]
		_, hasRH := f.Properties["relativeHumidity2m"]
		if hasTemp && hasRH {
			return f, true
		}
	}
	return domain.Feature{}, false
}

// coerceFloat extracts a float64 from values that may be float64, int, int64,
// or json.Number (the last two arise from cache JSON round-trips).
func coerceFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
