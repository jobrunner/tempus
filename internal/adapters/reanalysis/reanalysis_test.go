package reanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// TestEmbeddedElevationsMatchManifest pins the embedded model heights to the
// checksums recorded when they were fetched.
func TestEmbeddedElevationsMatchManifest(t *testing.T) {
	manifest, err := os.ReadFile("data/MANIFEST")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		fields := strings.Fields(line)
		raw, err := elevationFiles.ReadFile("data/" + fields[0])
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if want := strings.TrimPrefix(fields[1], "sha256="); hex.EncodeToString(sum[:]) != want {
			t.Errorf("%s: checksum %x, manifest %s", fields[0], sum, want)
		}
	}
}

func TestModelElevations(t *testing.T) {
	elev, err := loadElevations()
	if err != nil {
		t.Fatal(err)
	}
	era5 := elev[domainERA5]
	// The south pole row is ice sheet, the equator at 0°/0° is sea.
	if h := era5.At(0); h < 2000 {
		t.Errorf("south pole: %v m", h)
	}
	if h := era5.At(360*1440 + 720); h > -999 {
		t.Errorf("Gulf of Guinea should be sea: %v", h)
	}
	if h := era5.At(-1); !math.IsNaN(float64(h)) {
		t.Errorf("negative index: %v", h)
	}
	if h := era5.At(1440 * 721); !math.IsNaN(float64(h)) {
		t.Errorf("index past the grid: %v", h)
	}
}

// TestIFSCellSelectionMatchesOracle replays the 120 cell selections recorded
// against the Open-Meteo API in phase 0 (coast, date line, Greenwich, heights
// ±60/150/400 m off the DEM): the IFS cell tempus picks must be the one the
// API reported.
func TestIFSCellSelectionMatchesOracle(t *testing.T) {
	var cases []struct {
		Lab                 string
		Lat, Lon, Elevation float64
		APILat, APILon      float64
	}
	raw, err := os.ReadFile("testdata/ifs_selection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	elev, err := loadElevations()
	if err != nil {
		t.Fatal(err)
	}
	ifs := models[0]
	for _, c := range cases {
		cell := ifs.grid.Select(float32(c.Lat), float32(c.Lon), float32(c.Elevation), elev[domainIFS])
		lat, lon := ifs.grid.Coordinates(cell.Index)
		if math.Abs(float64(lat)-c.APILat) > 1e-4 || math.Abs(float64(lon)-c.APILon) > 1e-4 {
			t.Errorf("%s %v/%v @%v m: tempus cell %v/%v, API %v/%v", c.Lab, c.Lat, c.Lon, c.Elevation, lat, lon, c.APILat, c.APILon)
		}
	}
}

// fakeReader serves constant series per (model, variable) and records reads.
type fakeReader struct {
	mu     sync.Mutex
	values map[string]float32 // model/variable → value; missing → NaN
	reads  []string
	err    error
	// failOn makes only this model/variable fail with err.
	failOn string
	// at, when set, serves the value of model/variable at hour t instead of
	// the constant from values.
	at func(key string, t time.Time) float32
	// uncovered lists model/variable keys the archive has no file for.
	uncovered map[string]bool
	// coverageErr fails coverage lookups — all, or only coverageErrOn's.
	coverageErr   error
	coverageErrOn string
	// coverageHangOn makes that model/variable's coverage lookup hang until
	// its context ends; hangEnded is closed then (if set).
	coverageHangOn string
	hangEnded      chan struct{}
	// coverageHours overrides the covered hours per model/variable.
	coverageHours map[string]int
}

func (f *fakeReader) Coverage(ctx context.Context, model, variable string, _ []uint64, from, to time.Time) (int, error) {
	if f.coverageHangOn == model+"/"+variable {
		<-ctx.Done()
		if f.hangEnded != nil {
			close(f.hangEnded)
		}
		return 0, ctx.Err()
	}
	if f.coverageErr != nil && (f.coverageErrOn == "" || f.coverageErrOn == model+"/"+variable) {
		return 0, f.coverageErr
	}
	if n, ok := f.coverageHours[model+"/"+variable]; ok {
		return n, nil
	}
	if f.uncovered[model+"/"+variable] {
		return 0, nil
	}
	return int(to.Sub(from) / time.Hour), nil
}

func (f *fakeReader) ReadSeries(_ context.Context, model, variable string, _ []uint64, from, to time.Time) ([]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := model + "/" + variable
	f.reads = append(f.reads, key)
	if f.err != nil && (f.failOn == "" || f.failOn == key) {
		return nil, f.err
	}
	v, ok := f.values[key]
	out := make([]float32, int(to.Sub(from)/time.Hour))
	for i := range out {
		out[i] = nan32
		switch {
		case f.at != nil:
			out[i] = f.at(key, from.Add(time.Duration(i)*time.Hour))
		case ok:
			out[i] = v
		}
	}
	return out, nil
}

func (f *fakeReader) didRead(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reads {
		if r == key {
			return true
		}
	}
	return false
}

var (
	day    = time.Date(2005, 6, 1, 0, 0, 0, 0, time.UTC)
	munich = domain.GeoPoint{Lat: 48.14, Lon: 11.58, Elevation: 519}
)

func TestHourly_FallsBackOnlyWhereTheHigherModelHasGaps(t *testing.T) {
	f := &fakeReader{values: map[string]float32{
		// IFS has no data in 2005; ERA5-Land covers temperature, ERA5 the rest.
		domainERA5Land + "/" + VarTemperature: 20, domainERA5Land + "/" + VarDewPoint: 10,
		domainERA5 + "/" + VarPrecip: 0.4, domainERA5 + "/" + VarCloud: 90,
		domainERA5 + "/snowfall_water_equivalent": 0, domainERA5 + "/" + VarTemperature: 99,
	}}
	h, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour),
		domain.VarTemperature, domain.VarHumidity, domain.VarWeatherCode)
	if err != nil {
		t.Fatal(err)
	}
	if f.didRead(domainERA5 + "/" + VarTemperature) {
		t.Error("ERA5 temperature read although ERA5-Land filled every hour")
	}
	land := h.Cells[1]
	if land.Model != domainERA5Land || land.Elevation == munich.Elevation {
		t.Fatalf("the test needs an ERA5-Land cell off the target height: %+v", land)
	}
	shift := float32(float32(float32(land.Elevation)-float32(munich.Elevation)) * ra.LapseRate)
	if temp, want := h.Values[domain.VarTemperature][0], float64(20+shift); temp != want {
		t.Errorf("temperature from ERA5-Land, height-corrected: %v want %v", temp, want)
	}
	if wc := h.Values[domain.VarWeatherCode][0]; wc != 51 {
		t.Errorf("weather code from ERA5's 0.4 mm (drizzle beats cloud cover): %v", wc)
	}
	if rh := h.Values[domain.VarHumidity][0]; rh < 40 || rh > 60 {
		t.Errorf("humidity: %v", rh)
	}
	if len(h.Cells) != 3 || h.Cells[0].Model != domainIFS {
		t.Errorf("cells: %+v", h.Cells)
	}
	if _, ok := h.Values[domain.VarWind]; ok {
		t.Error("wind was not requested")
	}
}

func TestDaily_AggregatesUTCdays(t *testing.T) {
	f := &fakeReader{values: map[string]float32{
		domainIFS + "/" + VarTemperature: 12, domainIFS + "/" + VarPrecip: 0.5,
	}}
	d, err := NewService(f).Daily(context.Background(), domain.GeoPoint{Lat: 48.14, Lon: 11.58, Elevation: 519},
		time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2020, 3, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.TMax) != 3 || d.PrecipSum[1] != 12 {
		t.Fatalf("daily: %+v", d)
	}
	if d.TMax[0] != d.TMin[0] || d.TMean[2] != d.TMax[2] {
		t.Errorf("constant temperature must give equal extrema and mean: %+v", d)
	}
}

func TestService_Errors(t *testing.T) {
	svc := NewService(&fakeReader{err: output.NewTransientError(errors.New("bucket down"), time.Second)})
	_, err := svc.Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature)
	if pe, ok := output.AsProviderError(err); !ok || !pe.Retryable {
		t.Errorf("transient upstream error must stay retryable: %v", err)
	}
	svc = NewService(&fakeReader{err: errors.New("corrupt chunk")})
	_, err = svc.Daily(context.Background(), munich, day, day)
	if pe, ok := output.AsProviderError(err); !ok || pe.Retryable {
		t.Errorf("decode errors are permanent: %v", err)
	}
	ctxErr := NewService(&fakeReader{err: context.DeadlineExceeded})
	if _, err := ctxErr.Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature); err == nil {
		t.Error("deadline: no error")
	} else if pe, _ := output.AsProviderError(err); !pe.Retryable {
		t.Errorf("deadline must be retryable: %v", err)
	}
	if _, err := svc.Hourly(context.Background(), munich, day, day, domain.VarTemperature); err == nil {
		t.Error("empty hour range: no error")
	}
	if _, err := svc.Daily(context.Background(), munich, day, day.AddDate(0, 0, -1)); err == nil {
		t.Error("empty day range: no error")
	}
}

func TestHourly_RawUnitsAndUncorrectedVariables(t *testing.T) {
	f := &fakeReader{values: map[string]float32{
		domainIFS + "/" + VarPrecip: 0.4, domainIFS + "/" + VarCloud: 30,
		domainIFS + "/wind_u_component_10m": 3, domainIFS + "/wind_v_component_10m": 4,
		domainIFS + "/snowfall_water_equivalent": 1, domainIFS + "/" + VarTemperature: -5,
	}}
	// Far off the IFS cell's height, so the snow rule and the lapse-rate
	// correction would both act if they were (wrongly) applied.
	high := domain.GeoPoint{Lat: munich.Lat, Lon: munich.Lon, Elevation: 3000}
	h, err := NewService(f).Hourly(context.Background(), high, day, day.Add(2*time.Hour),
		domain.VarPrecip, domain.VarCloud, domain.VarWind, domain.VarSnowfall)
	if err != nil {
		t.Fatal(err)
	}
	for v, want := range map[string]float32{
		domain.VarPrecip: 0.4, domain.VarCloud: 30, domain.VarWind: 18, domain.VarSnowfall: ra.SnowfallCm(1),
	} {
		if got := h.Values[v]; len(got) != 2 || got[1] != float64(want) {
			t.Errorf("%s: got %v want %v (no height correction)", v, got, want)
		}
	}
}

func TestHourly_WindComponentErrors(t *testing.T) {
	for _, comp := range []string{"wind_u_component_10m", "wind_v_component_10m"} {
		f := &fakeReader{err: errors.New("corrupt chunk"), failOn: domainIFS + "/" + comp}
		_, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarWind)
		if err == nil {
			t.Errorf("%s: error swallowed", comp)
		}
	}
}

func TestDaily_WindowAndDayBoundaries(t *testing.T) {
	// Precipitation equals the day of the month in every hour of that UTC day,
	// temperature ten times that: an hour taken from the wrong day changes the
	// sums and means.
	f := &fakeReader{at: func(key string, at time.Time) float32 {
		switch key {
		case domainIFS + "/" + VarPrecip:
			return float32(at.Day())
		case domainIFS + "/" + VarTemperature:
			return float32(10 * at.Day())
		}
		return nan32
	}}
	d, err := NewService(f).Daily(context.Background(), munich,
		time.Date(2020, 3, 2, 15, 0, 0, 0, time.UTC), time.Date(2020, 3, 4, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2020, 3, 2, 0, 0, 0, 0, time.UTC); !d.FirstDay.Equal(want) || len(d.PrecipSum) != 3 {
		t.Fatalf("days: first %v, %d days", d.FirstDay, len(d.PrecipSum))
	}
	ifs := d.Cells[0]
	shift := float64(float32(float32(float32(ifs.Elevation)-float32(munich.Elevation)) * ra.LapseRate))
	for i, dom := range []float64{2, 3, 4} {
		if d.PrecipSum[i] != 24*dom {
			t.Errorf("day %d: precipitation sum %v want %v", i, d.PrecipSum[i], 24*dom)
		}
		if want := 10*dom + shift; math.Abs(d.TMean[i]-want) > 1e-3 {
			t.Errorf("day %d: mean %v want %v", i, d.TMean[i], want)
		}
		// The day's intervals reach from 00:00 to the next midnight, whose
		// value already belongs to the next day: the step lies inside.
		if d.TMin[i] > 10*dom+shift+1e-3 || d.TMax[i] < 10*(dom+1)+shift-1e-3 {
			t.Errorf("day %d: extrema %v/%v", i, d.TMin[i], d.TMax[i])
		}
	}
}

// TestHourly_SkipsModelsWithoutFilesAndReadsTheRestAtOnce: before 2017 the
// IFS has no files; it must not be read, and ERA5-Land and ERA5 are read in
// the same round instead of one after the other.
func TestHourly_SkipsModelsWithoutFilesAndReadsTheRestAtOnce(t *testing.T) {
	uncovered := map[string]bool{}
	for _, v := range []string{VarTemperature, VarDewPoint, VarPrecip, varSnowfallWater, varWindU, VarCloud} {
		uncovered[domainIFS+"/"+v] = true
	}
	f := &fakeReader{uncovered: uncovered, values: map[string]float32{
		domainERA5Land + "/" + VarTemperature: 20, domainERA5Land + "/" + VarDewPoint: 10,
		domainERA5 + "/" + VarPrecip: 0.4, domainERA5 + "/" + VarCloud: 90,
		domainERA5 + "/" + varSnowfallWater: 0,
		domainERA5 + "/" + varWindU:         3, domainERA5 + "/" + varWindV: 4,
	}}
	h, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour),
		domain.VarTemperature, domain.VarWeatherCode, domain.VarWind)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range f.reads {
		if strings.HasPrefix(r, domainIFS+"/") {
			t.Errorf("read %s although the IFS has no file for the range", r)
		}
	}
	if f.didRead(domainERA5 + "/" + VarTemperature) {
		t.Error("ERA5 temperature read although ERA5-Land covers the range without gaps")
	}
	if got := h.Values[domain.VarWind][0]; math.Abs(got-18) > 1e-4 {
		t.Errorf("wind = %v km/h, want 18 (5 m/s from u=3, v=4)", got)
	}
}

// TestHourly_CompleteHighModelReadsNothingBelow: when the IFS covers the
// range and has no gaps, the lower models are never read.
func TestHourly_CompleteHighModelReadsNothingBelow(t *testing.T) {
	f := &fakeReader{values: map[string]float32{
		domainIFS + "/" + VarTemperature: 12, domainIFS + "/" + VarPrecip: 0.5,
	}}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour),
		domain.VarTemperature, domain.VarPrecip); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.reads {
		if !strings.HasPrefix(r, domainIFS+"/") {
			t.Errorf("read %s although the IFS filled every hour", r)
		}
	}
	if len(f.reads) != 2 {
		t.Errorf("reads %v, want each needed series exactly once", f.reads)
	}
}

// TestHourly_FailedLookupStillReads: an unknown coverage schedules the read
// instead of failing the query; only the read's own failure counts.
func TestHourly_FailedLookupStillReads(t *testing.T) {
	f := &fakeReader{coverageErr: errors.New("listing failed"), values: map[string]float32{domainIFS + "/" + VarTemperature: 12}}
	h, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature)
	if err != nil {
		t.Fatalf("a failed lookup alone must not fail the read: %v", err)
	}
	if h.Values[domain.VarTemperature][0] != h.Values[domain.VarTemperature][0] {
		t.Error("temperature missing although the IFS read succeeded")
	}
	f = &fakeReader{coverageErr: errors.New("listing failed"), err: errors.New("bucket down")}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature); err == nil {
		t.Error("a failed lookup and a failed read must fail the query")
	}
}

// TestHourly_LowerModelFailureIgnoredWhenHigherModelsComplete: IFS and
// ERA5-Land cover complementary hours; ERA5 is scheduled but not needed, so
// its read failure must not fail the query.
func TestHourly_LowerModelFailureIgnoredWhenHigherModelsComplete(t *testing.T) {
	half := day.Add(12 * time.Hour)
	f := &fakeReader{
		at: func(key string, at time.Time) float32 {
			switch {
			case key == domainIFS+"/"+VarTemperature && !at.Before(half):
				return 12
			case key == domainERA5Land+"/"+VarTemperature && at.Before(half):
				return 10
			}
			return nan32
		},
		err:    errors.New("bucket down"),
		failOn: domainERA5 + "/" + VarTemperature,
		// Each covers half the day, so ERA5 is scheduled in round 1.
		coverageHours: map[string]int{domainIFS + "/" + VarTemperature: 12, domainERA5Land + "/" + VarTemperature: 12},
	}
	h, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour), domain.VarTemperature)
	if err != nil {
		t.Fatalf("unneeded ERA5 failure failed the query: %v", err)
	}
	for i, v := range h.Values[domain.VarTemperature] {
		if v != v {
			t.Fatalf("hour %d has no value", i)
		}
	}
	if !f.didRead(domainERA5 + "/" + VarTemperature) {
		t.Error("the test needs ERA5 scheduled in round 1")
	}
}

func TestArchiveDirectories(t *testing.T) {
	dirs := ArchiveDirectories()
	want := map[string]bool{
		domainIFS + "/" + varWindU: true, domainIFS + "/" + varWindV: true,
		domainIFS + "/" + varSnowfallWater: true, domainERA5Land + "/" + VarTemperature: true,
		domainERA5 + "/" + VarCloud: true,
	}
	got := map[string]bool{}
	for _, d := range dirs {
		got[d] = true
	}
	for d := range want {
		if !got[d] {
			t.Errorf("missing %s in %v", d, dirs)
		}
	}
	if got[domainERA5Land+"/"+VarPrecip] {
		t.Error("ERA5-Land provides no precipitation")
	}
	// IFS and ERA5: 7 stored variables each (wind as u and v); ERA5-Land: 2.
	if len(dirs) != 16 {
		t.Errorf("%d directories, want 16", len(dirs))
	}
}

// TestHourly_CoverageErrorOfAnUnneededModelIsIgnored: the IFS covers the
// range without gaps, so a failed ERA5-Land listing must not fail the read.
func TestHourly_CoverageErrorOfAnUnneededModelIsIgnored(t *testing.T) {
	f := &fakeReader{
		values:        map[string]float32{domainIFS + "/" + VarTemperature: 12},
		coverageErr:   errors.New("listing failed"),
		coverageErrOn: domainERA5Land + "/" + VarTemperature,
	}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature); err != nil {
		t.Errorf("unneeded model's coverage error failed the read: %v", err)
	}
}

// TestHourly_HangingLookupOfAnUnneededModelDoesNotHold: the IFS covers the
// range; a lookup for ERA5-Land that never answers (a bucket outage) must
// not delay the read.
func TestHourly_HangingLookupOfAnUnneededModelDoesNotHold(t *testing.T) {
	f := &fakeReader{
		values:         map[string]float32{domainIFS + "/" + VarTemperature: 12},
		coverageHangOn: domainERA5Land + "/" + VarTemperature,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	if _, err := NewService(f).Hourly(ctx, munich, day, day.Add(time.Hour), domain.VarTemperature); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("read took %v, held by an unneeded lookup", d)
	}
}

// ifsGapAtFirstHour serves IFS temperature with a gap in the first hour and
// a constant elsewhere; every other model has a constant everywhere.
func ifsGapAtFirstHour(key string, at time.Time) float32 {
	if key == domainIFS+"/"+VarTemperature && at.Equal(day) {
		return nan32
	}
	return 12
}

// TestHourly_GapRoundSkipsModelsWithoutFiles: a gap in the IFS is filled
// from the next model that has files, not from one that has none.
func TestHourly_GapRoundSkipsModelsWithoutFiles(t *testing.T) {
	f := &fakeReader{at: ifsGapAtFirstHour, uncovered: map[string]bool{domainERA5Land + "/" + VarTemperature: true}}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour), domain.VarTemperature); err != nil {
		t.Fatal(err)
	}
	if f.didRead(domainERA5Land + "/" + VarTemperature) {
		t.Error("gap round read ERA5-Land although it has no files")
	}
	if !f.didRead(domainERA5 + "/" + VarTemperature) {
		t.Error("gap round did not read ERA5 to fill the IFS gap")
	}
}

// TestHourly_GapRoundTriesAModelWhoseLookupFailed: an unknown coverage is
// no reason to leave a gap; the read is attempted.
func TestHourly_GapRoundTriesAModelWhoseLookupFailed(t *testing.T) {
	f := &fakeReader{at: ifsGapAtFirstHour, coverageErr: errors.New("listing failed"), coverageErrOn: domainERA5Land + "/" + VarTemperature}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(24*time.Hour), domain.VarTemperature); err != nil {
		t.Fatal(err)
	}
	if !f.didRead(domainERA5Land + "/" + VarTemperature) {
		t.Error("gap round skipped ERA5-Land although only its lookup failed")
	}
}

// TestHourly_DewPointIsHeightCorrectedInEveryRound: like the temperature,
// the dew point is moved to the target height — from round 1 (IFS) and
// from a gap round (ERA5-Land fills the IFS gap).
func TestHourly_DewPointIsHeightCorrectedInEveryRound(t *testing.T) {
	f := &fakeReader{at: func(key string, at time.Time) float32 {
		switch key {
		case domainIFS + "/" + VarDewPoint:
			if at.Equal(day) {
				return nan32
			}
			return 10
		case domainERA5Land + "/" + VarDewPoint:
			return 8
		}
		return nan32
	}}
	// Far enough above the cells that no 100 m shortcut skips the correction.
	alp := domain.GeoPoint{Lat: munich.Lat, Lon: munich.Lon, Elevation: 900}
	h, err := NewService(f).Hourly(context.Background(), alp, day, day.Add(2*time.Hour), domain.VarDewPoint)
	if err != nil {
		t.Fatal(err)
	}
	shift := func(model int) float64 {
		return float64(float32(float32(float32(h.Cells[model].Elevation)-float32(alp.Elevation)) * ra.LapseRate))
	}
	if h.Cells[0].Elevation == alp.Elevation || h.Cells[1].Elevation == alp.Elevation {
		t.Fatalf("the test needs cells off the target height: %+v", h.Cells)
	}
	if got, want := h.Values[domain.VarDewPoint][1], 10+shift(0); math.Abs(got-want) > 1e-4 {
		t.Errorf("IFS dew point %v, want %v (height-corrected)", got, want)
	}
	if got, want := h.Values[domain.VarDewPoint][0], 8+shift(1); math.Abs(got-want) > 1e-4 {
		t.Errorf("ERA5-Land dew point in the gap %v, want %v (height-corrected)", got, want)
	}
}

// TestHourly_LeftBehindLookupsEndWithTheRead: an unneeded lookup that was
// left waiting is cancelled when the read returns, not when the caller's
// context ends.
func TestHourly_LeftBehindLookupsEndWithTheRead(t *testing.T) {
	f := &fakeReader{
		values:         map[string]float32{domainIFS + "/" + VarTemperature: 12},
		coverageHangOn: domainERA5Land + "/" + VarTemperature,
		hangEnded:      make(chan struct{}),
	}
	if _, err := NewService(f).Hourly(context.Background(), munich, day, day.Add(time.Hour), domain.VarTemperature); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.hangEnded:
	case <-time.After(5 * time.Second):
		t.Fatal("the left-behind lookup is still running after the read returned")
	}
}
