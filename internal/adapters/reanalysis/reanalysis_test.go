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
}

func (f *fakeReader) ReadSeries(_ context.Context, model, variable string, _ []uint64, from, to time.Time) ([]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, model+"/"+variable)
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.values[model+"/"+variable]
	out := make([]float32, int(to.Sub(from)/time.Hour))
	for i := range out {
		out[i] = nan32
		if ok {
			out[i] = v
		}
	}
	return out, nil
}

func (f *fakeReader) didRead(key string) bool {
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
	temp := h.Values[domain.VarTemperature][0]
	if temp > 20.5 || temp < 19.5 {
		t.Errorf("temperature from ERA5-Land, height-corrected: %v", temp)
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
