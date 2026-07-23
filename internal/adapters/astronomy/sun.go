// Package astronomy provides pure-computation FeatureProviders for the sun and
// moon: position, rise/set, twilight and lunar phase. They fetch nothing
// external, need no cache and work for any date (past or future).
package astronomy

import (
	"context"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

const (
	sunProviderID   = "sun"
	sunProviderKind = "sun"
)

// SunProvider computes the solar position and events for the request.
type SunProvider struct{}

// NewSun returns a SunProvider.
func NewSun() *SunProvider { return &SunProvider{} }

// ID satisfies output.FeatureProvider.
func (p *SunProvider) ID() string { return sunProviderID }

// Kind satisfies output.FeatureProvider.
func (p *SunProvider) Kind() string { return sunProviderKind }

// Attribution satisfies output.FeatureProvider.
func (p *SunProvider) Attribution() domain.License {
	return domain.License{
		Name:        "NOAA Solar Calculator",
		URL:         "https://gml.noaa.gov/grad/solcalc/",
		Attribution: domain.SunSource,
	}
}

// Fetch computes the sun feature. It never fails or contacts the network.
func (p *SunProvider) Fetch(_ context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	lat, lon := req.Coordinate.Lat, req.Coordinate.Lon
	elev, az := domain.SunAltAz(lat, lon, req.Instant)
	de, en := domain.SunLightPhase(elev)
	events := domain.SolarEvents(lat, lon, req.Instant)

	props := map[string]any{
		"provider":      sunProviderID,
		"kind":          sunProviderKind,
		"observedAt":    req.Instant.UTC().Format(time.RFC3339),
		keyElevationDeg: round2(elev),
		keyAzimuthDeg:   round2(az),
		keyZenithDeg:    round2(90 - elev),
		"lightPhase":    map[string]string{"de": de, "en": en},
		"sunrise":       fmtTime(events.Sunrise),
		"sunset":        fmtTime(events.Sunset),
		"solarNoon":     fmtTime(events.SolarNoon),
		"twilight": map[string]any{
			"civil":        map[string]any{keyDawn: fmtTime(events.CivilDawn), keyDusk: fmtTime(events.CivilDusk)},
			"nautical":     map[string]any{keyDawn: fmtTime(events.NauticalDawn), keyDusk: fmtTime(events.NauticalDusk)},
			"astronomical": map[string]any{keyDawn: fmtTime(events.AstronomicalDawn), keyDusk: fmtTime(events.AstronomicalDusk)},
		},
		"units": map[string]string{
			keyElevationDeg: "°", keyAzimuthDeg: "° (von Nord, im Uhrzeigersinn)", keyZenithDeg: "°",
			"solarNoonElevationDeg": "°", "dayLengthMinutes": "min",
		},
	}
	if events.SolarNoon != nil {
		props["solarNoonElevationDeg"] = events.SolarNoonElevationDeg
	}
	if events.DayLengthMinutes > 0 {
		props["dayLengthMinutes"] = events.DayLengthMinutes
	}

	feat := domain.NewPointFeature(domain.Coordinate{Lat: lat, Lon: lon}, props, p.Attribution())
	return domain.ProviderResult{Feature: feat}, nil
}
