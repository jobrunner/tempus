// Package aggregate implements the weather-aggregate FeatureProvider: antecedent
// precipitation (24/72/120 h), the fund day's temperature extrema, and
// growing-degree-days accumulated to the instant. It reads a time range from
// the reanalysis archive (output.Reanalysis: ECMWF IFS, ERA5, ERA5-Land,
// elevation-corrected for the point) and computes the aggregates itself.
package aggregate

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const (
	providerID   = "aggregate"
	providerKind = "aggregate"
	gddMethod    = "average (Tmax+Tmin)/2 − base, floor 0, no upper cutoff"

	// precipWindowHours is the longest antecedent-precipitation window.
	precipWindowHours = 120
)

// errFuture is returned when an aggregate query targets a future instant.
var errFuture = errors.New("no aggregate data available for future dates")

// Options configures the provider. Reanalysis supplies the daily and hourly
// series; the per-request gddBase is computed in addition to base 5 and 10.
type Options struct {
	Reanalysis output.Reanalysis
	Clock      output.Clock
}

// Provider is the weather-aggregate provider.
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

// Attribution is the license every aggregate feature carries.
func (p *Provider) Attribution() domain.License {
	l := domain.ReanalysisLicense()
	l.Attribution += "; " + domain.AggregateSource
	return l
}

// Fetch computes the aggregate feature for the request's instant.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	// Aggregates need historical data leading up to the instant; the future has none.
	if req.Instant.After(p.clock.Now().UTC()) {
		return domain.ProviderResult{}, output.NewPermanentError(errFuture)
	}

	pt := req.PointOf()
	start := domain.GDDStartDate(req.Instant, req.Coordinate.Lat)

	// Daily range (GDD + fund-day extrema) from the GDD start to the fund day.
	daily, err := p.reanalysis.Daily(ctx, pt, start, req.Instant.UTC().Truncate(24*time.Hour))
	if err != nil {
		return domain.ProviderResult{}, err
	}

	// Hourly precipitation: the precipWindowHours hours ending with the instant's.
	end := req.Instant.UTC().Truncate(time.Hour).Add(time.Hour)
	hourly, err := p.reanalysis.Hourly(ctx, pt, end.Add(-precipWindowHours*time.Hour), end, domain.VarPrecip)
	if err != nil {
		return domain.ProviderResult{}, err
	}

	props, ok := p.buildProps(req, start, daily, hourly)
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(6 * time.Hour)
	}

	feat := domain.NewPointFeature(req.Coordinate, props, p.Attribution())
	return domain.ProviderResult{Feature: feat}, nil
}

// buildProps assembles the feature properties; ok is false when the archive has
// no usable data for the range yet (retryable).
func (p *Provider) buildProps(req domain.QueryRequest, start time.Time,
	daily domain.DailySeries, hourly domain.HourlySeries,
) (map[string]any, bool) {
	tmin, tmax := pairedDailyTemps(daily)

	props := map[string]any{
		"provider":   providerID,
		"kind":       providerKind,
		"observedAt": req.Instant.UTC().Format(time.RFC3339),
		"units": map[string]string{
			"precipitation": "mm", "temperature": "°C",
			"growingDegreeDays": "°C·d", "arrheniusThermalTime": "d (20 °C-Äquivalent)",
		},
	}

	precip, precipOK := precipWindows(req.Instant, hourly)
	if precipOK {
		props["precipitation"] = precip
	}

	dayOK := false
	if td := dayExtrema(daily, daily.Day(req.Instant)); td != nil {
		props["temperatureDay"] = td
		dayOK = true
	}

	gdd5, days := domain.GrowingDegreeDays(tmin, tmax, domain.GDDBase5Celsius)
	if days > 0 {
		gdd10, _ := domain.GrowingDegreeDays(tmin, tmax, domain.GDDBase10Celsius)
		// Base 5 and 10 are always computed so a client can never introduce
		// inconsistency by silently changing the base over time.
		g := map[string]any{
			"since":  start.Format("2006-01-02"),
			"days":   days,
			"method": gddMethod,
			"base5":  round1(gdd5),
			"base10": round1(gdd10),
		}
		// The per-request gddBase (if any) is computed in addition.
		if req.GDDBaseCelsius != nil {
			gc, _ := domain.GrowingDegreeDays(tmin, tmax, *req.GDDBaseCelsius)
			g["custom"] = map[string]any{"baseCelsius": *req.GDDBaseCelsius, "value": round1(gc)}
		}
		props["growingDegreeDays"] = g

		// Arrhenius thermal time: species-agnostic, base-free site index.
		att, _ := domain.ArrheniusThermalTime(tmin, tmax)
		props["arrheniusThermalTime"] = map[string]any{
			"value":              round1(att),
			"referenceTempC":     domain.ArrheniusReferenceTempC,
			"activationEnergyEv": domain.ArrheniusActivationEnergyEv,
			"days":               days,
			"method":             "Boltzmann-Arrhenius 2-Punkt-Rate (Tmin/Tmax), normiert auf 20 °C (artagnostisch, basisfrei)",
		}
	}

	return props, precipOK || dayOK || days > 0
}

// pairedDailyTemps returns the daily min/max pairs where both values are present.
func pairedDailyTemps(daily domain.DailySeries) (tmin, tmax []float64) {
	n := min(len(daily.TMin), len(daily.TMax))
	for i := 0; i < n; i++ {
		if math.IsNaN(daily.TMin[i]) || math.IsNaN(daily.TMax[i]) {
			continue
		}
		tmin = append(tmin, daily.TMin[i])
		tmax = append(tmax, daily.TMax[i])
	}
	return tmin, tmax
}

// dayExtrema returns the fund day's min/max/mean, or nil when the day is out of
// range or its max/min are missing. Mean falls back to (max+min)/2.
func dayExtrema(daily domain.DailySeries, idx int) map[string]any {
	if idx < 0 || idx >= len(daily.TMax) || idx >= len(daily.TMin) ||
		math.IsNaN(daily.TMax[idx]) || math.IsNaN(daily.TMin[idx]) {
		return nil
	}
	mx, mn := daily.TMax[idx], daily.TMin[idx]
	mean := (mx + mn) / 2
	if idx < len(daily.TMean) && !math.IsNaN(daily.TMean[idx]) {
		mean = daily.TMean[idx]
	}
	return map[string]any{"min": round1(mn), "max": round1(mx), "mean": round1(mean)}
}

// precipWindows sums precipitation over the 24/72/120 h ending at the instant
// (the instant's hour inclusive). ok is false when the instant's hour has no
// value yet.
func precipWindows(instant time.Time, hourly domain.HourlySeries) (map[string]any, bool) {
	vals := hourly.Values[domain.VarPrecip]
	idx := int(instant.UTC().Truncate(time.Hour).Sub(hourly.From) / time.Hour)
	if idx < 0 || idx >= len(vals) || math.IsNaN(vals[idx]) {
		return nil, false
	}
	return map[string]any{
		"last24h":  round1(sumWindow(vals, idx, 24)),
		"last72h":  round1(sumWindow(vals, idx, 72)),
		"last120h": round1(sumWindow(vals, idx, 120)),
	}, true
}

// sumWindow sums the up-to-hours values ending at endIdx (inclusive), skipping NaN.
func sumWindow(vals []float64, endIdx, hours int) float64 {
	start := max(endIdx-hours+1, 0)
	var sum float64
	for i := start; i <= endIdx && i < len(vals); i++ {
		if !math.IsNaN(vals[i]) {
			sum += vals[i]
		}
	}
	return sum
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
