package openmeteo

import (
	"math"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/domain/reanalysis"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// This file maps a reanalysis series onto a weather feature. It is kept apart
// from the provider's port wiring (openmeteo.go) because the mapping is where
// the domain knowledge sits: which hour to pick, which variables are required,
// how values are rounded for display, and which derived properties to enrich
// with.

// notYetAvailableRetry is how long a caller should wait when the target hour is
// inside the archive's window but has no temperature yet.
const notYetAvailableRetry = 2 * time.Hour

// toFeature builds the weather feature for the request's hour.
func toFeature(series domain.HourlySeries, req domain.QueryRequest) (domain.ProviderResult, error) {
	values, units, ok := hourlyValues(series, req.Instant.UTC())
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(notYetAvailableRetry)
	}

	props := map[string]any{
		"provider":    providerID,
		"kind":        providerKind,
		"observedAt":  req.Instant.UTC().Format(time.RFC3339),
		"units":       units,
		propElevation: req.Elevation,
		"modelCells":  modelCells(series.Cells),
		// Day/night comes from the sun's position: the reanalysis has no
		// reliable is_day for historical dates.
		"isDay":       domain.IsDaylight(req.Coordinate.Lat, req.Coordinate.Lon, req.Instant),
		"isDaySource": domain.SolarPositionSource,
	}
	for k, v := range values {
		props[k] = v
	}
	enrichWeatherCode(props)
	enrichBeaufort(props, units)

	feat := domain.NewPointFeature(req.Coordinate, props, domain.ReanalysisLicense())
	return domain.ProviderResult{Feature: feat}, nil
}

// hourlyValues reads hour t out of every variable, rounded for display and
// keyed by output property name, plus their units.
//
// ok is false when the temperature has no value for the hour: the archive has
// not filled it in yet, which the caller reports as not-yet-available rather
// than as a feature with holes in it. The other variables may legitimately be
// missing and are then left out.
func hourlyValues(series domain.HourlySeries, t time.Time) (values map[string]any, units map[string]string, ok bool) {
	if _, ok = series.At(domain.VarTemperature, t); !ok {
		return nil, nil, false
	}
	values, units = map[string]any{}, map[string]string{}
	for _, v := range hourlyVars {
		x, present := series.At(v.name, t)
		if !present {
			continue
		}
		values[v.prop] = normalize(v.name, displayRound(x, v.decimals))
		units[v.prop] = v.unit
	}
	return values, units, true
}

// displayRound rounds x exactly as Open-Meteo's JSON output did — in float32
// (0.05 becomes 0.1, -0.25 becomes -0.3) — and returns the nearest float64 to
// that decimal, so the JSON shows 0.1 rather than float32 noise.
func displayRound(x float64, decimals int) float64 {
	return roundTo(float64(reanalysis.Round(float32(x), decimals)), decimals)
}

// modelCells lists the grid points the series was read from, so a reader can
// see which model cells (and heights) stand behind the values.
func modelCells(cells []domain.ModelCell) []map[string]any {
	out := make([]map[string]any, len(cells))
	for i, c := range cells {
		out[i] = map[string]any{
			cellModel:     c.Model,
			cellLat:       roundTo(c.Lat, 5),
			cellLon:       roundTo(c.Lon, 5),
			propElevation: roundTo(c.Elevation, 1),
		}
	}
	return out
}

func roundTo(x float64, decimals int) float64 {
	scale := math.Pow10(decimals)
	return math.Round(x*scale) / scale
}

// enrichWeatherCode adds the bilingual WMO-4677 description for the weather
// code, when the WMO table has an entry for that code.
func enrichWeatherCode(props map[string]any) {
	raw, present := props[propWeatherCode]
	if !present {
		return
	}
	var code int
	switch wc := raw.(type) {
	case int:
		code = wc
	case float64:
		code = int(wc)
	default:
		return
	}
	de, en, ok := domain.WeatherCodeDescription(code)
	if !ok {
		return
	}
	props["weatherCodeDescription"] = map[string]string{"de": de, "en": en}
	props["weatherCodeSource"] = domain.WMOCodeSource
	props["weatherCodeSourceURL"] = domain.WMOCodeSourceURL
}

// enrichBeaufort adds the Beaufort wind force, converted from whatever unit the
// provider reported for the wind speed. A speed without a recognised unit is
// left unclassified rather than misclassified.
func enrichBeaufort(props map[string]any, units map[string]string) {
	speed, isFloat := props[propWindSpeed].(float64)
	if !isFloat {
		return
	}
	force, de, en, ok := domain.BeaufortFor(speed, units[propWindSpeed])
	if !ok {
		return
	}
	props["windBeaufort"] = force
	props["windBeaufortDescription"] = map[string]string{"de": de, "en": en}
	props["windBeaufortSource"] = domain.BeaufortSource
	props["windBeaufortSourceURL"] = domain.BeaufortSourceURL
}

// normalize converts weather_code, relative_humidity_2m and cloud_cover to
// int; leaves other numeric values as float64.
func normalize(name string, v float64) any {
	switch name {
	case domain.VarWeatherCode, domain.VarHumidity, domain.VarCloud:
		return int(v)
	default:
		return v
	}
}
