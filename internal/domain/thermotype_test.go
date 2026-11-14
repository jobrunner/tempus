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

// TestThermotype_IndexSelection pins which index decides the thermotype.
// WBCS08 ch. 3 ("a cualquier latitud, cuando … It es inferior a 120 o cuando
// … Ic es igual o superior a 21 … se utiliza … Tp") and P14 Table 7 ("Tp is
// used if Ic ≥ 21 or It, Itc < 120"); Itc replaces It outside 23° N/S (WBCS08
// ch. 2d); boreal and polar have Tp columns only (Fig. 19).
func TestThermotype_IndexSelection(t *testing.T) {
	cases := []struct {
		name      string
		mb        Macrobioclimate
		lat       float64
		ix        rmIndices
		wantIndex string
		wantCode  string
	}{
		{"Itc exactly 120 keeps Itc", MacroTemperate, 45, rmIndices{Ic: 15, It: 0, Itc: 120, Tp: 1500}, "Itc", "supratemperate"},
		{"Itc 119.99 switches to Tp", MacroTemperate, 45, rmIndices{Ic: 15, It: 0, Itc: 119.99, Tp: 1500}, "Tp", "mesotemperate"},
		{"Ic 20.99 keeps Itc", MacroTemperate, 45, rmIndices{Ic: 20.99, Itc: 300, Tp: 1500}, "Itc", "thermotemperate"},
		{"Ic exactly 21 switches to Tp", MacroTemperate, 45, rmIndices{Ic: 21, Itc: 300, Tp: 1500}, "Tp", "mesotemperate"},
		{"inside 23° the uncompensated It decides", MacroTropical, 22.99, rmIndices{Ic: 5, It: 500, Itc: 100, Tp: 2500}, "It", "thermotropical"},
		{"at 23° Itc decides", MacroTropical, -23, rmIndices{Ic: 5, It: 100, Itc: 500, Tp: 2500}, "Itc", "thermotropical"},
		{"tropics with Ic ≥ 21 use Tp", MacroTropical, 10, rmIndices{Ic: 21, It: 500, Tp: 2500}, "Tp", "thermotropical"},
		{"boreal always uses Tp", MacroBoreal, 60, rmIndices{Ic: 15, Itc: 500, Tp: 500}, "Tp", "supraboreal"},
		{"polar always uses Tp", MacroPolar, 75, rmIndices{Ic: 15, Itc: 500, Tp: 50}, "Tp", "suprapolar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := thermotypeFor(tc.ix, tc.mb, tc.lat)
			if got.Index != tc.wantIndex || got.Code != tc.wantCode {
				t.Errorf("= %s/%q, want %s/%q", got.Index, got.Code, tc.wantIndex, tc.wantCode)
			}
		})
	}
}

// TestThermotype_ItLadders pins every It/Itc bound of WBCS08 Fig. 19 at the
// bound and just above it. P14 Table 7 writes the intervals with an exclusive
// lower bound (e.g. thermotemperate 291–350), so a value equal to a bound
// belongs to the colder thermotype.
func TestThermotype_ItLadders(t *testing.T) {
	cases := []struct {
		mb   Macrobioclimate
		it   float64
		want string
	}{
		{MacroTropical, 710.01, "infratropical"}, {MacroTropical, 710, "thermotropical"},
		{MacroTropical, 490.01, "thermotropical"}, {MacroTropical, 490, "mesotropical"},
		{MacroTropical, 320.01, "mesotropical"}, {MacroTropical, 320, "supratropical"},
		{MacroTropical, 160.01, "supratropical"}, {MacroTropical, 160, "orotropical"},
		{MacroTropical, 120, "orotropical"},
		{MacroMediterranean, 450.01, "inframediterranean"}, {MacroMediterranean, 450, "thermomediterranean"},
		{MacroMediterranean, 350.01, "thermomediterranean"}, {MacroMediterranean, 350, "mesomediterranean"},
		{MacroMediterranean, 220.01, "mesomediterranean"}, {MacroMediterranean, 220, "supramediterranean"},
		{MacroMediterranean, 120, "supramediterranean"},
		{MacroTemperate, 410.01, "infratemperate"}, {MacroTemperate, 410, "thermotemperate"},
		{MacroTemperate, 290.01, "thermotemperate"}, {MacroTemperate, 290, "mesotemperate"},
		{MacroTemperate, 190.01, "mesotemperate"}, {MacroTemperate, 190, "supratemperate"},
		{MacroTemperate, 120, "supratemperate"},
	}
	for _, tc := range cases {
		got := thermotypeFor(rmIndices{Ic: 15, Itc: tc.it, Tp: 5000}, tc.mb, 30)
		if got.Code != tc.want || got.Index != "Itc" || got.Value != tc.it {
			t.Errorf("%s Itc %.2f = %q (%s %.2f), want %q", tc.mb, tc.it, got.Code, got.Index, got.Value, tc.want)
		}
	}
}

// TestThermotype_TpLadders pins every Tp bound of WBCS08 Fig. 19 (and P14
// Table 7 for the Mediterranean and temperate columns). Tp = 0 is gelid in
// every macrobioclimate; the polar sequence has no oro- or cryoro-.
func TestThermotype_TpLadders(t *testing.T) {
	cases := []struct {
		mb   Macrobioclimate
		tp   float64
		want string
	}{
		{MacroTropical, 2900.01, "infratropical"}, {MacroTropical, 2900, "thermotropical"},
		{MacroTropical, 2300.01, "thermotropical"}, {MacroTropical, 2300, "mesotropical"},
		{MacroTropical, 1700.01, "mesotropical"}, {MacroTropical, 1700, "supratropical"},
		{MacroTropical, 950.01, "supratropical"}, {MacroTropical, 950, "orotropical"},
		{MacroTropical, 450.01, "orotropical"}, {MacroTropical, 450, "cryorotropical"},
		{MacroTropical, 0.01, "cryorotropical"}, {MacroTropical, 0, "gelid"},
		{MacroMediterranean, 2400.01, "inframediterranean"}, {MacroMediterranean, 2400, "thermomediterranean"},
		{MacroMediterranean, 2100.01, "thermomediterranean"}, {MacroMediterranean, 2100, "mesomediterranean"},
		{MacroMediterranean, 1500.01, "mesomediterranean"}, {MacroMediterranean, 1500, "supramediterranean"},
		{MacroMediterranean, 900.01, "supramediterranean"}, {MacroMediterranean, 900, "oromediterranean"},
		{MacroMediterranean, 450.01, "oromediterranean"}, {MacroMediterranean, 450, "cryoromediterranean"},
		{MacroMediterranean, 0.01, "cryoromediterranean"}, {MacroMediterranean, 0, "gelid"},
		{MacroTemperate, 2350.01, "infratemperate"}, {MacroTemperate, 2350, "thermotemperate"},
		{MacroTemperate, 2000.01, "thermotemperate"}, {MacroTemperate, 2000, "mesotemperate"},
		{MacroTemperate, 1400.01, "mesotemperate"}, {MacroTemperate, 1400, "supratemperate"},
		{MacroTemperate, 800.01, "supratemperate"}, {MacroTemperate, 800, "orotemperate"},
		{MacroTemperate, 380.01, "orotemperate"}, {MacroTemperate, 380, "cryorotemperate"},
		{MacroTemperate, 0.01, "cryorotemperate"}, {MacroTemperate, 0, "gelid"},
		{MacroBoreal, 900, "thermoboreal"},
		{MacroBoreal, 680.01, "thermoboreal"}, {MacroBoreal, 680, "mesoboreal"},
		{MacroBoreal, 580.01, "mesoboreal"}, {MacroBoreal, 580, "supraboreal"},
		{MacroBoreal, 480.01, "supraboreal"}, {MacroBoreal, 480, "oroboreal"},
		{MacroBoreal, 380.01, "oroboreal"}, {MacroBoreal, 380, "cryoroboreal"},
		{MacroBoreal, 0.01, "cryoroboreal"}, {MacroBoreal, 0, "gelid"},
		{MacroPolar, 500, "thermopolar"},
		{MacroPolar, 280.01, "thermopolar"}, {MacroPolar, 280, "mesopolar"},
		{MacroPolar, 100.01, "mesopolar"}, {MacroPolar, 100, "suprapolar"},
		{MacroPolar, 0.01, "suprapolar"}, {MacroPolar, 0, "gelid"},
	}
	for _, tc := range cases {
		got := thermotypeFor(rmIndices{Ic: 30, Itc: 300, Tp: tc.tp}, tc.mb, 30)
		if got.Code != tc.want || got.Index != "Tp" || got.Value != tc.tp {
			t.Errorf("%s Tp %.2f = %q (%s %.2f), want %q", tc.mb, tc.tp, got.Code, got.Index, got.Value, tc.want)
		}
	}
}

func TestThermotype_BilingualNames(t *testing.T) {
	cases := []struct {
		mb         Macrobioclimate
		tp         float64
		wantDe     string
		wantEnCode string
	}{
		{MacroTropical, 3000, "infratropisch", "infratropical"},
		{MacroMediterranean, 1000, "supramediterran", "supramediterranean"},
		{MacroTemperate, 200, "cryorotemperat", "cryorotemperate"},
		{MacroBoreal, 500, "supraboreal", "supraboreal"},
		{MacroPolar, 50, "suprapolar", "suprapolar"},
		{MacroTemperate, 0, "gelid", "gelid"},
	}
	for _, tc := range cases {
		got := thermotypeFor(rmIndices{Ic: 30, Tp: tc.tp}, tc.mb, 50)
		if got.De != tc.wantDe || got.En != tc.wantEnCode || got.Code != tc.wantEnCode {
			t.Errorf("%s Tp %.0f = %q/%q/%q, want %q/%q", tc.mb, tc.tp, got.Code, got.De, got.En, tc.wantEnCode, tc.wantDe)
		}
	}
}

func TestMacrobioclimate_GermanNames(t *testing.T) {
	want := map[Macrobioclimate]string{
		MacroTropical: "tropisch", MacroMediterranean: "mediterran", MacroTemperate: "temperat",
		MacroBoreal: "boreal", MacroPolar: "polar",
	}
	for m, de := range want {
		if got := m.De(); got != de {
			t.Errorf("%s.De() = %q, want %q", m, got, de)
		}
	}
}

// TestRivasMartinez_ReportsDecidingIndex checks that the result carries the
// index that decided the thermotype, for auditability.
func TestRivasMartinez_ReportsDecidingIndex(t *testing.T) {
	want := map[string]struct {
		index string
		value float64
	}{
		"8391": {"Itc", 416.25}, "10379": {"Tp", 1175}, "94120": {"It", 783.42},
	}
	for _, s := range rmReferenceStations {
		w, ok := want[s.wmo]
		if !ok {
			continue
		}
		got := RivasMartinez(s.climate(), s.site()).Thermotype
		if got.Index != w.index || math.Abs(got.Value-w.value) > 0.01 {
			t.Errorf("%s: index %s = %.2f, want %s = %.2f", s.name, got.Index, got.Value, w.index, w.value)
		}
	}
}
