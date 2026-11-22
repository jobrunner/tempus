package reanalysis

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

const (
	// pointsEnv overrides how many seeded points the sweep draws.
	pointsEnv                = "TEMPUS_OM_POINTS"
	defaultConformancePoints = 20
	// conformanceSeed fixes the sample, so every run checks the same points
	// and a miss can be replayed.
	conformanceSeed = 20261122
	// maxSampleElevation bounds the drawn elevations (whole metres, from 0).
	// They are drawn independently of the terrain on purpose: a height far
	// off the model's exercises cell selection and lapse-rate correction.
	maxSampleElevation = 2500
)

// westPalearctic is the sampling box: Iceland/Azores to the Caspian, the
// northern Sahara to the North Cape.
var westPalearctic = struct{ south, north, west, east float64 }{30, 72, -25, 50}

// sweepVars are the weather provider's hourly variables and the decimals the
// archive API prints them with.
var sweepVars = []struct {
	name     string
	decimals int
}{
	{domain.VarTemperature, 1},
	{domain.VarHumidity, 0},
	{domain.VarPrecip, 1},
	{domain.VarWeatherCode, 0},
	{domain.VarWind, 1},
	{domain.VarCloud, 0},
}

// conformanceWindow is a range of whole UTC days, first … last inclusive.
type conformanceWindow struct {
	name        string
	first, last time.Time
}

// conformanceWindows covers the archive's eras: ERA5/ERA5-Land only (2005),
// the start of ECMWF IFS HRES on 1 January 2017, a settled recent year, and
// the ten days before the last three, where IFS is all but final.
func conformanceWindows(now time.Time) []conformanceWindow {
	today := now.UTC().Truncate(24 * time.Hour)
	return []conformanceWindow{
		{"2005", date(2005, 7, 12), date(2005, 7, 12)},
		{"ifs-switch-2017", date(2016, 12, 31), date(2017, 1, 1)},
		{"last-year", today.AddDate(-1, 0, 0), today.AddDate(-1, 0, 0)},
		{"recent", today.AddDate(0, 0, -12), today.AddDate(0, 0, -3)},
	}
}

// conformancePointCount parses TEMPUS_OM_POINTS (empty = the default).
func conformancePointCount(env string) (int, error) {
	if env == "" {
		return defaultConformancePoints, nil
	}
	n, err := strconv.Atoi(env)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s=%q: want a positive integer", pointsEnv, env)
	}
	return n, nil
}

// conformancePoints draws n points in the West Palearctic box with a seeded
// splitmix64 sequence: stable across Go versions and platforms, and a
// prefix of a longer draw.
func conformancePoints(n int, seed uint64) []domain.GeoPoint {
	state := seed
	next := func() float64 { // uniform in [0, 1)
		state += 0x9e3779b97f4a7c15
		z := state
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z ^= z >> 31
		return float64(z>>11) / (1 << 53)
	}
	box := westPalearctic
	out := make([]domain.GeoPoint, n)
	for i := range out {
		lat := box.south + next()*(box.north-box.south)
		lon := box.west + next()*(box.east-box.west)
		out[i] = domain.GeoPoint{
			// Five decimals (≈1 m) keep the logged points short; floor keeps
			// them inside the half-open box.
			Lat:       math.Floor(lat*1e5) / 1e5,
			Lon:       math.Floor(lon*1e5) / 1e5,
			Elevation: math.Floor(next() * maxSampleElevation),
		}
	}
	return out
}

// TestConformance_WestPalearctic is the end-to-end conformance sweep:
// Service.Hourly, reading the open-data bucket, against the Open-Meteo archive
// API (best_match, same elevation) for seeded points × fixed windows. One API
// call per point and window carries all variables. Every value must equal the
// API's after rounding to its decimals; misses are listed, and the ones that
// are one step in the last decimal are counted apart.
//
// Opt-in, it calls both services: TEMPUS_OM_LIVE=1 (make om-conformance).
// TEMPUS_OM_POINTS sets the number of points (default 20).
func TestConformance_WestPalearctic(t *testing.T) {
	skipUnlessLive(t)
	n, err := conformancePointCount(os.Getenv(pointsEnv))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	svc := liveService()
	names := make([]string, len(sweepVars))
	for i, v := range sweepVars {
		names[i] = v.name
	}
	tally := newTally(t)
	start, calls := time.Now(), 0
	windows := conformanceWindows(start)
	for i, p := range conformancePoints(n, conformanceSeed) {
		for _, w := range windows {
			label := fmt.Sprintf("#%d (%.5f, %.5f, %.0f m) %s", i, p.Lat, p.Lon, p.Elevation, w.name)
			got, err := svc.Hourly(context.Background(), p, w.first, w.last.Add(24*time.Hour), names...)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			api := archive(t, client, p, w.first, w.last, kindHourly, names)
			calls++
			for _, v := range sweepVars {
				tally.compare(label, w.name, v.name, got.Values[v.name], api[v.name], v.decimals)
			}
		}
	}
	t.Logf("%d points × %d windows, %d archive API calls, %v", n, len(windows), calls, time.Since(start).Round(time.Second))
	tally.report(1)
}
