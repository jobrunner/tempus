package domain

import "testing"

// Regression tests for float roundoff at the Rivas-Martínez bounds. Monthly
// normals carry one decimal; their sums and means are exact in decimal but not
// in binary, so a value that sits exactly on a bound can land on either side
// of it (the same class as the Köppen 70 % case; see roundoffTolerance). Each
// input below is exactly on its bound in decimal and was picked so that the
// naive float computation lands on the wrong side. The expected class is the
// one the source's inequality gives for the exact decimal value.

// Craiova, WMO 15450 (RO), 192 m, WMO 1991–2020 normals (NCEI 0253808). The
// positive months sum to exactly 140.0 °C → Tp = 1400; the float sum is
// 1400.0000000000002. Ic 24.2 ≥ 21 → Tp decides; temperate Tp 800–1400 is
// supratemperate (Fig. 19; P14 Table 7: mesotemperate starts at 1401).
func TestThermotype_CraiovaTpExactlyOnTheBound(t *testing.T) {
	c := MonthlyClimate{
		Tmean:  [12]float64{-0.9, 1.3, 6.2, 12, 17.2, 21.3, 23.3, 23.1, 17.8, 11.7, 5.8, 0.3},
		Tmin:   [12]float64{-4.1, -2.3, 1.7, 6.7, 11.4, 15.1, 16.9, 16.9, 12.4, 7.4, 2.6, -2.5},
		Tmax:   [12]float64{3, 6.2, 12.1, 18.2, 23.6, 27.8, 30.2, 30.3, 24.7, 17.6, 9.9, 3.9},
		Precip: [12]float64{42.5, 33.9, 45.9, 48.8, 67.6, 74.5, 73.3, 47.5, 52.5, 51.9, 48.4, 50.2},
	}
	site := RivasMartinezSite{Lat: 44.31, Lon: 23.867, ElevationM: 192, ElevationKnown: true}
	got := RivasMartinez(c, site)
	if got.Macrobioclimate != MacroTemperate || got.Thermotype.Code != "supratemperate" || got.Thermotype.Index != indexTp {
		t.Errorf("Craiova = %s / %s (%s %.13f), want temperate / supratemperate (Tp 1400)",
			got.Macrobioclimate, got.Thermotype.Code, got.Thermotype.Index, got.Thermotype.Value)
	}
}

// TestRoundoff_SyntheticBounds places one-decimal inputs exactly on a ladder
// bound, both index-selection bounds and a key threshold.
func TestRoundoff_SyntheticBounds(t *testing.T) {
	t.Run("Tp ladder bound 1400 (float 1400.0000000000002)", func(t *testing.T) {
		// Exact sum 140.0; every month above 0 °C. Ic 28.2 ≥ 21 → Tp.
		c := MonthlyClimate{Tmean: [12]float64{2.9, 0.5, 1.8, 27.6, 17.4, 17.1, 1.0, 0.5, 28.7, 10.9, 24.1, 7.5}}
		got := thermotypeFor(computeRMIndices(c, 45), MacroTemperate, 45)
		if got.Code != "supratemperate" {
			t.Errorf("= %s (Tp %.13f), want supratemperate", got.Code, got.Value)
		}
	})
	t.Run("It exactly 120 keeps the It/Itc ladder (float 119.99999999999999)", func(t *testing.T) {
		// mean(T) = 12.0 exactly, coldest month M = 4.4, m = −4.4 → It = 120;
		// Ic 10.7 (8–18) → Itc = It.
		c := MonthlyClimate{Tmean: [12]float64{12.0, 14.1, 14.5, 10.1, 4.3, 15.0, 14.6, 12.2, 13.6, 6.3, 12.5, 14.8}}
		c.Tmin, c.Tmax = c.Tmean, c.Tmean
		c.Tmin[4], c.Tmax[4] = -4.4, 4.4
		got := thermotypeFor(computeRMIndices(c, 45), MacroTemperate, 45)
		if got.Index != indexItc || got.Code != "supratemperate" {
			t.Errorf("= %s via %s %.14f, want supratemperate via Itc", got.Code, got.Index, got.Value)
		}
	})
	t.Run("Ic exactly 21 switches to Tp (float 20.999999999999996)", func(t *testing.T) {
		// Warmest 32.3, coldest 11.3 → Ic = 21.0 → Tp decides.
		c := MonthlyClimate{Tmean: [12]float64{11.3, 13, 17, 22, 27, 31, 32.3, 31, 27, 22, 17, 13}}
		c.Tmin, c.Tmax = c.Tmean, c.Tmean
		got := thermotypeFor(computeRMIndices(c, 30), MacroMediterranean, 30)
		if got.Index != indexTp {
			t.Errorf("= %s via %s, want Tp (Ic ≥ 21)", got.Code, got.Index)
		}
	})
	t.Run("key step 13: T exactly 3.8 is not boreal (float 3.7999999999999994)", func(t *testing.T) {
		// Ic 29.2 > 28 → boreal needs T < 3.8 and Tp < 800 (Tp 769). Wet
		// summers → step 14 temperate.
		c := MonthlyClimate{
			Tmean:  [12]float64{-12.5, -9.9, -3.7, 3.3, 9.4, 13.8, 16.7, 16.0, 12.2, 5.5, -3.0, -2.2},
			Precip: constPrecip(80),
		}
		if got := RivasMartinezMacrobioclimate(c, RivasMartinezSite{Lat: 60, ElevationKnown: true}); got != MacroTemperate {
			t.Errorf("= %s, want temperate", got)
		}
	})
}
