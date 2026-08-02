package domain

import (
	"math"
	"testing"
	"time"
)

func TestGrowingDegreeDays(t *testing.T) {
	// Day 1: (20+10)/2 - 10 = 5; day 2: (18+12)/2 - 10 = 5; day 3: (9+5)/2 - 10 = -3 -> 0.
	tmin := []float64{10, 12, 5}
	tmax := []float64{20, 18, 9}
	gdd, days := GrowingDegreeDays(tmin, tmax, 10)
	if math.Abs(gdd-10) > 1e-9 {
		t.Errorf("gdd = %.3f, want 10", gdd)
	}
	if days != 3 {
		t.Errorf("days = %d, want 3", days)
	}
}

func TestGrowingDegreeDays_MismatchedLengthUsesShorter(t *testing.T) {
	gdd, days := GrowingDegreeDays([]float64{10, 10}, []float64{30}, 10)
	if days != 1 {
		t.Errorf("days = %d, want 1", days)
	}
	if math.Abs(gdd-10) > 1e-9 { // (30+10)/2 - 10 = 10
		t.Errorf("gdd = %.3f, want 10", gdd)
	}
}

func TestGDDStartDate(t *testing.T) {
	cases := []struct {
		name    string
		instant time.Time
		lat     float64
		want    time.Time
	}{
		{"northern", time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC), 49, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"northern late", time.Date(2025, 11, 2, 0, 0, 0, 0, time.UTC), 52, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"southern before jul", time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC), -33, time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"southern after jul", time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC), -33, time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"southern on jul 1", time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC), -33, time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GDDStartDate(tc.instant, tc.lat); !got.Equal(tc.want) {
				t.Errorf("GDDStartDate = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMinMaxMean(t *testing.T) {
	mn, mx, mean, ok := MinMaxMean([]float64{3, -1, 5, 1})
	if !ok || mn != -1 || mx != 5 || math.Abs(mean-2) > 1e-9 {
		t.Errorf("MinMaxMean = (%.1f,%.1f,%.2f,%v), want (-1,5,2,true)", mn, mx, mean, ok)
	}
	if _, _, _, ok := MinMaxMean(nil); ok {
		t.Error("MinMaxMean(nil) ok = true, want false")
	}
}
