package omstore

import (
	"context"
	"math"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestLive_ThirtyYears reads 1991–2020 hourly temperature for one ERA5 cell
// from the real bucket and reports traffic. Opt-in: TEMPUS_OM_LIVE=1.
func TestLive_ThirtyYears(t *testing.T) {
	if os.Getenv("TEMPUS_OM_LIVE") != "1" {
		t.Skip("set TEMPUS_OM_LIVE=1 to read from the public bucket")
	}
	s := New(Options{BaseURL: "https://openmeteo.s3.amazonaws.com", HTTPClient: &http.Client{Timeout: 30 * time.Second}, MaxParallel: 32})
	from := time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, run := range []string{"cold", "warm"} {
		start := time.Now()
		got, err := s.ReadSeries(context.Background(), "copernicus_era5", "temperature_2m", []uint64{550, 764}, from, to)
		if err != nil {
			t.Fatal(err)
		}
		nan := 0
		for _, v := range got {
			if math.IsNaN(float64(v)) {
				nan++
			}
		}
		req, bytes := s.Stats().Snapshot()
		t.Logf("%s: %d hours (%d NaN) in %v; cumulative %d requests, %.1f kB", run, len(got), nan, time.Since(start), req, float64(bytes)/1e3)
	}
}
