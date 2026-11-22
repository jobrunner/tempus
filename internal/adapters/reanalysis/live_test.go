package reanalysis

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

// TestLive_ReanalysisMatchesOpenMeteoArchive replays the curated phase-0
// points (coast, date line, Greenwich, mountains, the southern hemisphere)
// against the Open-Meteo archive API, hourly and daily. Opt-in, it calls both
// services: TEMPUS_OM_LIVE=1. The seeded West-Palearctic sweep is
// TestConformance_WestPalearctic.
func TestLive_ReanalysisMatchesOpenMeteoArchive(t *testing.T) {
	skipUnlessLive(t)
	var points []struct {
		Lab       string  `json:"lab"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		Elevation float64 `json:"elevation"`
	}
	raw, err := os.ReadFile("testdata/conformance_points.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &points); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	svc := liveService()
	today := time.Now().UTC().Truncate(24 * time.Hour)
	windows := [][2]time.Time{
		{date(2005, 1, 10), date(2005, 1, 12)},
		{date(2016, 12, 30), date(2017, 1, 2)},
		{date(2021, 7, 14), date(2021, 7, 16)},
		{today.AddDate(0, 0, -4), today.AddDate(0, 0, -2)},
	}
	hourlyVars := map[string]int{
		domain.VarTemperature: 1, domain.VarDewPoint: 1, domain.VarHumidity: 0, domain.VarPrecip: 1,
		domain.VarSnowfall: 2, domain.VarWeatherCode: 0, domain.VarWind: 1, domain.VarCloud: 0,
	}
	names := make([]string, 0, len(hourlyVars))
	for v := range hourlyVars {
		names = append(names, v)
	}
	tally := newTally(t)
	for _, p := range points {
		gp := domain.GeoPoint{Lat: p.Lat, Lon: p.Lon, Elevation: p.Elevation}
		for _, w := range windows {
			window := w[0].Format(dayLayout)
			label := p.Lab + " " + window
			got, err := svc.Hourly(context.Background(), gp, w[0], w[1].Add(24*time.Hour), names...)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			api := archive(t, client, gp, w[0], w[1], kindHourly, names)
			for v, dec := range hourlyVars {
				tally.compare(label, window, v, got.Values[v], api[v], dec)
			}
			days, err := svc.Daily(context.Background(), gp, w[0], w[1])
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			daily := map[string][]float64{
				"temperature_2m_max": days.TMax, "temperature_2m_min": days.TMin,
				"temperature_2m_mean": days.TMean, "precipitation_sum": days.PrecipSum,
			}
			dailyNames := make([]string, 0, len(daily))
			for k := range daily {
				dailyNames = append(dailyNames, k)
			}
			api = archive(t, client, gp, w[0], w[1], kindDaily, dailyNames)
			for name, series := range daily {
				tally.compare(label, window, name, series, api[name], 1)
			}
		}
	}
	tally.report(0.999)
}
