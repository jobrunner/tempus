package domain

import "testing"

// Regression tests for float roundoff at the Köppen-Geiger thresholds of Beck
// et al. 2018, Table 2. Monthly normals carry one decimal; sums, means and
// derived bounds of them are exact in decimal but not in binary, so a value
// that sits exactly on a threshold can land on either side of it (see
// roundoffTolerance). Each input below is exactly on its threshold in decimal
// and was picked so that the naive float computation lands on the wrong side;
// the float value it produced is given in the case name. The expected class is
// the one Beck's inequality gives for the exact decimal value.
//
// Not covered here, because no one-decimal input can miss it: w
// "Pwdry<Pswet/10". Pswet = 10·Pwdry exactly makes Pswet a whole number of mm,
// and a whole number divided by 10 rounds to the same double as the one-decimal
// literal Pwdry — the comparison is always exact.
func TestKoppenGeiger_RoundoffAtBeck2018Thresholds(t *testing.T) {
	// A temperate regime with MAT 16 and Thot 24 (→ "a").
	temp16 := [12]float64{8, 10, 13, 16, 19, 22, 24, 23, 20, 16, 12, 9}
	// A moderately seasonal tropical regime: Tcold 25.
	tropT := [12]float64{25, 25, 26, 27, 28, 28, 27, 27, 27, 26, 25, 25}

	cases := []struct {
		name   string
		tmean  [12]float64
		precip [12]float64
		want   string
	}{
		// B "MAP<10×Pthreshold": MAT exactly 15 (the float mean is exact
		// here), even split → Pth 44; MAP exactly 440 sums to
		// 439.99999999999994. Not B; Psdry 32.0, Pwwet 39.7 → f; Thot 25.2 → a.
		{"MAP exactly 10·Pth is not B (MAP 439.99999999999994)",
			[12]float64{5.7, 8.2, 10.0, 13.3, 15.5, 22.4, 25.2, 22.7, 21.5, 14.8, 12.0, 8.7},
			[12]float64{32.6, 37.3, 39.7, 42.4, 40.3, 39.1, 32.0, 38.6, 33.7, 35.0, 35.4, 33.9}, "Cfa"},
		// BS "MAP≥5×Pthreshold": MAT exactly 15, even split → Pth 44; MAP
		// exactly 220 sums to 219.99999999999994.
		{"MAP exactly 5·Pth is BS (MAP 219.99999999999994)",
			[12]float64{7.4, 9.1, 9.8, 14.0, 18.0, 20.4, 23.2, 25.4, 20.6, 16.4, 11.3, 4.4},
			[12]float64{21.5, 20.5, 18.6, 19.5, 20.1, 15.5, 17.1, 16.7, 14.7, 22.1, 20.0, 13.7}, "BSk"},
		// h "MAT≥18": the months sum to exactly 216, float 215.99999999999997,
		// so MAT = 17.999999999999996. MAP 360 < 10·50 and ≥ 5·50 → BS.
		{"MAT exactly 18 is h (MAT 17.999999999999996)",
			[12]float64{11.1, 13.4, 14.4, 17.3, 19.9, 25.4, 24.6, 24.1, 20.6, 16.6, 14.1, 14.5},
			constPrecip(30), "BSh"},
		// Am "Pdry≥100-MAP/25": MAP exactly 2070 (exact in float, too), so the
		// bound is 100 − 82.8 = 17.2 — but 100-2070/25 in float is
		// 17.200000000000003 (82.8 is not representable).
		{"Pdry exactly 100-MAP/25 is Am (bound 17.200000000000003)", tropT,
			[12]float64{168.0, 188.6, 192.1, 179.7, 205.0, 183.5, 17.2, 195.0, 177.1, 187.6, 200.7, 175.5}, "Am"},
		// The same bound reached through the sum: MAP exactly 1587.5 sums to
		// 1587.4999999999998 → bound 36.500000000000007 for Pdry 36.5.
		{"Pdry exactly 100-MAP/25 is Am (MAP 1587.4999999999998)", tropT,
			[12]float64{124.2, 157.8, 137.7, 152.2, 143.1, 156.3, 36.5, 157.3, 129.1, 132.4, 124.6, 136.3}, "Am"},
		// s "Psdry<Pwwet/3": Psdry 30.4 is exactly Pwwet 91.2 / 3, but
		// 91.2/3 in float is 30.400000000000002. Not s, not w → f.
		{"Psdry exactly Pwwet/3 is not s (Pwwet/3 30.400000000000002)", temp16,
			[12]float64{80, 80, 80, 30.4, 50, 50, 50, 50, 50, 91.2, 80, 80}, "Cfa"},
		// s and w both hold; the halves are exactly equal, so Beck's Methods
		// ("assigning s when more precipitation falls in winter than in
		// summer and assigning w otherwise") give w. ONDJFM sums to
		// 600.3000000000001, AMJJAS to 600.3 (case from the review of #91).
		{"s and w both hold, equal halves in decimals → w (winter 600.3000000000001)", temp16,
			[12]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 100, 200, 300, 300, 200, 100}, "Cwa"},
		// The same tie the other way round: ONDJFM sums to exactly 760, AMJJAS
		// to 759.99999999999989.
		{"s and w both hold, equal halves in decimals → w (summer 759.99999999999989)", temp16,
			[12]float64{92.5, 46.6, 8.6, 14.7, 98.6, 209.1, 191.2, 140.9, 105.5, 309.0, 209.5, 93.8}, "Cwa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := KoppenGeiger(climate(tc.tmean, tc.precip), 40)
			if code != tc.want {
				t.Errorf("code = %q, want %q", code, tc.want)
			}
		})
	}
}
