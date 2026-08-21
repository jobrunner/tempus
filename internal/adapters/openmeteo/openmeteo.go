// Package openmeteo implements the weather FeatureProvider using Open-Meteo.
package openmeteo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// errFuture is returned when a weather query targets a future hour.
var errFuture = errors.New("no weather available for future dates")

const (
	providerID   = "open-meteo"
	providerKind = "weather"
	licenseName  = "CC-BY 4.0"
	licenseURL   = "https://open-meteo.com/en/license"
)

// primaryVar must be present for an hour to count as available: the others may
// legitimately be missing, temperature cannot.
const primaryVar = "temperature_2m"

// propWeatherCode is the output property name for the WMO code. It is named
// because both the variable mapping below and the enrichment in feature.go
// refer to it.
const propWeatherCode = "weatherCode"

// hourlyVars are requested from Open-Meteo, mapped to output property names.
var hourlyVars = []struct{ api, prop string }{
	{"temperature_2m", "temperature2m"},
	{"relative_humidity_2m", "relativeHumidity2m"},
	{"precipitation", "precipitation"},
	{"weather_code", propWeatherCode},
	{"wind_speed_10m", "windSpeed10m"},
	{"cloud_cover", "cloudCover"},
}

// Options configures the provider.
type Options struct {
	ArchiveBaseURL  string
	ForecastBaseURL string
	Timeout         time.Duration
	ArchiveDelay    time.Duration
	Clock           output.Clock
	HTTPClient      *http.Client
}

// Provider is the Open-Meteo weather provider.
type Provider struct {
	archiveBaseURL  string
	forecastBaseURL string
	archiveDelay    time.Duration
	clock           output.Clock
	client          *http.Client
}

// New builds the provider.
func New(opts Options) *Provider {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}
	return &Provider{
		archiveBaseURL:  opts.ArchiveBaseURL,
		forecastBaseURL: opts.ForecastBaseURL,
		archiveDelay:    opts.ArchiveDelay,
		clock:           opts.Clock,
		client:          client,
	}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the base license; Fetch may refine the attribution text.
func (p *Provider) Attribution() domain.License {
	return domain.License{Name: licenseName, URL: licenseURL, Attribution: "Weather data by Open-Meteo.com"}
}

type apiResponse struct {
	Latitude    float64                    `json:"latitude"`
	Longitude   float64                    `json:"longitude"`
	HourlyUnits map[string]string          `json:"hourly_units"`
	Hourly      map[string]json.RawMessage `json:"hourly"`
}

// Fetch retrieves the weather for the request's hour.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	// No weather for the future: this provider only serves past/present hours.
	// Reported as a permanent (non-retryable) provider status in the envelope.
	if req.Instant.After(p.clock.Now().UTC()) {
		return domain.ProviderResult{}, output.NewPermanentError(errFuture)
	}

	useArchive := p.clock.Now().UTC().Sub(req.Instant) >= p.archiveDelay
	endpoint := p.forecastBaseURL
	if useArchive {
		endpoint = p.archiveBaseURL
	}

	u, err := p.buildURL(endpoint, req, useArchive)
	if err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return domain.ProviderResult{}, output.NewTransientError(err, 30*time.Second)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return domain.ProviderResult{}, output.NewTransientError(
			fmt.Errorf("open-meteo status %d", resp.StatusCode), retryAfter(resp))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return domain.ProviderResult{}, output.NewPermanentError(fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, b))
	}

	var data apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}
	return p.toFeature(data, req, useArchive)
}

func (p *Provider) buildURL(base string, req domain.QueryRequest, useArchive bool) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", req.Coordinate.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", req.Coordinate.Lon))
	q.Set("timezone", "UTC")
	names := make([]string, len(hourlyVars))
	for i, v := range hourlyVars {
		names[i] = v.api
	}
	q.Set("hourly", strings.Join(names, ","))
	day := req.Instant.UTC().Format("2006-01-02")
	if useArchive {
		q.Set("start_date", day)
		q.Set("end_date", day)
	} else {
		// forecast endpoint: pull recent past so the target hour is covered.
		q.Set("past_days", "7")
		q.Set("forecast_days", "1")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Provider) license(useArchive bool) domain.License {
	src := "GFS/ICON forecast models"
	if useArchive {
		src = "ERA5 (Copernicus Climate Change Service / ECMWF)"
	}
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; " + src,
	}
}

func retryAfter(resp *http.Response) time.Duration {
	if s := resp.Header.Get("Retry-After"); s != "" {
		if secs, err := time.ParseDuration(s + "s"); err == nil {
			return secs
		}
	}
	return 30 * time.Second
}
