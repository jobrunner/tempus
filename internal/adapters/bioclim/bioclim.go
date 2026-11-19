// Package bioclim implements the bioclim FeatureProvider: the 19 WorldClim
// bioclimatic variables and the Köppen-Geiger class, computed from monthly
// climate normals of the reanalysis archive (output.Reanalysis: ECMWF IFS,
// ERA5, ERA5-Land, corrected for the request's elevation). The period is chosen
// contemporaneously to the query instant (or via the refPeriod override), so
// the values are time-appropriate even for historical records. Results are
// time-independent for a given location+period and are cached per coordinate
// and elevation.
package bioclim

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

const keyCode = "code"

const (
	providerID   = "bioclim"
	providerKind = "bioclim"
	// cacheVersion is part of every cache key. Bump it whenever the derived
	// classification changes, or cached features keep the old result for up to
	// cacheTTL. "2": Köppen-Geiger aligned with Beck et al. 2018. "3":
	// Rivas-Martínez macrobioclimate and thermotype per the WBCS key, with the
	// archive's elevation (new altitudinalBelt.macrobioclimate object). "4":
	// Köppen thresholds on computed values compared beyond float roundoff.
	// "5": normals from the reanalysis port (unrounded, client elevation).
	cacheVersion = "5"
	cacheTTL     = 365 * 24 * time.Hour
)

// Options configures the provider.
type Options struct {
	Reanalysis output.Reanalysis
	Cache      output.Cache
}

// Provider computes bioclimatic variables and the Köppen-Geiger class.
type Provider struct {
	reanalysis output.Reanalysis
	cache      output.Cache
}

// New builds the provider.
func New(opts Options) *Provider {
	return &Provider{reanalysis: opts.Reanalysis, cache: opts.Cache}
}

func (p *Provider) ID() string   { return providerID }
func (p *Provider) Kind() string { return providerKind }

// Attribution is the base license; Fetch refines the attribution text.
func (p *Provider) Attribution() domain.License {
	l := domain.ReanalysisLicense()
	l.Attribution = domain.BioclimSource
	return l
}

// Fetch returns the bioclim feature for the request coordinate. The reference
// period is derived from the instant (or req.RefPeriod). Ignores the time of day.
func (p *Provider) Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error) {
	startY, endY, err := domain.NormalPeriod(req.Instant, req.RefPeriod)
	if err != nil {
		return domain.ProviderResult{}, output.NewPermanentError(err)
	}

	key := cacheKey(req.PointOf(), startY, endY)
	if feat, ok := p.cachedFeature(ctx, key); ok {
		return domain.ProviderResult{Feature: feat, Cached: true}, nil
	}

	first := time.Date(startY, time.January, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(endY, time.December, 31, 0, 0, 0, 0, time.UTC)
	data, err := p.reanalysis.Daily(ctx, req.PointOf(), first, last)
	if err != nil {
		return domain.ProviderResult{}, err
	}

	clim, ok := monthlyNormals(data, endY-startY+1)
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(24 * time.Hour)
	}

	feat := p.buildFeature(req, clim, startY, endY)
	if lerr := feat.License.Validate(); lerr != nil {
		return domain.ProviderResult{}, output.NewPermanentError(lerr)
	}
	p.cacheFeature(ctx, key, feat)
	return domain.ProviderResult{Feature: feat}, nil
}

func (p *Provider) buildFeature(req domain.QueryRequest, clim domain.MonthlyClimate,
	startY, endY int,
) domain.Feature {
	b := domain.BioclimVariables(clim)
	code, kde, ken := domain.KoppenGeiger(clim, req.Coordinate.Lat)
	period := fmt.Sprintf("%d-%d", startY, endY)

	koppen := map[string]any{
		keyCode: code, "de": kde, "en": ken,
		// The coldest-month mean is the value that decides the C/D boundary (0 °C);
		// borderline flags that a slightly different dataset (e.g. a station-based
		// Köppen raster vs. ERA5) could change the main class — see the adjacent code.
		"coldestMonthMeanC": r1(domain.ColdestMonthMeanC(clim)),
	}
	if bl, adjCode, adjDe, adjEn := domain.KoppenBorderline(clim, req.Coordinate.Lat); bl {
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
		"altitudinalBelt": altitudinalBeltProps(clim, rmSite(req)),
		"units": map[string]string{
			"temperature": "°C", "precipitation": "mm",
			"bio3": "%", "bio4": "°C×100 (Std.abw.)", "bio15": "% (Variationskoeffizient)",
		},
	}
	return domain.NewPointFeature(req.Coordinate, props, license(period))
}

// license is the reanalysis license with the reference period and the bioclim
// method source appended to the attribution.
func license(period string) domain.License {
	l := domain.ReanalysisLicense()
	l.Attribution += "; " + period + "; " + domain.BioclimSource
	return l
}

// monthlyNormals aggregates the daily archive range into 12-month climate
// normals. Temperature normals are the mean daily value per calendar month;
// precipitation normals are the mean monthly total (per-month sum ÷ years).
// ok is false when any month has no valid data.
func monthlyNormals(data domain.DailySeries, years int) (domain.MonthlyClimate, bool) {
	var sumMin, sumMax, sumMean, sumPrecip [12]float64
	var days [12]int

	n := min(len(data.TMax), len(data.TMin), len(data.TMean), len(data.PrecipSum))
	for i := 0; i < n; i++ {
		m := int(data.FirstDay.AddDate(0, 0, i).Month()) - 1
		tmin, tmax, tmean, pr := data.TMin[i], data.TMax[i], data.TMean[i], data.PrecipSum[i]
		// Require all fields (incl. precipitation): counting a day with missing
		// precip would bias the monthly total — and thus BIO12–19 and Köppen —
		// toward drier values.
		if math.IsNaN(tmin) || math.IsNaN(tmax) || math.IsNaN(tmean) || math.IsNaN(pr) {
			continue
		}
		sumMin[m] += tmin
		sumMax[m] += tmax
		sumMean[m] += tmean
		sumPrecip[m] += pr
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

func r1(v float64) float64  { return math.Round(v*10) / 10 }
func rmm(v float64) float64 { return math.Round(v) }

// altitudinalBeltProps builds the climate-derived altitudinal belt object: the
// classic belt name (source of truth for FHL comparison), the deciding
// indicators, a borderline flag, and the Rivas-Martínez macrobioclimate and
// thermotype.
func altitudinalBeltProps(clim domain.MonthlyClimate, site domain.RivasMartinezSite) map[string]any {
	belt := domain.AltitudinalBelt(clim)
	thermotype, macro := rivasMartinezProps(clim, site)

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
		"thermotype":      thermotype,
		"macrobioclimate": macro,
		"source":          domain.BeltSource,
	}
	if belt.Borderline {
		out["adjacentBelt"] = map[string]any{"de": belt.AdjDe, "en": belt.AdjEn}
	}
	return out
}
