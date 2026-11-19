package aggregate

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeReanalysis serves fixed series and records what the provider asked for.
type fakeReanalysis struct {
	daily     domain.DailySeries
	hourly    domain.HourlySeries
	dailyErr  error
	hourlyErr error

	calls             int
	point             domain.GeoPoint
	firstDay, lastDay time.Time
	hourFrom, hourTo  time.Time
	hourlyVars        []string
}

func (f *fakeReanalysis) Hourly(_ context.Context, p domain.GeoPoint, from, to time.Time, variables ...string) (domain.HourlySeries, error) {
	f.calls++
	f.point, f.hourFrom, f.hourTo, f.hourlyVars = p, from, to, variables
	return f.hourly, f.hourlyErr
}

func (f *fakeReanalysis) Daily(_ context.Context, p domain.GeoPoint, firstDay, lastDay time.Time) (domain.DailySeries, error) {
	f.calls++
	f.point, f.firstDay, f.lastDay = p, firstDay, lastDay
	return f.daily, f.dailyErr
}

var (
	now     = time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	instant = time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)
	nan     = math.NaN()
)

// newFake returns three fund-window days (13–15 June) and four precipitation
// hours ending at the instant.
func newFake() *fakeReanalysis {
	return &fakeReanalysis{
		daily: domain.DailySeries{
			FirstDay: time.Date(2025, 6, 13, 0, 0, 0, 0, time.UTC),
			TMax:     []float64{20, 22, 24},
			TMin:     []float64{10, 12, 14},
			TMean:    []float64{15, 17, 19},
		},
		hourly: domain.HourlySeries{
			From:   time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			Values: map[string][]float64{domain.VarPrecip: {1, 2, 0, 3}},
		},
	}
}

func newProvider(f *fakeReanalysis) *Provider {
	return New(Options{Reanalysis: f, Clock: fixedClock{now}})
}

func req(instant time.Time, base *float64) domain.QueryRequest {
	return domain.QueryRequest{
		Coordinate:     domain.Coordinate{Lat: 49.79, Lon: 9.93},
		Elevation:      180,
		Instant:        instant,
		GDDBaseCelsius: base,
	}
}

func TestFetch_ComputesAggregates(t *testing.T) {
	f := newFake()
	r := req(instant, nil)
	res, err := newProvider(f).Fetch(context.Background(), r)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if props["kind"] != "aggregate" {
		t.Errorf("kind = %v, want aggregate", props["kind"])
	}

	precip, ok := props["precipitation"].(map[string]any)
	if !ok {
		t.Fatalf("precipitation missing: %v", props["precipitation"])
	}
	if precip["last24h"] != 6.0 { // 1+2+0+3
		t.Errorf("last24h = %v, want 6", precip["last24h"])
	}

	day, ok := props["temperatureDay"].(map[string]any)
	if !ok {
		t.Fatalf("temperatureDay missing")
	}
	if day["min"] != 14.0 || day["max"] != 24.0 || day["mean"] != 19.0 {
		t.Errorf("temperatureDay = %v, want min14/max24/mean19", day)
	}

	gdd, ok := props["growingDegreeDays"].(map[string]any)
	if !ok {
		t.Fatalf("growingDegreeDays missing")
	}
	// base 10: (5)+(7)+(9) = 21; base 5: (10)+(12)+(14) = 36. Both always present.
	if gdd["base10"] != 21.0 {
		t.Errorf("gdd base10 = %v, want 21", gdd["base10"])
	}
	if gdd["base5"] != 36.0 {
		t.Errorf("gdd base5 = %v, want 36", gdd["base5"])
	}
	if gdd["since"] != "2025-01-01" {
		t.Errorf("since = %v, want 2025-01-01", gdd["since"])
	}
	if gdd["days"] != 3 {
		t.Errorf("days = %v, want 3", gdd["days"])
	}
	if _, hasCustom := gdd["custom"]; hasCustom {
		t.Errorf("custom present without a gddBase override: %v", gdd["custom"])
	}

	att, ok := props["arrheniusThermalTime"].(map[string]any)
	if !ok {
		t.Fatalf("arrheniusThermalTime missing")
	}
	if att["referenceTempC"] != 20.0 || att["activationEnergyEv"] != 0.65 || att["days"] != 3 {
		t.Errorf("arrhenius meta = %v, want Tref 20 / E 0.65 / days 3", att)
	}
	if v, ok := att["value"].(float64); !ok || v <= 0 {
		t.Errorf("arrhenius value = %v, want > 0", att["value"])
	}

	// The daily range runs from the GDD start to the fund day, at the request point.
	if f.point != r.PointOf() {
		t.Errorf("point = %+v, want %+v", f.point, r.PointOf())
	}
	if want := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC); !f.firstDay.Equal(want) {
		t.Errorf("firstDay = %v, want %v", f.firstDay, want)
	}
	if want := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC); !f.lastDay.Equal(want) {
		t.Errorf("lastDay = %v, want %v", f.lastDay, want)
	}

	// The feature sits on the request coordinate and carries the reanalysis license.
	if c := res.Feature.Geometry.Coordinates; len(c) != 2 || c[0] != 9.93 || c[1] != 49.79 {
		t.Errorf("coordinates = %v, want request coordinate", res.Feature.Geometry.Coordinates)
	}
	lic, want := res.Feature.License, domain.ReanalysisLicense()
	if lic.Name != want.Name || lic.URL != want.URL || lic.Attribution == "" {
		t.Errorf("license = %+v, want the reanalysis license", lic)
	}
}

func TestFetch_GDDBaseOverride(t *testing.T) {
	base := 7.0 // distinct from the fixed 5 and 10
	res, err := newProvider(newFake()).Fetch(context.Background(), req(instant, &base))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	gdd := res.Feature.Properties["growingDegreeDays"].(map[string]any)
	// base5/base10 remain; the override is added under "custom".
	if gdd["base5"] != 36.0 || gdd["base10"] != 21.0 {
		t.Errorf("gdd base5/base10 = %v/%v, want 36/21", gdd["base5"], gdd["base10"])
	}
	custom, ok := gdd["custom"].(map[string]any)
	if !ok {
		t.Fatalf("custom missing for gddBase=7")
	}
	// base 7: (15-7)+(17-7)+(19-7) = 8+10+12 = 30.
	if custom["baseCelsius"] != 7.0 || custom["value"] != 30.0 {
		t.Errorf("custom = %v, want baseCelsius 7 / value 30", custom)
	}
}

func TestFetch_PrecipWindows(t *testing.T) {
	// 121 hours: one before the 120-h window, then 120 hours ending at the
	// instant. Each window counts the instant's hour and the hours before it.
	vals := make([]float64, 121)
	vals[0] = 1000 // outside every window
	for i := 1; i <= 120; i++ {
		switch {
		case i > 120-24:
			vals[i] = 1 // inside 24 h
		case i > 120-72:
			vals[i] = 0.5 // inside 72 h only
		default:
			vals[i] = 0.25 // inside 120 h only
		}
	}
	vals[110] = nan // a missing hour inside the window is skipped
	f := newFake()
	f.hourly = domain.HourlySeries{
		From:   instant.Add(-120 * time.Hour),
		Values: map[string][]float64{domain.VarPrecip: vals},
	}

	res, err := newProvider(f).Fetch(context.Background(), req(instant, nil))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	precip := res.Feature.Properties["precipitation"].(map[string]any)
	// 24 h: 23×1 (one NaN); 72 h: +48×0.5; 120 h: +48×0.25.
	if precip["last24h"] != 23.0 || precip["last72h"] != 47.0 || precip["last120h"] != 59.0 {
		t.Errorf("precipitation = %v, want 23/47/59", precip)
	}

	// Exactly the window's hours are requested: [instant−119 h, instant+1 h).
	if want := instant.Add(-119 * time.Hour); !f.hourFrom.Equal(want) {
		t.Errorf("hourly from = %v, want %v", f.hourFrom, want)
	}
	if want := instant.Add(time.Hour); !f.hourTo.Equal(want) {
		t.Errorf("hourly to = %v, want %v", f.hourTo, want)
	}
	if len(f.hourlyVars) != 1 || f.hourlyVars[0] != domain.VarPrecip {
		t.Errorf("hourly variables = %v, want [%s]", f.hourlyVars, domain.VarPrecip)
	}
}

func TestFetch_SumsUseUnroundedValues(t *testing.T) {
	// 0.04 mm per hour would round to 0.0 at display precision; summed
	// unrounded, 24 h give 0.96 → 1.0. Likewise 10.04 °C days add up.
	vals := make([]float64, 24)
	for i := range vals {
		vals[i] = 0.04
	}
	f := newFake()
	f.hourly = domain.HourlySeries{
		From:   instant.Add(-23 * time.Hour),
		Values: map[string][]float64{domain.VarPrecip: vals},
	}
	f.daily = domain.DailySeries{
		FirstDay: time.Date(2025, 6, 13, 0, 0, 0, 0, time.UTC),
		TMax:     []float64{10.04, 10.04, 10.04},
		TMin:     []float64{10.04, 10.04, 10.04},
		TMean:    []float64{10.04, 10.04, 10.04},
	}

	res, err := newProvider(f).Fetch(context.Background(), req(instant, nil))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if got := props["precipitation"].(map[string]any)["last24h"]; got != 1.0 {
		t.Errorf("last24h = %v, want 1 (24×0.04 unrounded)", got)
	}
	if got := props["growingDegreeDays"].(map[string]any)["base10"]; got != 0.1 {
		t.Errorf("gdd base10 = %v, want 0.1 (3×0.04 unrounded)", got)
	}
	if got := props["temperatureDay"].(map[string]any)["max"]; got != 10.0 {
		t.Errorf("temperatureDay max = %v, want display-rounded 10", got)
	}
}

func TestFetch_MissingValues(t *testing.T) {
	// The instant's hour is NaN (not yet in the archive): no precipitation.
	// The fund day's mean is NaN: (max+min)/2 instead. An incomplete day
	// drops out of the GDD sum.
	f := newFake()
	f.hourly.Values[domain.VarPrecip][3] = nan
	f.daily.TMean[2] = nan
	f.daily.TMin[0] = nan

	res, err := newProvider(f).Fetch(context.Background(), req(instant, nil))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	if _, has := props["precipitation"]; has {
		t.Errorf("precipitation present for a NaN instant hour: %v", props["precipitation"])
	}
	if got := props["temperatureDay"].(map[string]any)["mean"]; got != 19.0 {
		t.Errorf("mean = %v, want (24+14)/2 = 19", got)
	}
	gdd := props["growingDegreeDays"].(map[string]any)
	if gdd["days"] != 2 || gdd["base10"] != 16.0 {
		t.Errorf("gdd days/base10 = %v/%v, want 2/16", gdd["days"], gdd["base10"])
	}
}

func TestFetch_NotYetAvailable(t *testing.T) {
	// Every value NaN: nothing usable yet, so a retryable not-yet-available error.
	f := newFake()
	f.hourly.Values[domain.VarPrecip] = []float64{nan, nan, nan, nan}
	f.daily.TMax = []float64{nan, nan, nan}
	f.daily.TMin = []float64{nan, nan, nan}
	f.daily.TMean = []float64{nan, nan, nan}

	_, err := newProvider(f).Fetch(context.Background(), req(instant, nil))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassNotYetAvailable || !pe.Retryable {
		t.Fatalf("want retryable not-yet-available ProviderError, got %v", err)
	}
}

func TestFetch_FutureRejected(t *testing.T) {
	f := newFake()
	_, err := newProvider(f).Fetch(context.Background(), req(now.Add(time.Hour), nil))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Retryable {
		t.Fatalf("want non-retryable ProviderError for future date, got %v", err)
	}
	if f.calls != 0 {
		t.Errorf("reanalysis called %d times for a future instant, want 0", f.calls)
	}
}

func TestFetch_PassesPortErrorsThrough(t *testing.T) {
	// Port failures are already classified; they must surface unchanged.
	want := output.NewTransientError(errors.New("bucket unavailable"), 3*time.Hour)
	for _, tc := range []struct {
		name string
		set  func(*fakeReanalysis)
	}{
		{"daily", func(f *fakeReanalysis) { f.dailyErr = want }},
		{"hourly", func(f *fakeReanalysis) { f.hourlyErr = want }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			tc.set(f)
			_, err := newProvider(f).Fetch(context.Background(), req(instant, nil))
			pe, ok := output.AsProviderError(err)
			if !ok || pe != want {
				t.Errorf("Fetch err = %v, want the port's ProviderError unchanged", err)
			}
		})
	}
}

func TestProviderIDKind(t *testing.T) {
	p := New(Options{Clock: fixedClock{time.Now().UTC()}})
	if p.ID() != "aggregate" || p.Kind() != "aggregate" {
		t.Errorf("ID/Kind = %q/%q, want aggregate/aggregate", p.ID(), p.Kind())
	}
	if p.Attribution().Attribution == "" {
		t.Error("attribution empty")
	}
}
