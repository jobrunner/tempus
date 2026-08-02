package domain

import (
	"math"
	"testing"
	"time"
)

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want %.4f (±%.4f)", name, got, want, tol)
	}
}

// A fully controlled 12-month input with hand-computed BIO values.
func TestBioclimVariables(t *testing.T) {
	tmean := [12]float64{0, 0, 5, 10, 15, 20, 20, 20, 15, 10, 5, 0}
	var tmin, tmax [12]float64
	for i := range tmean {
		tmin[i] = tmean[i] - 5
		tmax[i] = tmean[i] + 5
	}
	precip := [12]float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120}

	b := BioclimVariables(MonthlyClimate{Tmin: tmin, Tmax: tmax, Tmean: tmean, Precip: precip})

	approx(t, "BIO1", b.Bio1, 10, 1e-6)
	approx(t, "BIO2", b.Bio2, 10, 1e-6)
	approx(t, "BIO3", b.Bio3, 33.3333, 1e-3)
	approx(t, "BIO4", b.Bio4, 763.79, 0.1)
	approx(t, "BIO5", b.Bio5, 25, 1e-6)
	approx(t, "BIO6", b.Bio6, -5, 1e-6)
	approx(t, "BIO7", b.Bio7, 30, 1e-6)
	approx(t, "BIO8", b.Bio8, 5, 1e-6)      // temp of wettest quarter (Oct-Dec)
	approx(t, "BIO9", b.Bio9, 1.6667, 1e-3) // temp of driest quarter (Jan-Mar)
	approx(t, "BIO10", b.Bio10, 20, 1e-6)
	approx(t, "BIO11", b.Bio11, 0, 1e-6)
	approx(t, "BIO12", b.Bio12, 780, 1e-6)
	approx(t, "BIO13", b.Bio13, 120, 1e-6)
	approx(t, "BIO14", b.Bio14, 10, 1e-6)
	approx(t, "BIO15", b.Bio15, 53.11, 0.1)
	approx(t, "BIO16", b.Bio16, 330, 1e-6) // wettest quarter Oct-Dec
	approx(t, "BIO17", b.Bio17, 60, 1e-6)  // driest quarter Jan-Mar
	approx(t, "BIO18", b.Bio18, 210, 1e-6) // precip of warmest quarter Jun-Aug
	approx(t, "BIO19", b.Bio19, 150, 1e-6) // precip of coldest quarter Dec-Feb
}

func TestNormalPeriod_Auto(t *testing.T) {
	cases := []struct {
		year               int
		wantStart, wantEnd int
	}{
		{2005, 1991, 2020},
		{2024, 1991, 2020},
		{1991, 1991, 2020},
		{1985, 1961, 1990},
		{1961, 1961, 1990},
		{1954, 1940, 1969}, // 1931-1960 clamped to ERA5 floor
		{1920, 1940, 1969}, // pre-ERA5 -> earliest available
	}
	for _, tc := range cases {
		start, end, err := NormalPeriod(time.Date(tc.year, 6, 1, 0, 0, 0, 0, time.UTC), "")
		if err != nil {
			t.Fatalf("year %d: %v", tc.year, err)
		}
		if start != tc.wantStart || end != tc.wantEnd {
			t.Errorf("year %d -> %d-%d, want %d-%d", tc.year, start, end, tc.wantStart, tc.wantEnd)
		}
	}
}

func TestNormalPeriod_Override(t *testing.T) {
	start, end, err := NormalPeriod(time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC), "1970-2000")
	if err != nil || start != 1970 || end != 2000 {
		t.Fatalf("override 1970-2000 -> %d-%d err=%v", start, end, err)
	}
	for _, bad := range []string{"2000", "1900-1930", "2000-1990", "abc-def", "1970/2000"} {
		if _, _, err := NormalPeriod(time.Now().UTC(), bad); err == nil {
			t.Errorf("expected error for override %q", bad)
		}
	}
}
