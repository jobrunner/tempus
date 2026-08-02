// Package bioclim implements the bioclim FeatureProvider: the 19 WorldClim
// bioclimatic variables and the Köppen-Geiger class, computed from ERA5
// (Open-Meteo archive) monthly climate normals. The reference period is chosen
// contemporaneously to the query instant (or via the refPeriod override), so
// the values are time-appropriate even for historical records. Results are
// time-independent for a given location+period and are cached per coordinate.
package bioclim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const keyCode = "code"

const (
	providerID   = "bioclim"
	providerKind = "bioclim"
	licenseName  = "CC-BY 4.0"
	licenseURL   = "https://open-meteo.com/en/license"
	cacheVersion = "1"
	cacheTTL     = 365 * 24 * time.Hour
)

// Options configures the provider.
type Options struct {
	ArchiveBaseURL string
	Timeout        time.Duration
	Cache          output.Cache
	HTTPClient     *http.Client
}

// Provider computes bioclimatic variables and the Köppen-Geiger class.
type Provider struct {
	archiveBaseURL string
	cache          output.Cache
	client         *http.Client
}

// New builds the provider.
func New(opts Options) *Provider {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: opts.Timeout}
	}
	return &Provider{archiveBaseURL: opts.ArchiveBaseURL, cache: opts.Cache, client: client}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the base license; Fetch refines the attribution text.
func (p *Provider) Attribution() domain.License {
	return domain.License{Name: licenseName, URL: licenseURL, Attribution: domain.BioclimSource}
}

type dailyResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Daily     struct {
		Time   []string   `json:"time"`
		TMax   []*float64 `json:"temperature_2m_max"`
		TMin   []*float64 `json:"temperature_2m_min"`
		TMean  []*float64 `json:"temperature_2m_mean"`
		Precip []*float64 `json:"precipitation_sum"`
	} `json:"daily"`
}

// Fetch returns the bioclim feature for the request coordinate. The reference
// period is derived from the instant (or req.RefPeriod). Ignores the time of day.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	startY, endY, err := domain.NormalPeriod(req.Instant, req.RefPeriod)
	if err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}

	key := cacheKey(req.Coordinate, startY, endY)
	if p.cache != nil {
		if raw, ok, _ := p.cache.Get(ctx, key); ok {
			var feat domain.Feature
			if json.Unmarshal(raw, &feat) == nil {
				return domain.ProviderResult{Feature: feat, Cached: true}, nil
			}
		}
	}

	u, err := p.buildURL(req.Coordinate, startY, endY)
	if err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}
	var data dailyResponse
	if err := p.getJSON(ctx, u, &data); err != nil {
		return domain.ProviderResult{}, err
	}

	clim, ok := monthlyNormals(data, endY-startY+1)
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(24 * time.Hour)
	}

	feat := p.buildFeature(data, clim, startY, endY)
	if p.cache != nil {
		if raw, err := json.Marshal(feat); err == nil {
			_ = p.cache.Set(ctx, key, raw, cacheTTL)
		}
	}
	return domain.ProviderResult{Feature: feat}, nil
}

func (p *Provider) buildFeature(data dailyResponse, clim domain.MonthlyClimate,
	startY, endY int,
) domain.Feature {
	b := domain.BioclimVariables(clim)
	code, kde, ken := domain.KoppenGeiger(clim, data.Latitude)
	period := fmt.Sprintf("%d-%d", startY, endY)

	koppen := map[string]any{
		keyCode: code, "de": kde, "en": ken,
		// The coldest-month mean is the value that decides the C/D boundary (0 °C);
		// borderline flags that a slightly different dataset (e.g. a station-based
		// Köppen raster vs. ERA5) could change the main class — see the adjacent code.
		"coldestMonthMeanC": r1(domain.ColdestMonthMeanC(clim)),
	}
	if bl, adjCode, adjDe, adjEn := domain.KoppenBorderline(clim, data.Latitude); bl {
		koppen["borderline"] = true
		koppen["adjacent"] = map[string]any{keyCode: adjCode, "de": adjDe, "en": adjEn}
	}

	props := map[string]any{
		"provider":        providerID,
		"kind":            providerKind,
		"referencePeriod": period,
		"bio": map[string]any{
			"bio1": r1(b.Bio1), "bio2": r1(b.Bio2), "bio3": r1(b.Bio3), "bio4": r1(b.Bio4),
			"bio5": r1(b.Bio5), "bio6": r1(b.Bio6), "bio7": r1(b.Bio7), "bio8": r1(b.Bio8),
			"bio9": r1(b.Bio9), "bio10": r1(b.Bio10), "bio11": r1(b.Bio11),
			"bio12": rmm(b.Bio12), "bio13": rmm(b.Bio13), "bio14": rmm(b.Bio14),
			"bio15": r1(b.Bio15), "bio16": rmm(b.Bio16), "bio17": rmm(b.Bio17),
			"bio18": rmm(b.Bio18), "bio19": rmm(b.Bio19),
		},
		"koppen":          koppen,
		"altitudinalBelt": altitudinalBeltProps(clim, data.Latitude),
		"units": map[string]string{
			"temperature": "°C", "precipitation": "mm",
			"bio3": "%", "bio4": "°C×100 (Std.abw.)", "bio15": "% (Variationskoeffizient)",
		},
	}
	// Geometry uses the provider-resolved coordinate (Open-Meteo grid cell),
	// consistent with the other providers.
	resolved := domain.Coordinate{Lat: data.Latitude, Lon: data.Longitude}
	return domain.NewPointFeature(resolved, props, p.license(period))
}

func (p *Provider) buildURL(coord domain.Coordinate, startY, endY int) (string, error) {
	u, err := url.Parse(p.archiveBaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", coord.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", coord.Lon))
	q.Set("timezone", "UTC")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,temperature_2m_mean,precipitation_sum")
	q.Set("start_date", fmt.Sprintf("%d-01-01", startY))
	q.Set("end_date", fmt.Sprintf("%d-12-31", endY))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Provider) getJSON(ctx context.Context, u string, dst any) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return output.NewTransientError(err, 30*time.Second)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return output.NewTransientError(fmt.Errorf("open-meteo status %d", resp.StatusCode), 60*time.Second)
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

func (p *Provider) license(period string) domain.License {
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; ERA5 (Copernicus Climate Change Service / ECMWF), " + period + "; " + domain.BioclimSource,
	}
}

// monthlyNormals aggregates the daily archive range into 12-month climate
// normals. Temperature normals are the mean daily value per calendar month;
// precipitation normals are the mean monthly total (per-month sum ÷ years).
// ok is false when any month has no valid data.
func monthlyNormals(data dailyResponse, years int) (domain.MonthlyClimate, bool) {
	var sumMin, sumMax, sumMean, sumPrecip [12]float64
	var days [12]int

	n := len(data.Daily.Time)
	for i := 0; i < n; i++ {
		m, ok := monthIndex(data.Daily.Time[i])
		if !ok {
			continue
		}
		tmin := valueAt(data.Daily.TMin, i)
		tmax := valueAt(data.Daily.TMax, i)
		tmean := valueAt(data.Daily.TMean, i)
		pr := valueAt(data.Daily.Precip, i)
		// Require all fields (incl. precipitation): counting a day with missing
		// precip would bias the monthly total — and thus BIO12–19 and Köppen —
		// toward drier values.
		if tmin == nil || tmax == nil || tmean == nil || pr == nil {
			continue
		}
		sumMin[m] += *tmin
		sumMax[m] += *tmax
		sumMean[m] += *tmean
		sumPrecip[m] += *pr
		days[m]++
	}

	var clim domain.MonthlyClimate
	for m := 0; m < 12; m++ {
		if days[m] == 0 || years <= 0 {
			return domain.MonthlyClimate{}, false
		}
		d := float64(days[m])
		clim.Tmin[m] = sumMin[m] / d
		clim.Tmax[m] = sumMax[m] / d
		clim.Tmean[m] = sumMean[m] / d
		clim.Precip[m] = sumPrecip[m] / float64(years)
	}
	return clim, true
}

// monthIndex extracts the 0-based month from a "YYYY-MM-DD" date string.
func monthIndex(date string) (int, bool) {
	if len(date) < 7 {
		return 0, false
	}
	m, err := strconv.Atoi(date[5:7])
	if err != nil || m < 1 || m > 12 {
		return 0, false
	}
	return m - 1, true
}

func valueAt(s []*float64, i int) *float64 {
	if i < len(s) {
		return s[i]
	}
	return nil
}

// cacheKey rounds the coordinate to ~10 m (4 decimals). This is far finer than
// ERA5's native grid (~0.1–0.25°), so two coordinates that share a key resolve
// to the same ERA5 cell and thus the same climate — no incorrect collisions —
// while co-located records (same georeference) still share a cache entry.
func cacheKey(coord domain.Coordinate, startY, endY int) string {
	return fmt.Sprintf("%s|%s|%.4f|%.4f|%d-%d", providerID, cacheVersion, coord.Lat, coord.Lon, startY, endY)
}

func r1(v float64) float64  { return math.Round(v*10) / 10 }
func rmm(v float64) float64 { return math.Round(v) }

// altitudinalBeltProps builds the climate-derived altitudinal belt object: the
// classic belt name (source of truth for FHL comparison), the deciding
// indicators, a borderline flag, and the Rivas-Martínez thermotype.
func altitudinalBeltProps(clim domain.MonthlyClimate, latDeg float64) map[string]any {
	belt := domain.AltitudinalBelt(clim)
	tCode, tDe, tEn := domain.RivasMartinezThermotype(clim, latDeg)

	out := map[string]any{
		"belt":       map[string]any{"de": belt.De, "en": belt.En},
		"basis":      "thermoclimatic isotherm, Central-Europe-calibrated",
		"borderline": belt.Borderline,
		"indicators": map[string]any{
			"warmestMonthC":   r1(belt.WarmestMonthC),
			"warmestQuarterC": r1(belt.WarmestQuarterC),
			"matC":            r1(belt.MATC),
			"biotemperatureC": r1(belt.BiotemperatureC),
		},
		"thermotype": map[string]any{keyCode: tCode, "de": tDe, "en": tEn},
		"source":     domain.BeltSource,
	}
	if belt.Borderline {
		out["adjacentBelt"] = map[string]any{"de": belt.AdjDe, "en": belt.AdjEn}
	}
	return out
}
