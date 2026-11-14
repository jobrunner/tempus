package domain

import (
	"math"
	"testing"
)

// Synthetic boundary tests for the WBCS08 key 5.1 ("Clave para los
// macrobioclimas"). Each case changes only the index under test from a base
// whose other steps cannot decide; the source wording behind every bound is
// quoted in docs/research/rivas-martinez-reference.md.

// wetIx has no summer drought (Ios > 2), equal half-year rain and a mild,
// oceanic temperature regime: it ends in "temperate" unless a step fires.
func wetIx() rmIndices {
	return rmIndices{
		T: 10, Ic: 15, It: 200, Itc: 200, Tp: 1200, Io: 5, m: 0, M: 5, Tps: 500, Tmax: 18,
		Ios2: 5, Ios3: 5, Ios4: 5, Pss: 300, Psw: 300, Pcm1: 100, Pcm2: 100, Pcm3: 100,
	}
}

// dryIx is wetIx with a Mediterranean summer drought, a wetter cold half-year
// and no warm-season rain maximum: it ends in "mediterranean".
func dryIx() rmIndices {
	ix := wetIx()
	ix.Ios2, ix.Ios3, ix.Ios4 = 0.5, 0.5, 0.5
	ix.Pss, ix.Psw = 100, 300
	ix.Pcm1 = 50
	return ix
}

func at(lat float64) RivasMartinezSite {
	return RivasMartinezSite{Lat: lat, ElevationKnown: true}
}

func atElev(lat, elev float64) RivasMartinezSite {
	return RivasMartinezSite{Lat: lat, ElevationM: elev, ElevationKnown: true}
}

type macroCase struct {
	name string
	ix   rmIndices
	site RivasMartinezSite
	want Macrobioclimate
}

func runMacroCases(t *testing.T, cases []macroCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := macrobioclimateFrom(tc.ix, tc.site); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}

func with(ix rmIndices, f func(*rmIndices)) rmIndices {
	f(&ix)
	return ix
}

// Steps 1–2: "Territorios intertropicales entre los paralelos 23º N y S" →
// tropical; "entre los paralelos 23° y 35° N y S" → subtropical steps 3–10;
// "al N y S del paralelo 35°" → steps 11–14.
func TestMacrobioclimate_LatitudeBelts(t *testing.T) {
	step4 := with(wetIx(), func(ix *rmIndices) { ix.T, ix.m = 26, 11 })
	runMacroCases(t, []macroCase{
		{"22.99 N is tropical whatever the climate", wetIx(), at(22.99), MacroTropical},
		{"22.99 S is tropical", wetIx(), at(-22.99), MacroTropical},
		{"23 N leaves the tropical belt", wetIx(), at(23), MacroTemperate},
		{"35 N is still subtropical (step 4 applies)", step4, at(35), MacroTropical},
		{"35 S is still subtropical", step4, at(-35), MacroTropical},
		{"35.01 N is extratropical (step 4 does not apply)", step4, at(35.01), MacroTemperate},
	})
}

// Step 3: "Eurasia: 26 a 35° N, longitud 70° E a 120° E, altitud >= 2.000 m"
// → step 10 (temperate/mediterranean by summer drought), skipping step 4.
func TestMacrobioclimate_Step3HighAsia(t *testing.T) {
	ix := with(wetIx(), func(ix *rmIndices) { ix.T, ix.m = 26, 11 }) // step 4 would say tropical
	site := func(lat, lon, elev float64) RivasMartinezSite {
		return RivasMartinezSite{Lat: lat, Lon: lon, ElevationM: elev, ElevationKnown: true}
	}
	unknown := RivasMartinezSite{Lat: 30, Lon: 90, ElevationM: 3000}
	runMacroCases(t, []macroCase{
		{"2000 m qualifies", ix, site(30, 90, 2000), MacroTemperate},
		{"1999.9 m does not", ix, site(30, 90, 1999.9), MacroTropical},
		{"26 N qualifies", ix, site(26, 90, 3000), MacroTemperate},
		{"25.99 N does not", ix, site(25.99, 90, 3000), MacroTropical},
		{"35 N qualifies", ix, site(35, 90, 3000), MacroTemperate},
		{"70 E qualifies", ix, site(30, 70, 3000), MacroTemperate},
		{"69.99 E does not", ix, site(30, 69.99, 3000), MacroTropical},
		{"120 E qualifies", ix, site(30, 120, 3000), MacroTemperate},
		{"120.01 E does not", ix, site(30, 120.01, 3000), MacroTropical},
		{"southern hemisphere does not", ix, site(-30, 90, 3000), MacroTropical},
		{"unknown elevation does not", ix, unknown, MacroTropical},
		{"step 10 with summer drought is mediterranean",
			with(dryIx(), func(ix *rmIndices) { ix.T, ix.m = 26, 11 }), site(30, 90, 3000), MacroMediterranean},
	})
}

// Step 4: two of "T >= 25°, m >= 10°, It, Itc >= 580" → tropical. The dry base
// otherwise ends mediterranean (steps 5→8 or 6→7→9).
func TestMacrobioclimate_Step4(t *testing.T) {
	d := func(f func(*rmIndices)) rmIndices { return with(dryIx(), f) }
	runMacroCases(t, []macroCase{
		{"T 25 and m 10", d(func(ix *rmIndices) { ix.T, ix.m = 25, 10 }), at(30), MacroTropical},
		{"T 24.99 and m 10", d(func(ix *rmIndices) { ix.T, ix.m = 24.99, 10 }), at(30), MacroMediterranean},
		{"T 25 and m 9.99", d(func(ix *rmIndices) { ix.T, ix.m = 25, 9.99 }), at(30), MacroMediterranean},
		{"T 25 and Itc 580", d(func(ix *rmIndices) { ix.T, ix.Itc = 25, 580 }), at(30), MacroTropical},
		{"T 25 and Itc 579.99", d(func(ix *rmIndices) { ix.T, ix.Itc = 25, 579.99 }), at(30), MacroMediterranean},
		{"m 10 and Itc 580", d(func(ix *rmIndices) { ix.m, ix.Itc = 10, 580 }), at(30), MacroTropical},
		{"one criterion is not enough", d(func(ix *rmIndices) { ix.Itc = 900 }), at(30), MacroMediterranean},
	})
}

// Step 5, reduced to 200 m (footnote (1): +0.6 °C T, +0.5 °C M, +13 It/Itc per
// 100 m above 200 m): two of "T > 21°, M > 18°, It, Itc >= 470" → step 6,
// otherwise step 8. The wet base makes step 6/7 say tropical and step 8 say
// temperate.
func TestMacrobioclimate_Step5(t *testing.T) {
	w := func(f func(*rmIndices)) rmIndices { return with(wetIx(), f) }
	runMacroCases(t, []macroCase{
		{"T 21.01 and M 18.01", w(func(ix *rmIndices) { ix.T, ix.M = 21.01, 18.01 }), at(30), MacroTropical},
		{"T exactly 21", w(func(ix *rmIndices) { ix.T, ix.M = 21, 18.01 }), at(30), MacroTemperate},
		{"M exactly 18", w(func(ix *rmIndices) { ix.T, ix.M = 21.01, 18 }), at(30), MacroTemperate},
		{"T 21.01 and Itc 470", w(func(ix *rmIndices) { ix.T, ix.Itc = 21.01, 470 }), at(30), MacroTropical},
		{"T 21.01 and Itc 469.99", w(func(ix *rmIndices) { ix.T, ix.Itc = 21.01, 469.99 }), at(30), MacroTemperate},
		{"T reduced to 200 m (+0.6/100 m)", w(func(ix *rmIndices) { ix.T, ix.M = 20.5, 18.01 }), atElev(30, 300), MacroTropical},
		{"no reduction at 200 m", w(func(ix *rmIndices) { ix.T, ix.M = 20.5, 18.01 }), atElev(30, 200), MacroTemperate},
		{"M reduced to 200 m (+0.5/100 m)", w(func(ix *rmIndices) { ix.T, ix.M = 21.01, 17.6 }), atElev(30, 300), MacroTropical},
		{"Itc reduced to 200 m (+13/100 m)", w(func(ix *rmIndices) { ix.T, ix.Itc = 21.01, 460 }), atElev(30, 300), MacroTropical},
		{"Itc reduction is proportional", w(func(ix *rmIndices) { ix.T, ix.Itc = 21.01, 460 }), atElev(30, 250), MacroTemperate},
		{"unknown elevation: no reduction", w(func(ix *rmIndices) { ix.T, ix.M = 20.5, 18.01 }),
			RivasMartinezSite{Lat: 30, ElevationM: 3000}, MacroTemperate},
	})
}

// Steps 6, 7, 9: "Pss > Psw" → tropical; else "Ios2 > 2 o … Iosc > 2" →
// tropical; else "Pcm3 < Pcm1 > Pcm2" → tropical, otherwise mediterranean.
func TestMacrobioclimate_Steps6to9(t *testing.T) {
	warm := func(base rmIndices, f func(*rmIndices)) rmIndices {
		return with(base, func(ix *rmIndices) { ix.T, ix.M = 21.01, 18.01; f(ix) })
	}
	none := func(*rmIndices) {}
	runMacroCases(t, []macroCase{
		{"Pss > Psw", warm(dryIx(), func(ix *rmIndices) { ix.Pss, ix.Psw = 300.01, 300 }), at(30), MacroTropical},
		{"Pss = Psw falls through", warm(dryIx(), func(ix *rmIndices) { ix.Pss, ix.Psw = 300, 300 }), at(30), MacroMediterranean},
		{"no summer drought", warm(wetIx(), func(ix *rmIndices) { ix.Pss, ix.Psw = 100, 300 }), at(30), MacroTropical},
		{"warm four-month rain maximum", warm(dryIx(), func(ix *rmIndices) { ix.Pcm1 = 100.01 }), at(30), MacroTropical},
		{"Pcm1 = Pcm2 is not a maximum", warm(dryIx(), func(ix *rmIndices) { ix.Pcm1, ix.Pcm3 = 100, 50 }), at(30), MacroMediterranean},
		{"Pcm1 = Pcm3 is not a maximum", warm(dryIx(), func(ix *rmIndices) { ix.Pcm1, ix.Pcm2 = 100, 50 }), at(30), MacroMediterranean},
		{"otherwise mediterranean", warm(dryIx(), none), at(30), MacroMediterranean},
	})
}

// Steps 8 and 14: "Ios2 > 2 o … Iosc > 2" → temperate, otherwise mediterranean.
func TestMacrobioclimate_Steps8and14(t *testing.T) {
	runMacroCases(t, []macroCase{
		{"subtropical wet", wetIx(), at(30), MacroTemperate},
		{"subtropical dry", dryIx(), at(30), MacroMediterranean},
		{"extratropical wet", wetIx(), at(45), MacroTemperate},
		{"extratropical dry", dryIx(), at(45), MacroMediterranean},
	})
}

// Step 11: "latitud Norte > 72º o … Sur > 56º" → polar; otherwise, at 200 m,
// "Tp < 380" → polar. Footnote (1): north of 48° N / south of 51° S add 0.4 °C
// to T and 12 to Tp per 100 m; elsewhere T +0.6 °C, and Tp +55 (the only other
// Tp rate the footnote gives).
func TestMacrobioclimate_Step11Polar(t *testing.T) {
	tp := func(v float64) rmIndices { return with(wetIx(), func(ix *rmIndices) { ix.Tp = v }) }
	runMacroCases(t, []macroCase{
		{"72 N is not polar by latitude", wetIx(), at(72), MacroTemperate},
		{"72.01 N is polar", wetIx(), at(72.01), MacroPolar},
		{"56 S is not polar by latitude", wetIx(), at(-56), MacroTemperate},
		{"56.01 S is polar", wetIx(), at(-56.01), MacroPolar},
		{"Tp 379.99 is polar", tp(379.99), at(45), MacroPolar},
		{"Tp exactly 380 is not polar", tp(380), at(45), MacroTemperate},
		{"Tp +55/100 m up to 48 N", tp(330), atElev(48, 300), MacroTemperate},
		{"Tp +12/100 m north of 48 N", tp(330), atElev(48.01, 300), MacroPolar},
		{"Tp +55/100 m up to 51 S", tp(330), atElev(-51, 300), MacroTemperate},
		{"Tp +12/100 m south of 51 S", tp(330), atElev(-51.01, 300), MacroPolar},
		{"no reduction at exactly 200 m", tp(379.99), atElev(45, 200), MacroPolar},
		{"unknown elevation: no reduction", tp(379.99), RivasMartinezSite{Lat: 45, ElevationM: 3000}, MacroPolar},
	})
}

// Step 12: "Ic < 11, … Tps <= 290, y … Tmax <= 10º" → boreal.
func TestMacrobioclimate_Step12(t *testing.T) {
	b := func(ic, tps, tmax float64) rmIndices {
		return with(wetIx(), func(ix *rmIndices) { ix.Ic, ix.Tps, ix.Tmax = ic, tps, tmax })
	}
	runMacroCases(t, []macroCase{
		{"all three met", b(10.99, 290, 10), at(60), MacroBoreal},
		{"Ic exactly 11", b(11, 290, 10), at(60), MacroTemperate},
		{"Tps 290.01", b(10.99, 290.01, 10), at(60), MacroTemperate},
		{"Tmax 10.01", b(10.99, 290, 10.01), at(60), MacroTemperate},
	})
}

// Step 13 (values at 200 m): "Ic < 21, T < 5.3º y Tp < 720; … Ic = 21-28, T <
// 4.8º y Tp < 740; … Ic > 28, T < 3.8º y Tp < 800" → boreal.
func TestMacrobioclimate_Step13(t *testing.T) {
	b := func(ic, tm, tp float64) rmIndices {
		return with(wetIx(), func(ix *rmIndices) { ix.Ic, ix.T, ix.Tp = ic, tm, tp })
	}
	runMacroCases(t, []macroCase{
		{"Ic 20.99: T 5.29, Tp 719.99", b(20.99, 5.29, 719.99), at(60), MacroBoreal},
		{"Ic 20.99: T exactly 5.3", b(20.99, 5.3, 600), at(60), MacroTemperate},
		{"Ic 20.99: Tp exactly 720", b(20.99, 4, 720), at(60), MacroTemperate},
		{"Ic 21 uses the 21–28 row", b(21, 5.29, 600), at(60), MacroTemperate},
		{"Ic 21: T 4.79, Tp 739.99", b(21, 4.79, 739.99), at(60), MacroBoreal},
		{"Ic 28: T 4.79, Tp 739.99", b(28, 4.79, 739.99), at(60), MacroBoreal},
		{"Ic 28: T exactly 4.8", b(28, 4.8, 600), at(60), MacroTemperate},
		{"Ic 28: Tp exactly 740", b(28, 4, 740), at(60), MacroTemperate},
		{"Ic 28.01 uses the > 28 row", b(28.01, 4.79, 600), at(60), MacroTemperate},
		{"Ic 28.01: T 3.79, Tp 799.99", b(28.01, 3.79, 799.99), at(60), MacroBoreal},
		{"Ic 28.01: Tp exactly 800", b(28.01, 3.79, 800), at(60), MacroTemperate},
		{"T +0.4/100 m north of 48 N (4.85 → 5.25)", b(15, 4.85, 600), atElev(60, 300), MacroBoreal},
		{"T +0.6/100 m up to 48 N (4.75 → 5.35)", b(15, 4.75, 600), atElev(45, 300), MacroTemperate},
		{"T 4.75 at 200 m stays boreal at 45 N", b(15, 4.75, 600), atElev(45, 200), MacroBoreal},
	})
}

// TestIosLimits pins P14 Table 2 (after RM2011): the Ios2 and Ios3 limits per
// annual Io range; "–" cells are the uncompensated 2.0.
func TestIosLimits(t *testing.T) {
	cases := []struct{ io, lim2, lim3 float64 }{
		{0, 2.0, 2.0}, {2.0, 2.0, 2.0},
		{2.0001, 1.9, 2.0}, {2.8, 1.9, 2.0},
		{2.8001, 1.8, 1.9}, {3.6, 1.8, 1.9}, {4.8, 1.8, 1.9},
		{4.8001, 1.7, 1.9}, {6.0, 1.7, 1.9},
		{6.0001, 1.5, 1.8}, {7.0, 1.5, 1.8},
		{7.0001, 1.4, 1.8}, {8.0, 1.4, 1.8},
		{8.0001, 1.3, 1.8}, {9.0, 1.3, 1.8},
		{9.0001, 1.2, 1.8}, {10.0, 1.2, 1.8},
		{10.0001, 1.1, 1.7}, {11.0, 1.1, 1.7},
		{11.0001, 1.0, 1.7}, {12.0, 1.0, 1.7},
		{12.0001, 0.9, 1.7}, {50, 0.9, 1.7},
		{math.NaN(), 2.0, 2.0}, // no Io: no compensation
	}
	for _, tc := range cases {
		if l2, l3 := iosLimits(tc.io); l2 != tc.lim2 || l3 != tc.lim3 {
			t.Errorf("iosLimits(%g) = %g, %g; want %g, %g", tc.io, l2, l3, tc.lim2, tc.lim3)
		}
	}
}

// TestMediterraneanSummer pins the P14 Table 2 procedure: "Ios2 values >2
// always identify the temperate macrobioclimate … Ios2 values lower than the
// value indicated … mediterranean … between 2 and the value …: if Ios3 is
// lower than those shown …, mediterranean, otherwise if it is >2, temperate,
// and if Ios3 is between … consider Ios4. If this is <2, … mediterranean, if
// not … temperate." Io 5 selects the row Ios2 ≥ 1.7, Ios3 ≥ 1.9.
func TestMediterraneanSummer(t *testing.T) {
	cases := []struct {
		name             string
		ios2, ios3, ios4 float64
		want             bool
	}{
		{"Ios2 2.01 is never mediterranean", 2.01, 0, 0, false},
		{"Ios2 1.69 is mediterranean", 1.69, 5, 5, true},
		{"Ios2 at its limit, Ios3 below its limit", 1.7, 1.89, 5, true},
		{"Ios3 above 2", 1.7, 2.01, 0, false},
		{"Ios3 at its limit, Ios4 below 2", 1.7, 1.9, 1.99, true},
		{"Ios3 at its limit, Ios4 exactly 2", 1.7, 1.9, 2.0, false},
		{"Ios2 exactly 2, Ios3 exactly 2, Ios4 1.99", 2.0, 2.0, 1.99, true},
		{"no positive summer temperature", math.Inf(1), math.Inf(1), math.Inf(1), false},
	}
	for _, tc := range cases {
		if got := mediterraneanSummer(5, tc.ios2, tc.ios3, tc.ios4); got != tc.want {
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestItcCompensation pins WBCS08 Fig. 7.
func TestItcCompensation(t *testing.T) {
	cases := []struct{ ic, want float64 }{
		{0, -80}, {7.9, -1}, {8, 0}, {12, 0}, {18, 0}, {18.5, 2.5}, {21, 15},
		{21.5, 22.5}, {28, 120}, {30, 170}, {46, 570}, {50, 690},
	}
	for _, tc := range cases {
		if got := itcCompensation(tc.ic); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("itcCompensation(%g) = %g, want %g", tc.ic, got, tc.want)
		}
	}
}

// TestRMIndices_SouthernHemisphere uses Hobart: the summer quarter is DJF
// (WBCS08 ch. 2c), JF is its warmer pair, Ios4 adds November; the warmer
// half-year is Oct–Mar and the warmest four-month run starts in December.
func TestRMIndices_SouthernHemisphere(t *testing.T) {
	var s rmStation
	for _, x := range rmReferenceStations {
		if x.wmo == "94970" {
			s = x
		}
	}
	ix := computeRMIndices(s.climate(), s.lat)
	checks := []struct {
		name      string
		got, want float64
	}{
		{"Ios2", ix.Ios2, (43.7 + 37.8) / (17.9 + 17.6)},
		{"Ios3", ix.Ios3, (53.7 + 43.7 + 37.8) / (16.4 + 17.9 + 17.6)},
		{"Ios4", ix.Ios4, (49.4 + 53.7 + 43.7 + 37.8) / (14.7 + 16.4 + 17.9 + 17.6)},
		{"Tps", ix.Tps, 519},
		{"Pss", ix.Pss, 52.8 + 49.4 + 53.7 + 43.7 + 37.8 + 37},
		{"Psw", ix.Psw, 42.6 + 39.2 + 46 + 44.5 + 63 + 55.6},
		{"Pcm1", ix.Pcm1, 53.7 + 43.7 + 37.8 + 37},
		{"Pcm2", ix.Pcm2, 42.6 + 39.2 + 46 + 44.5},
		{"Pcm3", ix.Pcm3, 63 + 55.6 + 52.8 + 49.4},
		{"Itc (Ic 8.9: no compensation)", ix.Itc, ix.It},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %.4f, want %.4f", c.name, c.got, c.want)
		}
	}
}

// TestRMIndices_Edges covers the ombrothermic edge cases: months at or below
// 0 °C count neither P nor T (Io = Pp/Tp, WBCS08 ch. 2a), a summer without a
// positive month cannot be dry (P ≤ 2T needs T > 0), the earlier pair wins a
// tie, and the tropics get no Itc compensation (WBCS08 ch. 2d).
func TestRMIndices_Edges(t *testing.T) {
	cold := MonthlyClimate{Tmean: constPrecip(-5), Precip: constPrecip(10)}
	ix := computeRMIndices(cold, 70)
	if !math.IsInf(ix.Ios2, 1) || !math.IsInf(ix.Ios3, 1) || !math.IsInf(ix.Ios4, 1) || ix.Io != 0 || ix.Tps != 0 {
		t.Errorf("all-frost climate: Ios2/3/4 = %v/%v/%v Io %v Tps %v, want +Inf and 0", ix.Ios2, ix.Ios3, ix.Ios4, ix.Io, ix.Tps)
	}

	c := MonthlyClimate{
		Tmean:  [12]float64{-10, -8, -5, -2, -1, 10, 10, 10, 5, 0, -4, -8},
		Precip: [12]float64{10, 10, 10, 10, 100, 20, 40, 60, 10, 10, 10, 10},
	}
	ix = computeRMIndices(c, 50)
	if want := 60.0 / 20; math.Abs(ix.Ios2-want) > 1e-9 {
		t.Errorf("tie JJ = JA: Ios2 = %v, want %v (JJ)", ix.Ios2, want)
	}
	if math.Abs(ix.Ios4-ix.Ios3) > 1e-9 {
		t.Errorf("frozen May must not count: Ios4 = %v, Ios3 = %v", ix.Ios4, ix.Ios3)
	}
	if want := 130.0 / 350 * 10; math.Abs(ix.Io-want) > 1e-9 {
		t.Errorf("Io = %v, want %v (positive months only)", ix.Io, want)
	}

	trop := MonthlyClimate{Tmean: [12]float64{0, 2, 5, 10, 15, 20, 30, 25, 20, 15, 10, 5}}
	if ix := computeRMIndices(trop, 10); ix.Itc != ix.It {
		t.Errorf("tropics: Itc = %v, want It = %v", ix.Itc, ix.It)
	}
	if ix := computeRMIndices(trop, 23); ix.Itc == ix.It {
		t.Errorf("23° with Ic 30: Itc = It = %v, want compensated", ix.Itc)
	}
}
