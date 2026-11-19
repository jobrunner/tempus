package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/omstore"
	"github.com/jobrunner/tempus/internal/adapters/reanalysis"
	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
)

// TestLive_ReanalysisMatchesOpenMeteoArchive is the end-to-end conformance
// check: tempus reads the open-data bucket and must reproduce the Open-Meteo
// archive API (best_match, same elevation) after the API's own rounding.
// Opt-in, it calls both services: TEMPUS_OM_LIVE=1.
func TestLive_ReanalysisMatchesOpenMeteoArchive(t *testing.T) {
	if os.Getenv("TEMPUS_OM_LIVE") != "1" {
		t.Skip("set TEMPUS_OM_LIVE=1 to compare against the live Open-Meteo API")
	}
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
	svc := reanalysis.NewService(omstore.New(omstore.Options{BaseURL: "https://openmeteo.s3.amazonaws.com", HTTPClient: client, MaxParallel: 32}))
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
	tally := conformance{t: t, stats: map[string][2]int{}}
	for _, p := range points {
		gp := domain.GeoPoint{Lat: p.Lat, Lon: p.Lon, Elevation: p.Elevation}
		for _, w := range windows {
			label := p.Lab + " " + w[0].Format("2006-01-02")
			got, err := svc.Hourly(context.Background(), gp, w[0], w[1].Add(24*time.Hour), names...)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			api := archive(t, client, gp, w[0], w[1], "hourly", names)
			for v, dec := range hourlyVars {
				tally.compare(label, v, got.Values[v], api[v], dec)
			}
			days, err := svc.Daily(context.Background(), gp, w[0], w[1])
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			daily := map[string][]float64{
				"temperature_2m_max": days.TMax, "temperature_2m_min": days.TMin,
				"temperature_2m_mean": days.TMean, "precipitation_sum": days.PrecipSum,
			}
			api = archive(t, client, gp, w[0], w[1], "daily", keys(daily))
			for name, series := range daily {
				tally.compare(label, name, series, api[name], 1)
			}
		}
	}
	tally.report()
}

// conformance counts equal values per variable and logs the first misses.
type conformance struct {
	t     *testing.T
	stats map[string][2]int
}

func (c conformance) compare(label, variable string, mine []float64, api []*float64, decimals int) {
	for i, a := range api {
		s := c.stats[variable]
		s[1]++
		v := ra.Round(float32(mine[i]), decimals)
		if same(a, v) {
			s[0]++
		} else if s[1]-s[0] <= 3 {
			c.t.Logf("%s %s #%d: tempus %v api %v", label, variable, i, v, deref(a))
		}
		c.stats[variable] = s
	}
}

func (c conformance) report() {
	for v, s := range c.stats {
		c.t.Logf("%-22s %d/%d equal", v, s[0], s[1])
		if float64(s[0]) < 0.999*float64(s[1]) {
			c.t.Errorf("%s: only %d of %d equal", v, s[0], s[1])
		}
	}
}

func keys(m map[string][]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// same compares in float32: the API prints the float32 result of its own
// rounding, which parses back to the same float32.
func same(api *float64, mine float32) bool {
	if api == nil {
		return math.IsNaN(float64(mine))
	}
	return float32(*api) == mine
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// archive queries the Open-Meteo archive API (the oracle).
func archive(t *testing.T, c *http.Client, p domain.GeoPoint, from, to time.Time, kind string, vars []string) map[string][]*float64 {
	t.Helper()
	q := url.Values{
		"latitude": {fmt.Sprint(p.Lat)}, "longitude": {fmt.Sprint(p.Lon)}, "elevation": {fmt.Sprint(p.Elevation)},
		"start_date": {from.Format("2006-01-02")}, "end_date": {to.Format("2006-01-02")},
		"timezone": {"UTC"}, kind: {joinComma(vars)},
	}
	for attempt := range 5 {
		resp, err := c.Get("https://archive-api.open-meteo.com/v1/archive?" + q.Encode())
		if err == nil && resp.StatusCode == http.StatusOK {
			var body map[string]json.RawMessage
			err = json.NewDecoder(resp.Body).Decode(&body)
			_ = resp.Body.Close()
			var out map[string][]*float64
			var block map[string]json.RawMessage
			if err == nil && json.Unmarshal(body[kind], &block) == nil {
				out = map[string][]*float64{}
				for _, v := range vars {
					var s []*float64
					_ = json.Unmarshal(block[v], &s)
					out[v] = s
				}
				return out
			}
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}
	t.Fatalf("archive API unavailable")
	return nil
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}
