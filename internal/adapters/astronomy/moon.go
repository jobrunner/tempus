package astronomy

import (
	"context"
	"math"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

const (
	moonProviderID   = "moon"
	moonProviderKind = "moon"
)

// MoonProvider computes the lunar position, rise/set and phase for the request.
type MoonProvider struct{}

// NewMoon returns a MoonProvider.
func NewMoon() *MoonProvider { return &MoonProvider{} }

// ID satisfies output.FeatureProvider.
func (p *MoonProvider) ID() string { return moonProviderID }

// Kind satisfies output.FeatureProvider.
func (p *MoonProvider) Kind() string { return moonProviderKind }

// Attribution satisfies output.FeatureProvider.
func (p *MoonProvider) Attribution() domain.License {
	return domain.License{
		Name:        "Meeus, Astronomical Algorithms",
		URL:         "https://en.wikipedia.org/wiki/Position_of_the_Moon",
		Attribution: domain.MoonSource,
	}
}

// Fetch computes the moon feature. It never fails or contacts the network.
func (p *MoonProvider) Fetch(_ context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	lat, lon := req.Coordinate.Lat, req.Coordinate.Lon
	elev, az, distKm := domain.MoonPosition(lat, lon, req.Instant)
	illum, phaseAngle, age, de, en := domain.MoonPhase(req.Instant)
	rise, set := domain.MoonTimes(lat, lon, req.Instant)

	props := map[string]any{
		"provider":      moonProviderID,
		"kind":          moonProviderKind,
		"observedAt":    req.Instant.UTC().Format(time.RFC3339),
		keyElevationDeg: round2(elev),
		keyAzimuthDeg:   round2(az),
		keyDistanceKm:   math.Round(distKm),
		"moonrise":      fmtTime(rise),
		"moonset":       fmtTime(set),
		keyIllumination: illum,
		keyPhaseAngle:   phaseAngle,
		keyAgeDays:      age,
		"phase":         map[string]string{"de": de, "en": en},
		"units": map[string]string{
			keyElevationDeg: "°", keyAzimuthDeg: "° (von Nord, im Uhrzeigersinn)",
			keyDistanceKm: "km", keyIllumination: "%", keyPhaseAngle: "°", keyAgeDays: "d",
		},
	}

	feat := domain.NewPointFeature(domain.Coordinate{Lat: lat, Lon: lon}, props, p.Attribution())
	return domain.ProviderResult{Feature: feat}, nil
}
