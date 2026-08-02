package domain

import (
	"math"
	"testing"
)

func TestThermicityIndex(t *testing.T) {
	// Coldest month = Jan (−5). T = (11×10 + (−5))/12 = 8.75. M=0, m=−10.
	// It = (8.75 + 0 − 10) × 10 = −12.5.
	c := MonthlyClimate{}
	for i := range c.Tmean {
		c.Tmean[i], c.Tmax[i], c.Tmin[i] = 10, 15, 5
	}
	c.Tmean[0], c.Tmax[0], c.Tmin[0] = -5, 0, -10
	if got := ThermicityIndex(c); math.Abs(got-(-12.5)) > 1e-9 {
		t.Errorf("It = %.3f, want -12.5", got)
	}
}

func TestPositiveTemperature(t *testing.T) {
	// positive months: 3+8×6+3 = 54 → Tp = 540.
	c := MonthlyClimate{Tmean: [12]float64{-5, -2, 3, 8, 8, 8, 8, 8, 8, 3, -2, -5}}
	if got := PositiveTemperature(c); math.Abs(got-540) > 1e-9 {
		t.Errorf("Tp = %.1f, want 540", got)
	}
}

func TestThermotypeHorizon(t *testing.T) {
	cases := []struct {
		tp   float64
		want string
	}{
		{2100, "thermo"}, {1600, "meso"}, {1000, "supra"}, {500, "oro"}, {200, "cryoro"},
	}
	for _, tc := range cases {
		if got := thermotypeHorizon(tc.tp); got != tc.want {
			t.Errorf("thermotypeHorizon(%.0f) = %q, want %q", tc.tp, got, tc.want)
		}
	}
}

func TestRivasMartinezThermotype_MediterraneanSuffix(t *testing.T) {
	// Dry-summer temperate climate (→ Köppen Cs*, Mediterranean macrobioclimate).
	c := MonthlyClimate{
		Tmean:  [12]float64{7, 8, 10, 13, 16, 19, 21, 20, 17, 13, 9, 7},
		Precip: [12]float64{80, 70, 60, 30, 15, 5, 3, 8, 25, 60, 80, 90},
	}
	for i := range c.Tmean {
		c.Tmin[i], c.Tmax[i] = c.Tmean[i]-5, c.Tmean[i]+5
	}
	_, _, en := RivasMartinezThermotype(c, 40)
	if !hasSuffix(en, "mediterranean") {
		t.Errorf("thermotype = %q, want a *mediterranean", en)
	}
}

func TestRivasMartinezThermotype_ColdPrefix(t *testing.T) {
	// Cold alpine-ish climate → low Tp → oro/cryoro prefix.
	c := MonthlyClimate{
		Tmean:  [12]float64{-8, -7, -4, 0, 3, 5, 6, 5, 3, -1, -5, -7},
		Precip: constPrecip(60),
	}
	for i := range c.Tmean {
		c.Tmin[i], c.Tmax[i] = c.Tmean[i]-4, c.Tmean[i]+4
	}
	code, _, _ := RivasMartinezThermotype(c, 46)
	if !hasPrefix(code, "oro") && !hasPrefix(code, "cryoro") {
		t.Errorf("thermotype = %q, want oro*/cryoro*", code)
	}
}

func hasSuffix(s, suf string) bool { return len(s) >= len(suf) && s[len(s)-len(suf):] == suf }
func hasPrefix(s, pre string) bool { return len(s) >= len(pre) && s[:len(pre)] == pre }
