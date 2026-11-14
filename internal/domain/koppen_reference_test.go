package domain

import "testing"

// Reference tests for KoppenGeiger against Beck et al. 2018 (Sci. Data
// 5:180214), Table 2 and the Methods text — the criteria the classifier claims
// to follow. Sources, quotes and the derivation of every expected class are in
// docs/research/koppen-reference.md. Expected classes were computed with an
// independent implementation of Beck 2018 Table 2 from the station normals;
// they are not sampled from the Beck map raster.

// TestKoppenGeiger_ReferenceStations pins real stations. All but the South
// Pole are official WMO Climatological Standard Normals 1991–2020 (NCEI
// Accession 0253808, doi:10.25921/800j-vn07), monthly TAVG and PRCP.
// Köppen ignores Tmin/Tmax, so only Tmean and precipitation are given.
func TestKoppenGeiger_ReferenceStations(t *testing.T) {
	cases := []struct {
		name, wmo string
		lat       float64
		tmean     [12]float64
		precip    [12]float64
		want      string
	}{
		{ // Amundsen-Scott South Pole: NOT an official WMO normal. 1991–2020 mean
			// of the BAS READER monthly means (legacy.bas.ac.uk/met/READER), which
			// carry no precipitation. Thot −27.3 ≤ 0 and MAT −49 makes Pth < 0,
			// so B is impossible for any P ≥ 0; P = 0 is used.
			name: "South Pole", wmo: "", lat: -90, want: "EF",
			tmean:  [12]float64{-28.1, -40.7, -53.6, -57.4, -57.7, -58.1, -60.2, -59.6, -58.9, -50.9, -37.2, -27.3},
			precip: [12]float64{},
		},
		{ // WMO 6680, Switzerland, 2501.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Säntis", wmo: "6680", lat: 47.249, want: "ET",
			tmean:  [12]float64{-6.8, -7.5, -5.6, -3, 1, 4.5, 6.4, 6.8, 3.6, 1.1, -3.3, -5.9},
			precip: [12]float64{269.5, 221.9, 254.6, 176.2, 224, 242.8, 278.6, 266.3, 214.3, 193.4, 204.9, 293.6},
		},
		{ // WMO 10961, Germany, 2964.5 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Zugspitze", wmo: "10961", lat: 47.421, want: "ET",
			tmean:  [12]float64{-10.1, -10.9, -9, -6.2, -2, 1.5, 3.4, 3.8, 0.5, -2, -6.3, -9.2},
			precip: [12]float64{176.5, 157.4, 208.1, 171.7, 192.7, 181.4, 184.5, 184.6, 150.2, 127.6, 157.4, 183.3},
		},
		{ // WMO 71018, Canada, 30.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Resolute", wmo: "71018", lat: 74.716, want: "ET",
			tmean:  [12]float64{-31.4, -32.1, -29.4, -21.2, -10, 0.5, 4.8, 2.4, -3.7, -12.2, -21.5, -27.3},
			precip: [12]float64{4.9, 4.4, 7.4, 8.2, 9.5, 15.7, 29.3, 32.7, 19, 15, 16.4, 6.7},
		},
		{ // WMO 72613, United States, 1910.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Thot = 10.0 exactly: E needs Thot ≤ 10 (D1). Pre-fix code: Dfc.
			name: "Mount Washington", wmo: "72613", lat: 44.267, want: "ET",
			tmean:  [12]float64{-14.5, -14.5, -10.6, -4.6, 2.4, 7.5, 10, 9.3, 6.2, -0.4, -6.2, -11.2},
			precip: [12]float64{145.9, 138.3, 170.7, 185.6, 195, 218.2, 226.7, 196, 194.5, 253.6, 205.6, 186.7},
		},
		{ // WMO 72202, United States, 9.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Miami Intl", wmo: "72202", lat: 25.791, want: "Am",
			tmean:  [12]float64{20.3, 21.5, 22.8, 24.8, 26.7, 28.2, 28.9, 29, 28.3, 26.7, 23.8, 21.8},
			precip: [12]float64{46.6, 54.5, 62.5, 85.5, 160.6, 266.9, 187, 243.2, 259.6, 194.3, 89.6, 62},
		},
		{ // WMO 94287, Australia, 2.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Cairns Aero", wmo: "94287", lat: -16.874, want: "Am",
			tmean:  [12]float64{28, 27.9, 27.2, 25.9, 24.1, 22.6, 21.8, 22.3, 23.9, 25.5, 26.9, 27.8},
			precip: [12]float64{388.7, 475.4, 367, 178.6, 81.2, 42.2, 35.9, 26.6, 28.4, 63.3, 85.1, 184.6},
		},
		{ // WMO 91165, United States, 31.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Beck has no As (D2); driest month Jul. Pre-fix code: As.
			name: "Lihue", wmo: "91165", lat: 21.984, want: "Aw",
			tmean:  [12]float64{22.4, 22.3, 22.7, 23.7, 24.6, 25.7, 26.4, 26.8, 26.7, 26, 24.6, 23.3},
			precip: [12]float64{70.5, 92.1, 142.4, 51.6, 55.3, 45.5, 44.4, 59.2, 55.4, 83.2, 102.5, 117.8},
		},
		{ // WMO 82397, Brazil, 29.9 m. WMO 1991–2020 normals (NCEI 0253808).
			// Beck has no As (D2). Pre-fix code: As.
			name: "Fortaleza", wmo: "82397", lat: -3.815, want: "Aw",
			tmean:  [12]float64{27.4, 27.3, 27, 26.8, 26.8, 26.4, 26.2, 26.6, 27, 27.4, 27.7, 27.8},
			precip: [12]float64{156.4, 187, 336.9, 385, 229, 130, 69.7, 20, 13.6, 9.5, 9.8, 37.1},
		},
		{ // WMO 94120, Australia, 30.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Dry season in the cold half; Aw under every rule set.
			name: "Darwin Airport", wmo: "94120", lat: -12.424, want: "Aw",
			tmean:  [12]float64{28.5, 28.3, 28.5, 28.6, 27.3, 25.6, 25.2, 25.9, 28, 29.3, 29.6, 29.3},
			precip: [12]float64{470.7, 412.4, 313.7, 105.1, 20.7, 2.2, 0.9, 0.8, 14.3, 68.9, 143.5, 279.3},
		},
		{ // WMO 48455, Thailand, 3.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Bangkok Metropolis", wmo: "48455", lat: 13.726, want: "Aw",
			tmean:  [12]float64{27.4, 28.6, 29.7, 30.7, 30.3, 29.7, 29.2, 29, 28.6, 28.4, 28.4, 27.3},
			precip: [12]float64{23.6, 21.4, 51, 93.3, 216.8, 198.5, 189.7, 227.1, 335.9, 288.7, 44.6, 11.6},
		},
		{ // WMO 24688, Russian Federation (In Asia), 740.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Oymyakon", wmo: "24688", lat: 63.25, want: "Dfd",
			tmean:  [12]float64{-45.7, -42.2, -30.2, -12.7, 3.6, 12.7, 15.3, 10.8, 2.5, -13.8, -34.4, -44.9},
			precip: [12]float64{6.3, 6.7, 5.3, 5.4, 15, 38.5, 46.5, 39.2, 24.5, 13.4, 12.6, 7.1},
		},
		{ // WMO 24966, Russian Federation (In Asia), 169.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Tcold −38.3, 0.3 K from the d threshold: knife-edge regression pin.
			name: "Ust-Maya", wmo: "24966", lat: 60.383, want: "Dfd",
			tmean:  [12]float64{-38.3, -33.7, -19, -3.1, 8.1, 16.3, 18.9, 15, 6.5, -6.5, -25.9, -38.2},
			precip: [12]float64{11.8, 8.4, 8.1, 10.9, 28.4, 34.5, 54.1, 50.8, 44.5, 29.4, 19.5, 11.2},
		},
		{ // WMO 24266, Russian Federation (In Asia), 136.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Psdry 4.0 < Pwwet/3 = 4.27: knife-edge regression pin for s.
			name: "Verkhoyansk", wmo: "24266", lat: 67.55, want: "Dsd",
			tmean:  [12]float64{-44.7, -42.2, -28.8, -10.8, 4.2, 14.2, 16.7, 12.1, 2.7, -13.6, -33.7, -43.4},
			precip: [12]float64{5.9, 5.3, 4.6, 4, 15.7, 30.1, 34.7, 29.4, 22.1, 12.8, 11.1, 6.1},
		},
		{ // WMO 6639, Switzerland, 1404.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Sep = 10.0 exactly: Tmon10 counts months > 10 (D4), so 3 → c. Pre-fix code: Dfb.
			name: "Napf", wmo: "6639", lat: 47.005, want: "Dfc",
			tmean:  [12]float64{-1.3, -1.6, 1, 4.2, 8.3, 11.9, 13.9, 13.9, 10, 6.8, 2.2, -0.4},
			precip: [12]float64{98.4, 98.3, 114.1, 125.5, 187.8, 178, 188.2, 175.6, 131.1, 118.2, 109.7, 123.9},
		},
		{ // WMO 22820, Russian Federation (In Europe), 110.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Sep = 10.0 exactly (D4). Pre-fix code: Dfb.
			name: "Petrozavodsk", wmo: "22820", lat: 61.817, want: "Dfc",
			tmean:  [12]float64{-8.4, -8.2, -3.5, 2.5, 8.9, 14.1, 17.1, 15, 10, 3.8, -1.9, -5.7},
			precip: [12]float64{37.9, 28.3, 31.6, 32, 47.7, 61.1, 80.2, 79.5, 59.2, 53.6, 50.1, 43.6},
		},
		{ // WMO 56778, China, 1892.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// s and w both hold, summer wetter → w (D3). Pre-fix code: Csb.
			name: "Kunming", wmo: "56778", lat: 25.02, want: "Cwb",
			tmean:  [12]float64{9.3, 11.5, 14.8, 17.8, 19.6, 20.7, 20.5, 20.2, 18.8, 16.2, 12.5, 9.5},
			precip: [12]float64{23.8, 11.9, 19.6, 25.4, 80.1, 173.1, 215.7, 195.9, 119.3, 82.4, 30.1, 13.7},
		},
		{ // WMO 76680, Mexico, 2309.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// s and w both hold, summer wetter → w (D3). Pre-fix code: Csb.
			name: "Mexico City Tacubaya", wmo: "76680", lat: 19.404, want: "Cwb",
			tmean:  [12]float64{15.1, 16.9, 18.7, 19.9, 19.9, 18.9, 17.8, 18, 17.3, 17, 16.3, 15.6},
			precip: [12]float64{4.8, 5.2, 9.6, 16.9, 45.9, 108.9, 133.2, 132.1, 124.4, 57.2, 14.7, 2.3},
		},
		{ // WMO 67481, Zambia, 1360.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// s and w both hold, summer (ONDJFM) wetter → w (D3). Pre-fix code: Csa.
			name: "Isoka", wmo: "67481", lat: -10.167, want: "Cwa",
			tmean:  [12]float64{22.3, 22.2, 22.1, 21.3, 19.6, 18.1, 17.7, 19.8, 22.4, 24.4, 24.6, 22.9},
			precip: [12]float64{231.2, 223.8, 171.4, 71.8, 7.1, 0.2, 0.2, 0.1, 0.1, 8.4, 68.4, 218},
		},
		{ // WMO 65473, Ghana, 14.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Warmer half is ONDJFM, so AMJJAS is winter with 74 % of MAP → Pth = 2·MAT, not B (D6). Pre-fix code: BSh.
			name: "Tema", wmo: "65473", lat: 5.617, want: "Aw",
			tmean:  [12]float64{28.1, 28.9, 29, 28.8, 28.2, 26.9, 25.8, 25.3, 26.2, 27.3, 28.3, 28.5},
			precip: [12]float64{8.4, 22, 51.3, 89, 141.3, 183.9, 59.6, 15, 35.5, 62.1, 25.2, 14.7},
		},
		{ // WMO 52055210, Colombia, 2820.0 m. WMO 1991–2020 normals (NCEI 0253808).
			// Warmer half is ONDJFM (12.75 vs 12.51 °C), so the dry Aug is winter → f (D6). Pre-fix code: Csb.
			name: "Botana", wmo: "52055210", lat: 1.16, want: "Cfb",
			tmean:  [12]float64{12.58, 12.82, 12.96, 13.05, 13.09, 12.53, 12.01, 11.97, 12.42, 12.76, 12.73, 12.64},
			precip: [12]float64{86.31, 73.2, 93.52, 102.58, 90.98, 67.37, 58.28, 39.58, 42.99, 83.49, 119.11, 96.25},
		},
		{ // WMO 2836, Finland, 179.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Sodankylä Tähtelä", wmo: "2836", lat: 67.367, want: "Dfc",
			tmean:  [12]float64{-12.5, -12.1, -7.1, -0.8, 5.6, 11.9, 15, 12.4, 7, 0, -5.8, -9.6},
			precip: [12]float64{36.1, 30.8, 30, 31.6, 40.1, 60.8, 75.5, 55.8, 51.7, 47.4, 42.4, 40.9},
		},
		{ // WMO 8391, Spain, 34.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Sevilla San Pablo", wmo: "8391", lat: 37.417, want: "Csa",
			tmean:  [12]float64{11.2, 12.6, 15.5, 17.6, 21.3, 25.3, 28, 28.1, 24.7, 20.4, 15.1, 12.1},
			precip: [12]float64{56.3, 46.2, 46.9, 54, 33.9, 5.8, 0.6, 2.5, 33.1, 75.4, 72.2, 77.2},
		},
		{ // WMO 16239, Italy, 129.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Roma Ciampino", wmo: "16239", lat: 41.808, want: "Csa",
			tmean:  [12]float64{7.47, 8, 10.7, 13.6, 17.96, 22.48, 25.11, 25.43, 21.02, 16.97, 12.42, 8.52},
			precip: [12]float64{65.62, 62.76, 58.58, 68.63, 56.85, 30.08, 19.79, 30.2, 64.86, 88.11, 108.24, 98.31},
		},
		{ // WMO 94610, Australia, 15.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Perth Airport", wmo: "94610", lat: -31.927, want: "Csa",
			tmean:  [12]float64{24.9, 25, 23.1, 19.8, 16.4, 14.2, 13.3, 13.7, 14.8, 17.2, 20.2, 22.9},
			precip: [12]float64{15.8, 16.6, 17.9, 30.7, 81.4, 123.9, 139.6, 122.1, 78.9, 33.6, 28.7, 9.6},
		},
		{ // WMO 8487, Spain, 21.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Almería Aeropuerto", wmo: "8487", lat: 36.846, want: "BSh",
			tmean:  [12]float64{12.8, 13.3, 15.1, 17, 19.9, 23.4, 26.1, 26.9, 24, 20.4, 16.2, 13.8},
			precip: [12]float64{20.8, 23.3, 20.7, 15.2, 10.9, 5.5, 0.6, 2.3, 16.1, 25.2, 25.1, 31.8},
		},
		{ // WMO 8222, Spain, 667.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Madrid Retiro", wmo: "8222", lat: 40.412, want: "BSk",
			tmean:  [12]float64{6.6, 8, 11.4, 13.7, 17.7, 23, 26.4, 25.9, 21.2, 15.5, 10, 7.1},
			precip: [12]float64{32, 33.5, 35.3, 46.1, 48.3, 19.6, 9.2, 10.4, 23.6, 64.2, 52.2, 42.4},
		},
		{ // WMO 70274, United States, 70.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Palmer", wmo: "70274", lat: 61.596, want: "Dsc",
			tmean:  [12]float64{-9.2, -6.2, -3.6, 3.8, 9.4, 13.5, 15.1, 13.5, 9.1, 1.9, -5.4, -7.6},
			precip: [12]float64{16.4, 17.7, 12, 7.5, 15.8, 25.3, 40.6, 58.9, 52.8, 31.8, 16.8, 24.3},
		},
		{ // WMO 10379, Germany, 80.9 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Potsdam", wmo: "10379", lat: 52.381, want: "Cfb",
			tmean:  [12]float64{0.7, 1.6, 4.7, 9.9, 14.2, 17.4, 19.4, 18.9, 14.6, 9.6, 4.8, 1.7},
			precip: [12]float64{45.3, 36.1, 39.3, 29.2, 53.3, 60.8, 76.2, 59.2, 47.1, 42.8, 42.3, 46.1},
		},
		{ // WMO 11035, Austria, 198.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Wien Hohe Warte", wmo: "11035", lat: 48.249, want: "Cfb",
			tmean:  [12]float64{1.1, 2.8, 6.9, 11.9, 16.3, 20, 21.9, 21.6, 16.6, 11.2, 6.2, 1.8},
			precip: [12]float64{42.1, 38.1, 51.6, 41.8, 78.9, 70, 77.7, 69.1, 64.1, 46.9, 46, 46.8},
		},
		{ // WMO 2978, Finland, 3.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Helsinki Kaisaniemi", wmo: "2978", lat: 60.175, want: "Dfb",
			tmean:  [12]float64{-3.1, -3.8, -0.7, 4.4, 10.4, 14.9, 18.1, 16.9, 12.3, 6.6, 2.4, -0.7},
			precip: [12]float64{52.8, 38.3, 34.4, 34.1, 37.9, 60.1, 57.1, 81.4, 55.7, 73.3, 69.3, 58.3},
		},
		{ // WMO 94970, Australia, 51.0 m. WMO 1991–2020 normals (NCEI 0253808).
			name: "Hobart", wmo: "94970", lat: -42.89, want: "Cfb",
			tmean:  [12]float64{17.9, 17.6, 16.2, 13.8, 11.6, 9.2, 9, 9.8, 11.4, 13.1, 14.7, 16.4},
			precip: [12]float64{43.7, 37.8, 37, 42.6, 39.2, 46, 44.5, 63, 55.6, 52.8, 49.4, 53.7},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := MonthlyClimate{Tmin: tc.tmean, Tmax: tc.tmean, Tmean: tc.tmean, Precip: tc.precip}
			code, de, en := KoppenGeiger(c, tc.lat)
			if code != tc.want {
				t.Errorf("%s (WMO %s): code = %q, want %q (Beck et al. 2018)", tc.name, tc.wmo, code, tc.want)
			}
			if de == "" || en == "" {
				t.Errorf("%s: missing description for %q", tc.name, code)
			}
		})
	}
}

// monthsWith returns a [12]float64 filled with v, then overridden at the given
// month indices (0 = Jan).
func monthsWith(v float64, overrides map[int]float64) [12]float64 {
	p := constPrecip(v)
	for i, x := range overrides {
		p[i] = x
	}
	return p
}

// TestKoppenGeiger_Beck2018Thresholds pins every inequality of Beck et al.
// 2018, Table 2 exactly at its boundary, with synthetic normals built so that
// only the criterion under test decides the class.
func TestKoppenGeiger_Beck2018Thresholds(t *testing.T) {
	// A moderately seasonal tropical regime: Tcold 25, AMJJAS the warmer half.
	tropT := [12]float64{25, 25, 26, 27, 28, 28, 27, 27, 27, 26, 25, 25}
	// A temperate regime with MAT 15 and AMJJAS the warmer half.
	temp15 := [12]float64{7, 8, 10, 13, 17, 21, 24, 24, 20, 16, 11, 9}
	// A temperate regime with MAT 16 and Thot 24 (→ "a").
	temp16 := [12]float64{8, 10, 13, 16, 19, 22, 24, 23, 20, 16, 12, 9}

	cases := []struct {
		name   string
		tmean  [12]float64
		precip [12]float64
		lat    float64
		want   string
	}{
		// E: "Not (B) & Thot≤10"; C and D need "Thot>10" (D1).
		{"Thot exactly 10 is E", [12]float64{-10, -8, -5, 0, 4, 8, 10, 9, 5, 0, -5, -8}, constPrecip(50), 45, "ET"},
		{"Thot 10.1 is no longer E", [12]float64{-10, -8, -5, 0, 4, 8, 10.1, 9, 5, 0, -5, -8}, constPrecip(50), 45, "Dfc"},
		// ET "Thot>0", EF "Thot≤0".
		{"Thot exactly 0 is EF", [12]float64{-30, -28, -25, -20, -10, -3, 0, -2, -8, -15, -22, -28}, constPrecip(20), 80, "EF"},
		{"Thot 0.1 is ET", [12]float64{-30, -28, -25, -20, -10, -3, 0.1, -2, -8, -15, -22, -28}, constPrecip(20), 80, "ET"},

		// Tmon10 = "the number of months with air temperature >10 °C" (D4):
		// three months above 10 plus one at exactly 10 is 3, i.e. c, not b.
		{"a month at exactly 10 °C does not count for Tmon10", [12]float64{2, 3, 5, 7, 9, 10, 12, 13, 11, 8, 5, 3}, constPrecip(80), 50, "Cfc"},
		{"a month at 10.1 °C counts for Tmon10", [12]float64{2, 3, 5, 7, 9, 10.1, 12, 13, 11, 8, 5, 3}, constPrecip(80), 50, "Cfb"},

		// Af "Pdry≥60".
		{"Pdry exactly 60 is Af", tropT, monthsWith(200, map[int]float64{6: 60}), 10, "Af"},
		// Am "Pdry≥100-MAP/25": MAP 2000 → threshold exactly 20.
		{"Pdry exactly 100-MAP/25 is Am", tropT, monthsWith(180, map[int]float64{6: 20}), 10, "Am"},
		// Aw "Pdry<100-MAP/25". Beck has no As (D2): the driest month falling in
		// the summer half (Jul, AMJJAS warmer) still gives Aw.
		{"Pdry below 100-MAP/25 is Aw even with a summer dry month", tropT, monthsWith(180, map[int]float64{6: 19}), 10, "Aw"},

		// d "Not (a or b) & Tcold<-38" (strict; D5: a and b take precedence).
		{"Tcold exactly -38 is not d", [12]float64{-38, -35, -20, -2, 5, 9, 12, 10, 3, -5, -25, -36}, constPrecip(30), 65, "Dfc"},
		{"Tcold -38.1 with Tmon10 1 is d", [12]float64{-38.1, -35, -20, -2, 5, 9, 18, 10, 3, -5, -25, -36}, constPrecip(30), 65, "Dfd"},
		{"Tcold below -38 with Thot exactly 22 is a, not d", [12]float64{-40, -35, -20, -2, 10, 18, 22, 18, 8, -5, -25, -38}, constPrecip(30), 65, "Dfa"},
		{"Tcold below -38 with Tmon10 4 is b, not d", [12]float64{-40, -35, -20, -2, 11, 18, 20, 18, 11, -5, -25, -38}, constPrecip(30), 65, "Dfb"},

		// Pthreshold "=2×MAT if >70% of precipitation falls in winter" (D7):
		// exactly 70 % (280 of 400 mm in ONDJFM) is the "otherwise" case,
		// Pth = 2·15+14 = 44, and MAP 400 < 10·44 makes it arid.
		{"exactly 70 % winter precipitation uses the even threshold", temp15,
			[12]float64{50, 40, 40, 20, 20, 20, 20, 20, 20, 50, 50, 50}, 40, "BSk"},
		// "=2×MAT+28 if >70% … in summer": exactly 70 % (350 of 500 mm in
		// AMJJAS) gives Pth 44, and MAP 500 ≥ 10·44 is not arid.
		{"exactly 70 % summer precipitation uses the even threshold", temp15,
			[12]float64{25, 25, 25, 60, 60, 60, 60, 55, 55, 25, 25, 25}, 40, "Cfa"},

		// s "Psdry<40 & Psdry<Pwwet/3" and w "Pwdry<Pswet/10" both hold here.
		// Beck 2018 Methods: "assigning s when more precipitation falls in
		// winter than in summer and assigning w otherwise" (D3).
		{"s and w both hold, summer wetter → w", temp16,
			[12]float64{5, 5, 20, 10, 100, 200, 200, 150, 100, 60, 20, 5}, 30, "Cwa"},
		{"s and w both hold, winter wetter → s", temp16,
			[12]float64{250, 200, 4, 60, 30, 10, 5, 10, 30, 100, 200, 250}, 30, "Csa"},
		{"s and w both hold, equal halves → w (otherwise)", temp16,
			[12]float64{100, 55, 5, 10, 100, 200, 200, 150, 100, 300, 200, 100}, 30, "Cwa"},

		// "Summer (winter) is the six-month period that is warmer (colder)
		// between April-September and October-March" (D6): the latitude sign
		// does not matter. The same northern-type normals give the same class
		// at a southern latitude.
		{"half-years follow temperature, not latitude sign", temp16,
			[12]float64{5, 5, 20, 10, 100, 200, 200, 150, 100, 60, 20, 5}, -30, "Cwa"},
		// Beck 2018 leaves a temperature tie between the halves open; tempus
		// takes April–September as summer, so the dry June makes it "s".
		{"equal half-year temperatures take April–September as summer", constPrecip(15),
			[12]float64{100, 100, 100, 50, 50, 10, 50, 50, 50, 100, 100, 100}, 45, "Csb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := KoppenGeiger(climate(tc.tmean, tc.precip), tc.lat)
			if code != tc.want {
				t.Errorf("code = %q, want %q", code, tc.want)
			}
		})
	}
}
