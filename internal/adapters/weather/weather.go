// Package weather implements the weather FeatureProvider. It reads
// Open-Meteo's open-data reanalysis archive (ECMWF IFS, ERA5, ERA5-Land)
// through the output.Reanalysis port rather than calling the Open-Meteo API,
// and presents the hour the way the API used to: same property names, units
// and display rounding.
package weather

import (
	"context"
	"errors"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// errFuture is returned when a weather query targets a future hour.
var errFuture = errors.New("no weather available for future dates")

const (
	providerID   = "open-meteo"
	providerKind = "weather"
)

// Output property names, shared by the variable mapping below, the
// enrichments in feature.go and the tests.
const (
	propTemperature = "temperature2m"
	propHumidity    = "relativeHumidity2m"
	propPrecip      = "precipitation"
	propWeatherCode = "weatherCode"
	propWindSpeed   = "windSpeed10m"
	propCloudCover  = "cloudCover"
	propElevation   = "elevation"
)

// Keys of a modelCells entry (plus propElevation for the cell's height).
const (
	cellModel = "model"
	cellLat   = "lat"
	cellLon   = "lon"
)

// unitKmh is the wind-speed unit Open-Meteo reported and the Beaufort lookup
// expects.
const unitKmh = "km/h"

// hourlyVars are read from the archive, mapped to output property names, the
// unit the Open-Meteo API reported for them, and the number of decimals its
// JSON output kept.
var hourlyVars = []struct {
	name, prop, unit string
	decimals         int
}{
	{domain.VarTemperature, propTemperature, "°C", 1},
	{domain.VarHumidity, propHumidity, "%", 0},
	{domain.VarPrecip, propPrecip, "mm", 1},
	{domain.VarWeatherCode, propWeatherCode, "wmo code", 0},
	{domain.VarWind, propWindSpeed, unitKmh, 1},
	{domain.VarCloud, propCloudCover, "%", 0},
}

// Options configures the provider.
type Options struct {
	Reanalysis output.Reanalysis
	Clock      output.Clock
}

// Provider is the weather provider backed by the reanalysis archive.
type Provider struct {
	reanalysis output.Reanalysis
	clock      output.Clock
}

// New builds the provider.
func New(opts Options) *Provider {
	return &Provider{reanalysis: opts.Reanalysis, clock: opts.Clock}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the license every weather feature carries.
func (p *Provider) Attribution() domain.License {
	return domain.ReanalysisLicense()
}

// Fetch retrieves the weather for the request's hour.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	// No weather for the future: this provider only serves past/present hours.
	// Reported as a permanent (non-retryable) provider status in the envelope.
	if req.Instant.After(p.clock.Now().UTC()) {
		return domain.ProviderResult{}, output.NewPermanentError(errFuture)
	}

	// Read the whole UTC day, as the Open-Meteo archive request for that date
	// did: the model selection and elevation correction see the same window,
	// so the hour comes out identical.
	day := req.Instant.UTC().Truncate(24 * time.Hour)
	names := make([]string, len(hourlyVars))
	for i, v := range hourlyVars {
		names[i] = v.name
	}
	series, err := p.reanalysis.Hourly(ctx, req.PointOf(), day, day.Add(24*time.Hour), names...)
	if err != nil {
		// The port already reports ProviderErrors.
		return domain.ProviderResult{}, err
	}
	return toFeature(series, req)
}
