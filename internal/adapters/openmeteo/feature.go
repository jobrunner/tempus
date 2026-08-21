package openmeteo

import (
	"encoding/json"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// This file maps an Open-Meteo API response onto a weather feature. It is kept
// apart from the provider's transport concerns (openmeteo.go) because the
// mapping is where the domain knowledge sits: which hour to pick, which
// variables are required, and which derived properties to enrich with.

// notYetAvailableRetry is how long a caller should wait when the target hour is
// in the response window but the provider has not filled it in yet.
const notYetAvailableRetry = 2 * time.Hour

// toFeature builds the weather feature for the request's hour.
func (p *Provider) toFeature(data apiResponse, req domain.QueryRequest, useArchive bool) (domain.ProviderResult, error) {
	idx := hourIndex(data, req)
	if idx < 0 {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(notYetAvailableRetry)
	}

	values, units, ok := hourlyValues(data, idx)
	if !ok {
		return domain.ProviderResult{}, output.NewNotYetAvailableError(notYetAvailableRetry)
	}

	props := map[string]any{
		"provider":   providerID,
		"kind":       providerKind,
		"observedAt": req.Instant.UTC().Format(time.RFC3339),
		"units":      units,
		// Day/night comes from the sun's position: Open-Meteo's archive is_day is
		// always 0 for historical dates and therefore unreliable.
		"isDay":       domain.IsDaylight(data.Latitude, data.Longitude, req.Instant),
		"isDaySource": domain.SolarPositionSource,
	}
	for k, v := range values {
		props[k] = v
	}
	enrichWeatherCode(props)
	enrichBeaufort(props, units)

	feat := domain.NewPointFeature(
		domain.Coordinate{Lat: data.Latitude, Lon: data.Longitude},
		props,
		p.license(useArchive),
	)
	return domain.ProviderResult{Feature: feat}, nil
}

// hourIndex locates the request's hour in the response's time series, or -1.
func hourIndex(data apiResponse, req domain.QueryRequest) int {
	var times []string
	if raw, ok := data.Hourly["time"]; ok {
		_ = json.Unmarshal(raw, &times)
	}
	return indexOf(times, req.Instant.UTC().Format("2006-01-02T15:04"))
}

// hourlyValues reads the requested hour out of every hourly variable, returning
// the values keyed by output property name plus their units.
//
// ok is false when the primary variable is present in the response but has no
// value for this hour: the hour exists and the provider has not filled it in
// yet, which the caller reports as not-yet-available rather than as a feature
// with holes in it. A response that omits the primary variable's key altogether
// is a different case and stays ok — it would be a malformed response to a
// request that always asks for it. Both cases are pinned by
// TestHourlyValues_PrimaryVariableHandling.
func hourlyValues(data apiResponse, idx int) (values map[string]any, units map[string]string, ok bool) {
	values, units, ok = map[string]any{}, map[string]string{}, true
	for _, v := range hourlyVars {
		raw, present := data.Hourly[v.api]
		if !present {
			continue
		}
		var vals []*float64
		if json.Unmarshal(raw, &vals) != nil || idx >= len(vals) || vals[idx] == nil {
			if v.api == primaryVar {
				ok = false
			}
			continue
		}
		values[v.prop] = normalize(v.api, *vals[idx])
		if unit, has := data.HourlyUnits[v.api]; has {
			units[v.prop] = unit
		}
	}
	return values, units, ok
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
	speed, isFloat := props["windSpeed10m"].(float64)
	if !isFloat {
		return
	}
	force, de, en, ok := domain.BeaufortFor(speed, units["windSpeed10m"])
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
func normalize(apiName string, v float64) any {
	switch apiName {
	case "weather_code", "relative_humidity_2m", "cloud_cover":
		return int(v)
	default:
		return v
	}
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}
