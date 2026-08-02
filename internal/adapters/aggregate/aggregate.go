// Package aggregate implements the weather-aggregate FeatureProvider: antecedent
// precipitation (24/72/120 h), the fund day's temperature extrema, and
// growing-degree-days accumulated to the instant. It fetches a time range from
// Open-Meteo (unlike the single-hour weather provider) and computes the
// aggregates itself.
package aggregate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const (
	providerID   = "aggregate"
	providerKind = "aggregate"
	licenseName  = "CC-BY 4.0"
	licenseURL   = "https://open-meteo.com/en/license"
	gddMethod    = "average (Tmax+Tmin)/2 − base, floor 0, no upper cutoff"
)

// errFuture is returned when an aggregate query targets a future instant.
var errFuture = errors.New("no aggregate data available for future dates")

// Options configures the provider. The endpoints and delay are shared with the
// weather provider; DefaultGDDBase is the base temperature used when a request
// does not override it via gddBase.
type Options struct {
	ArchiveBaseURL  string
	ForecastBaseURL string
	Timeout         time.Duration
	ArchiveDelay    time.Duration
	DefaultGDDBase  float64
	Clock           output.Clock
	HTTPClient      *http.Client
}

// Provider is the weather-aggregate provider.
type Provider struct {
	archiveBaseURL  string
	forecastBaseURL string
	archiveDelay    time.Duration
	defaultGDDBase  float64
	clock           output.Clock
	client          *http.Client
}

// New builds the provider.
func New(opts Options) *Provider {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}
	base := opts.DefaultGDDBase
	if base == 0 {
		base = domain.DefaultGDDBaseCelsius
	}
	return &Provider{
		archiveBaseURL:  opts.ArchiveBaseURL,
		forecastBaseURL: opts.ForecastBaseURL,
		archiveDelay:    opts.ArchiveDelay,
		defaultGDDBase:  base,
		clock:           opts.Clock,
		client:          client,
	}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the base license; Fetch refines the attribution text.
func (p *Provider) Attribution() domain.License {
	return domain.License{Name: licenseName, URL: licenseURL, Attribution: domain.AggregateSource}
}

type dailyResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Daily     struct {
		Time  []string   `json:"time"`
		TMax  []*float64 `json:"temperature_2m_max"`
		TMin  []*float64 `json:"temperature_2m_min"`
		TMean []*float64 `json:"temperature_2m_mean"`
	} `json:"daily"`
}

type hourlyResponse struct {
	Hourly struct {
		Time   []string   `json:"time"`
		Precip []*float64 `json:"precipitation"`
	} `json:"hourly"`
}

// Fetch computes the aggregate feature for the request's instant.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	// Aggregates need historical data leading up to the instant; the future has none.
	if req.Instant.After(p.clock.Now().UTC()) {
		return domain.ProviderResult{}, output.NewPermanentError(errFuture)
	}

	base := p.defaultGDDBase
	if req.GDDBaseCelsius != nil {
		base = *req.GDDBaseCelsius
	}
	start := domain.GDDStartDate(req.Instant, req.Coordinate.Lat)

	// Daily range (GDD + fund-day extrema) — always from the archive.
	var daily dailyResponse
	if err := p.getJSON(ctx, p.dailyURL(req, start), &daily); err != nil {
		return domain.ProviderResult{}, err
	}

	// Hourly precipitation window (last 5 days) — forecast endpoint for recent dates.
	useArchive := p.clock.Now().UTC().Sub(req.Instant) >= p.archiveDelay
	var hourly hourlyResponse
	if err := p.getJSON(ctx, p.precipURL(req, useArchive), &hourly); err != nil {
		return domain.ProviderResult{}, err
	}

	props, ok := p.buildProps(req, start, base, daily, hourly)
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(6 * time.Hour)
	}

	feat := domain.NewPointFeature(
		domain.Coordinate{Lat: daily.Latitude, Lon: daily.Longitude},
		props,
		p.license(),
	)
	return domain.ProviderResult{Feature: feat}, nil
}

// buildProps assembles the feature properties; ok is false when the archive has
// no usable data for the range yet (retryable).
func (p *Provider) buildProps(req domain.QueryRequest, start time.Time, base float64,
	daily dailyResponse, hourly hourlyResponse,
) (map[string]any, bool) {
	tmin, tmax := pairedDailyTemps(daily)
	gdd, days := domain.GrowingDegreeDays(tmin, tmax, base)

	props := map[string]any{
		"provider":   providerID,
		"kind":       providerKind,
		"observedAt": req.Instant.UTC().Format(time.RFC3339),
		"units": map[string]string{
			"precipitation": "mm", "temperature": "°C", "growingDegreeDays": "°C·d",
		},
	}

	precip, precipOK := precipWindows(req.Instant, hourly)
	if precipOK {
		props["precipitation"] = precip
	}

	dayOK := false
	if idx := indexOf(daily.Daily.Time, req.Instant.UTC().Format("2006-01-02")); idx >= 0 {
		if td := dayExtrema(daily, idx); td != nil {
			props["temperatureDay"] = td
			dayOK = true
		}
	}

	if days > 0 {
		props["growingDegreeDays"] = map[string]any{
			"value":       round1(gdd),
			"baseCelsius": base,
			"since":       start.Format("2006-01-02"),
			"days":        days,
			"method":      gddMethod,
		}
	}

	return props, precipOK || dayOK || days > 0
}

func (p *Provider) dailyURL(req domain.QueryRequest, start time.Time) string {
	u, _ := url.Parse(p.archiveBaseURL)
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", req.Coordinate.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", req.Coordinate.Lon))
	q.Set("timezone", "UTC")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,temperature_2m_mean")
	q.Set("start_date", start.Format("2006-01-02"))
	q.Set("end_date", req.Instant.UTC().Format("2006-01-02"))
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *Provider) precipURL(req domain.QueryRequest, useArchive bool) string {
	base := p.forecastBaseURL
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", req.Coordinate.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", req.Coordinate.Lon))
	q.Set("timezone", "UTC")
	q.Set("hourly", "precipitation")
	if useArchive {
		u, _ = url.Parse(p.archiveBaseURL)
		q = u.Query()
		q.Set("latitude", fmt.Sprintf("%.5f", req.Coordinate.Lat))
		q.Set("longitude", fmt.Sprintf("%.5f", req.Coordinate.Lon))
		q.Set("timezone", "UTC")
		q.Set("hourly", "precipitation")
		q.Set("start_date", req.Instant.Add(-5*24*time.Hour).UTC().Format("2006-01-02"))
		q.Set("end_date", req.Instant.UTC().Format("2006-01-02"))
	} else {
		q.Set("past_days", "7")
		q.Set("forecast_days", "1")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *Provider) getJSON(ctx context.Context, u string, dst any) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return output.NewTransientError(err, 30*time.Second)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return output.NewTransientError(fmt.Errorf("open-meteo status %d", resp.StatusCode), 30*time.Second)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return output.NewPermanentError(fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, b))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return output.NewPermanentError(err)
	}
	return nil
}

func (p *Provider) license() domain.License {
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; ERA5 (Copernicus Climate Change Service / ECMWF); " + domain.AggregateSource,
	}
}

// pairedDailyTemps returns the daily min/max pairs where both values are present.
func pairedDailyTemps(daily dailyResponse) (tmin, tmax []float64) {
	n := len(daily.Daily.TMin)
	if len(daily.Daily.TMax) < n {
		n = len(daily.Daily.TMax)
	}
	for i := 0; i < n; i++ {
		if daily.Daily.TMin[i] == nil || daily.Daily.TMax[i] == nil {
			continue
		}
		tmin = append(tmin, *daily.Daily.TMin[i])
		tmax = append(tmax, *daily.Daily.TMax[i])
	}
	return tmin, tmax
}

// dayExtrema returns the fund day's min/max/mean, or nil when the max/min are
// missing. Mean falls back to (max+min)/2.
func dayExtrema(daily dailyResponse, idx int) map[string]any {
	if idx >= len(daily.Daily.TMax) || idx >= len(daily.Daily.TMin) ||
		daily.Daily.TMax[idx] == nil || daily.Daily.TMin[idx] == nil {
		return nil
	}
	mx, mn := *daily.Daily.TMax[idx], *daily.Daily.TMin[idx]
	mean := (mx + mn) / 2
	if idx < len(daily.Daily.TMean) && daily.Daily.TMean[idx] != nil {
		mean = *daily.Daily.TMean[idx]
	}
	return map[string]any{"min": round1(mn), "max": round1(mx), "mean": round1(mean)}
}

// precipWindows sums precipitation over the 24/72/120 h ending at the instant.
// ok is false when the instant's hour is not present in the response.
func precipWindows(instant time.Time, hourly hourlyResponse) (map[string]any, bool) {
	target := instant.UTC().Format("2006-01-02T15:04")
	idx := indexOf(hourly.Hourly.Time, target)
	if idx < 0 {
		return nil, false
	}
	return map[string]any{
		"last24h":  round1(sumWindow(hourly.Hourly.Precip, idx, 24)),
		"last72h":  round1(sumWindow(hourly.Hourly.Precip, idx, 72)),
		"last120h": round1(sumWindow(hourly.Hourly.Precip, idx, 120)),
	}, true
}

// sumWindow sums the up-to-hours values ending at endIdx (inclusive), skipping nils.
func sumWindow(vals []*float64, endIdx, hours int) float64 {
	start := endIdx - hours + 1
	if start < 0 {
		start = 0
	}
	var sum float64
	for i := start; i <= endIdx && i < len(vals); i++ {
		if vals[i] != nil {
			sum += *vals[i]
		}
	}
	return sum
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
