package openmeteo

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeReanalysis serves a canned series and records the last Hourly call.
type fakeReanalysis struct {
	series domain.HourlySeries
	err    error

	calls    int
	point    domain.GeoPoint
	from, to time.Time
	vars     []string
}

func (f *fakeReanalysis) Hourly(_ context.Context, p domain.GeoPoint, from, to time.Time, variables ...string) (domain.HourlySeries, error) {
	f.calls++
	f.point, f.from, f.to, f.vars = p, from, to, variables
	return f.series, f.err
}

func (f *fakeReanalysis) Daily(context.Context, domain.GeoPoint, time.Time, time.Time) (domain.DailySeries, error) {
	return domain.DailySeries{}, errors.New("not used by the weather provider")
}

var (
	day     = time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	instant = day.Add(13 * time.Hour)
)

func req(t time.Time) domain.QueryRequest {
	return domain.QueryRequest{Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93}, Elevation: 177, Instant: t}
}

// daySeries is a 24-hour series for day with value at hour 13 and NaN elsewhere.
func daySeries(values map[string]float64) domain.HourlySeries {
	s := domain.HourlySeries{From: day, Values: map[string][]float64{}}
	for name, v := range values {
		hours := make([]float64, 24)
		for i := range hours {
			hours[i] = math.NaN()
		}
		hours[13] = v
		s.Values[name] = hours
	}
	return s
}

func newProvider(fake *fakeReanalysis) *Provider {
	return New(Options{
		Reanalysis: fake,
		Clock:      fixedClock{time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)},
	})
}

func fullHour() map[string]float64 {
	return map[string]float64{
		domain.VarTemperature: 21.44,
		domain.VarHumidity:    64.5,
		domain.VarPrecip:      0.05,
		domain.VarWeatherCode: 3,
		domain.VarWind:        12.06,
		domain.VarCloud:       87.4,
	}
}

func TestFetch_RoundsLikeOpenMeteoAndReportsUnits(t *testing.T) {
	hour := fullHour()
	hour[domain.VarTemperature] = -0.25
	p := newProvider(&fakeReanalysis{series: daySeries(hour)})

	res, err := p.Fetch(context.Background(), req(instant))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	want := map[string]any{
		propTemperature: -0.3,
		propHumidity:    65,
		propPrecip:      0.1,
		propWeatherCode: 3,
		propWindSpeed:   12.1,
		propCloudCover:  87,
	}
	for k, v := range want {
		if props[k] != v {
			t.Errorf("%s = %v (%T), want %v (%T)", k, props[k], props[k], v, v)
		}
	}
	wantUnits := map[string]string{
		propTemperature: "°C",
		propHumidity:    "%",
		propPrecip:      "mm",
		propWeatherCode: "wmo code",
		propWindSpeed:   unitKmh,
		propCloudCover:  "%",
	}
	if got := props["units"]; !reflect.DeepEqual(got, wantUnits) {
		t.Errorf("units = %v, want %v", got, wantUnits)
	}
	if props["provider"] != providerID || props["kind"] != providerKind {
		t.Errorf("provider/kind = %v/%v", props["provider"], props["kind"])
	}
	if got := props["observedAt"]; got != "2025-06-15T13:00:00Z" {
		t.Errorf("observedAt = %v", got)
	}
}

func TestFetch_ReadsTheWholeUTCDay(t *testing.T) {
	fake := &fakeReanalysis{series: daySeries(fullHour())}
	p := newProvider(fake)

	if _, err := p.Fetch(context.Background(), req(instant)); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !fake.from.Equal(day) || !fake.to.Equal(day.Add(24*time.Hour)) {
		t.Errorf("window = [%v, %v), want the UTC day [%v, %v)", fake.from, fake.to, day, day.Add(24*time.Hour))
	}
	if want := (domain.GeoPoint{Lat: 49.79, Lon: 9.93, Elevation: 177}); fake.point != want {
		t.Errorf("point = %+v, want %+v", fake.point, want)
	}
	wantVars := []string{
		domain.VarTemperature, domain.VarHumidity, domain.VarPrecip,
		domain.VarWeatherCode, domain.VarWind, domain.VarCloud,
	}
	if !reflect.DeepEqual(fake.vars, wantVars) {
		t.Errorf("variables = %v, want %v", fake.vars, wantVars)
	}
}

func TestFetch_GeometryElevationAndModelCells(t *testing.T) {
	series := daySeries(fullHour())
	series.Cells = []domain.ModelCell{
		{Model: "ecmwf_ifs", Lat: 49.787654321, Lon: 9.9312345678, Elevation: 181.26},
		{Model: "copernicus_era5_land", Lat: 49.8, Lon: 9.9, Elevation: 177},
	}
	p := newProvider(&fakeReanalysis{series: series})

	res, err := p.Fetch(context.Background(), req(instant))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// geometry is the requested point, not a model cell.
	if c := res.Feature.Geometry.Coordinates; c[0] != 9.93 || c[1] != 49.79 {
		t.Errorf("geometry = %v, want [9.93 49.79]", c)
	}
	if got := res.Feature.Properties[propElevation]; got != 177.0 {
		t.Errorf("elevation = %v, want 177", got)
	}
	want := []map[string]any{
		{cellModel: "ecmwf_ifs", cellLat: 49.78765, cellLon: 9.93123, propElevation: 181.3},
		{cellModel: "copernicus_era5_land", cellLat: 49.8, cellLon: 9.9, propElevation: 177.0},
	}
	if got := res.Feature.Properties["modelCells"]; !reflect.DeepEqual(got, want) {
		t.Errorf("modelCells = %v, want %v", got, want)
	}
}

func TestFetch_LicenseAndAttribution(t *testing.T) {
	p := newProvider(&fakeReanalysis{series: daySeries(fullHour())})
	res, err := p.Fetch(context.Background(), req(instant))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.Feature.License != domain.ReanalysisLicense() {
		t.Errorf("feature license = %+v, want the reanalysis license", res.Feature.License)
	}
	if p.Attribution() != domain.ReanalysisLicense() {
		t.Errorf("Attribution = %+v, want the reanalysis license", p.Attribution())
	}
	if p.ID() != "open-meteo" || p.Kind() != "weather" {
		t.Errorf("ID/Kind = %s/%s", p.ID(), p.Kind())
	}
}

func TestFetch_Enrichments(t *testing.T) {
	// weather_code 3 → "Bedeckt"/"Overcast"; 12.06 km/h → 12.1 → Beaufort 3.
	p := newProvider(&fakeReanalysis{series: daySeries(fullHour())})
	res, err := p.Fetch(context.Background(), req(instant))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties

	if props["isDay"] != true {
		t.Errorf("isDay = %v, want true (solar-computed)", props["isDay"])
	}
	if src, ok := props["isDaySource"].(string); !ok || src == "" {
		t.Errorf("isDaySource = %v, want non-empty string", props["isDaySource"])
	}

	desc, ok := props["weatherCodeDescription"].(map[string]string)
	if !ok || desc["de"] != "Bedeckt" || desc["en"] != "Overcast" {
		t.Errorf("weatherCodeDescription = %v, want Bedeckt/Overcast", props["weatherCodeDescription"])
	}
	if got := props["weatherCodeSourceURL"]; got != domain.WMOCodeSourceURL {
		t.Errorf("weatherCodeSourceURL = %v, want %q", got, domain.WMOCodeSourceURL)
	}

	if got := props["windBeaufort"]; got != 3 {
		t.Errorf("windBeaufort = %v (%T), want 3", got, got)
	}
	bdesc, ok := props["windBeaufortDescription"].(map[string]string)
	if !ok || bdesc["de"] != "schwache Brise" || bdesc["en"] != "Gentle breeze" {
		t.Errorf("windBeaufortDescription = %v, want schwache Brise/Gentle breeze", props["windBeaufortDescription"])
	}
	if got := props["windBeaufortSourceURL"]; got != domain.BeaufortSourceURL {
		t.Errorf("windBeaufortSourceURL = %v, want %q", got, domain.BeaufortSourceURL)
	}
}

func TestFetch_MissingSecondaryVariableIsOmitted(t *testing.T) {
	hour := fullHour()
	delete(hour, domain.VarWind)
	hour[domain.VarCloud] = math.NaN()
	p := newProvider(&fakeReanalysis{series: daySeries(hour)})

	res, err := p.Fetch(context.Background(), req(instant))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	props := res.Feature.Properties
	for _, key := range []string{propWindSpeed, "windBeaufort", propCloudCover} {
		if v, present := props[key]; present {
			t.Errorf("%s = %v, want absent", key, v)
		}
	}
	units, _ := props["units"].(map[string]string)
	if _, present := units[propWindSpeed]; present {
		t.Errorf("units carry windSpeed10m without a value: %v", units)
	}
}

func TestFetch_MissingTemperatureIsNotYetAvailable(t *testing.T) {
	hour := fullHour()
	hour[domain.VarTemperature] = math.NaN()
	p := newProvider(&fakeReanalysis{series: daySeries(hour)})

	_, err := p.Fetch(context.Background(), req(instant))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassNotYetAvailable || pe.RetryAfter != notYetAvailableRetry {
		t.Fatalf("want not-yet-available, got %v", err)
	}
}

func TestFetch_PortErrorPassesThrough(t *testing.T) {
	portErr := output.NewTransientError(errors.New("bucket unreachable"), 30*time.Second)
	p := newProvider(&fakeReanalysis{err: portErr})

	_, err := p.Fetch(context.Background(), req(instant))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Class != output.ClassTransient || pe.RetryAfter != 30*time.Second {
		t.Fatalf("want the port's transient error unchanged, got %v", err)
	}
}

func TestFetch_FutureRejected(t *testing.T) {
	fake := &fakeReanalysis{series: daySeries(fullHour())}
	p := newProvider(fake)

	// fixedClock is 2026-07-21T12:00Z; ask for a later hour.
	_, err := p.Fetch(context.Background(), req(time.Date(2026, 7, 21, 13, 0, 0, 0, time.UTC)))
	pe, ok := output.AsProviderError(err)
	if !ok || pe.Retryable {
		t.Fatalf("want non-retryable ProviderError for future date, got %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("provider read the archive %d times for a future date", fake.calls)
	}
}
