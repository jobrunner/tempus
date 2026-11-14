package domain

import "testing"

// Reference tests for the Rivas-Martínez macrobioclimate key and thermotypes.
// Every criterion, its source and the derivation of each expectation are in
// docs/research/rivas-martinez-reference.md. Sources:
//
//   - WBCS08: Rivas-Martínez, Global Bioclimatics (Clasificación Bioclimática
//     de la Tierra), versión 01-12-2008, globalbioclimatics.org, via Wayback
//     (identical text 2013 and 2021): ch. 5.1 "Clave para los macrobioclimas"
//     with footnote (1) (altitude reduction to 200 m), ch. 2 (indices, Fig. 7
//     Itc), ch. 3 Fig. 19 (thermotype horizons).
//   - P14: Pesaresi, Galdenzi, Biondi & Casavecchia 2014, J. Maps 10(4),
//     doi:10.1080/17445647.2014.891472, Table 2 (Ios compensation) and Table 7
//     (thermotype thresholds), both "according to Rivas-Martínez et al. 2011".
//
// Station data: official WMO Climatological Standard Normals 1991–2020 (NCEI
// Accession 0253808, doi:10.25921/800j-vn07), monthly TAVG, TMIN, TMAX, PRCP;
// station elevation from the same file's metadata. The South Pole is the only
// exception (see its case). Index values in the comments were computed with
// an independent script from the same normals.

type rmStation struct {
	name, wmo                 string
	lat, lon, elev            float64
	tmean, tmin, tmax, precip [12]float64
	wantMacro                 Macrobioclimate
	wantThermotype            string // "" = borderline, not asserted
}

func (s rmStation) climate() MonthlyClimate {
	return MonthlyClimate{Tmean: s.tmean, Tmin: s.tmin, Tmax: s.tmax, Precip: s.precip}
}

func (s rmStation) site() RivasMartinezSite {
	return RivasMartinezSite{Lat: s.lat, Lon: s.lon, ElevationM: s.elev, ElevationKnown: true}
}

var rmReferenceStations = []rmStation{
	{ // WMO 6680 Säntis, CH, 2501 m. Not polar: lat 47.2 is not > 72, and at
		// 200 m (WBCS08 key 5.1 step 11, footnote (1), lat < 48 so +55 Tp and
		// +0.6 °C per 100 m) Tp = 234 + 55·23.01 = 1500 > 380. Step 13 (Ic 14.3 < 21):
		// T200 = −0.72 + 0.6·23.01 = 13.1 ≥ 5.3 → not boreal. Step 14: Ios2 41.3 > 2
		// → temperate. Thermotype: Itc −155 < 120 → Tp 234 → cryorotemperate
		// (Fig. 19 / P14 Table 7: Tp 1–380).
		name: "Säntis", wmo: "6680", lat: 47.249, lon: 9.343, elev: 2501,
		tmean:     [12]float64{-6.8, -7.5, -5.6, -3, 1, 4.5, 6.4, 6.8, 3.6, 1.1, -3.3, -5.9},
		tmin:      [12]float64{-9.3, -10, -8, -5.3, -1.3, 2.1, 4, 4.5, 1.3, -1.3, -5.7, -8.4},
		tmax:      [12]float64{-4.1, -4.8, -3.3, -1, 3.2, 6.9, 9.2, 9.6, 6.2, 3.7, -0.7, -3.2},
		precip:    [12]float64{269.5, 221.9, 254.6, 176.2, 224, 242.8, 278.6, 266.3, 214.3, 193.4, 204.9, 293.6},
		wantMacro: MacroTemperate, wantThermotype: "cryorotemperate",
	},
	{ // WMO 10961 Zugspitze, DE, 2964.5 m. As Säntis: Tp200 = 92 + 55·27.6 = 1612,
		// T200 = 12.7, Ios2 51.3 → temperate; Tp 92 → cryorotemperate.
		name: "Zugspitze", wmo: "10961", lat: 47.421, lon: 10.985, elev: 2964.5,
		tmean:     [12]float64{-10.1, -10.9, -9, -6.2, -2, 1.5, 3.4, 3.8, 0.5, -2, -6.3, -9.2},
		tmin:      [12]float64{-12.7, -13.5, -11.6, -8.7, -4.4, -0.9, 0.9, 1.4, -1.9, -4.5, -8.8, -11.8},
		tmax:      [12]float64{-7.3, -8, -6.3, -3.2, 0.9, 4.5, 6.5, 6.9, 3.3, 0.6, -3.6, -6.4},
		precip:    [12]float64{176.5, 157.4, 208.1, 171.7, 192.7, 181.4, 184.5, 184.6, 150.2, 127.6, 157.4, 183.3},
		wantMacro: MacroTemperate, wantThermotype: "cryorotemperate",
	},
	{ // WMO 8487 Almería Aeropuerto, ES, 21 m. Step 14 with Io 0.86 (P14 Table 2
		// row Io ≤ 2.0: limit 2.0): Ios2 0.05 < 2.0 → mediterranean. Thermotype
		// not asserted: It 448 sits 2 units below the infra/thermo bound 450.
		name: "Almería Aeropuerto", wmo: "8487", lat: 36.846, lon: -2.357, elev: 21,
		tmean:     [12]float64{12.8, 13.3, 15.1, 17, 19.9, 23.4, 26.1, 26.9, 24, 20.4, 16.2, 13.8},
		tmin:      [12]float64{8.7, 9.2, 10.8, 12.8, 15.5, 18.9, 21.8, 22.6, 19.8, 16.3, 12.2, 9.7},
		tmax:      [12]float64{17, 17.5, 19.4, 21.3, 24.3, 27.8, 30.5, 31.2, 28.2, 24.5, 20.3, 17.9},
		precip:    [12]float64{20.8, 23.3, 20.7, 15.2, 10.9, 5.5, 0.6, 2.3, 16.1, 25.2, 25.1, 31.8},
		wantMacro: MacroMediterranean,
	},
	{ // WMO 8222 Madrid Retiro, ES, 667 m. Step 14: Io 2.23 (limit 1.9), Ios2
		// 0.37 → mediterranean. Thermotype: Ic 19.8 < 21, Itc = 287 + 5·1.8 = 296
		// ≥ 120 → Itc ladder: 220 < 296 ≤ 350 → mesomediterranean.
		name: "Madrid Retiro", wmo: "8222", lat: 40.412, lon: -3.678, elev: 667,
		tmean:     [12]float64{6.6, 8, 11.4, 13.7, 17.7, 23, 26.4, 25.9, 21.2, 15.5, 10, 7.1},
		tmin:      [12]float64{3, 3.7, 6.3, 8.2, 11.9, 16.5, 19.5, 19.3, 15.5, 11.1, 6.4, 3.7},
		tmax:      [12]float64{10.2, 12.3, 16.4, 19.1, 23.5, 29.4, 33.3, 32.6, 26.9, 20, 13.7, 10.4},
		precip:    [12]float64{32, 33.5, 35.3, 46.1, 48.3, 19.6, 9.2, 10.4, 23.6, 64.2, 52.2, 42.4},
		wantMacro: MacroMediterranean, wantThermotype: "mesomediterranean",
	},
	{ // WMO 56778 Kunming, CN, 1892 m, lat 25.0 (subtropical belt; lat < 26 so
		// not step 3). Step 4: T 15.95, m 4.0, Itc 362 → no. Step 5 at 200 m
		// (+0.6 °C, +0.5 °C per 100 m): T 26.1 > 21 and M 24.8 > 18 → step 6:
		// Pss 810 > Psw 182 → tropical. Thermotype: Itc 362 → mesotropical
		// (Fig. 19: 320–490).
		name: "Kunming", wmo: "56778", lat: 25.02, lon: 102.68, elev: 1892,
		tmean:     [12]float64{9.3, 11.5, 14.8, 17.8, 19.6, 20.7, 20.5, 20.2, 18.8, 16.2, 12.5, 9.5},
		tmin:      [12]float64{4, 5.8, 8.9, 12, 14.9, 17.3, 17.6, 17.2, 15.7, 12.9, 8.2, 4.8},
		tmax:      [12]float64{16.3, 18.5, 21.8, 24.3, 25.2, 25.4, 24.8, 25, 23.5, 21.1, 18.7, 16},
		precip:    [12]float64{23.8, 11.9, 19.6, 25.4, 80.1, 173.1, 215.7, 195.9, 119.3, 82.4, 30.1, 13.7},
		wantMacro: MacroTropical, wantThermotype: "mesotropical",
	},
	{ // WMO 76680 Mexico City Tacubaya, MX, 2309 m. Step 1: lat 19.4 < 23 →
		// tropical at any altitude. It 465 → mesotropical (320–490).
		name: "Mexico City Tacubaya", wmo: "76680", lat: 19.404, lon: -99.197, elev: 2309,
		tmean:     [12]float64{15.1, 16.9, 18.7, 19.9, 19.9, 18.9, 17.8, 18, 17.3, 17, 16.3, 15.6},
		tmin:      [12]float64{8.1, 9.7, 11.2, 13.1, 13.7, 13.9, 13.1, 13.2, 13.3, 12.3, 10, 8.5},
		tmax:      [12]float64{20.8, 23.2, 25.1, 26.6, 26.5, 24.9, 23.7, 23.8, 23, 22.5, 21.8, 21.2},
		precip:    [12]float64{4.8, 5.2, 9.6, 16.9, 45.9, 108.9, 133.2, 132.1, 124.4, 57.2, 14.7, 2.3},
		wantMacro: MacroTropical, wantThermotype: "mesotropical",
	},
	{ // WMO 24266 Verkhoyansk, RU, 136 m. Step 11: Tp 499 > 380. Step 12: Ic 61
		// ≥ 11. Step 13 (Ic > 28): T −13.9 < 3.8 and Tp 499 < 800 → boreal.
		// Thermotype (always Tp): 480 < 499 ≤ 580 → supraboreal.
		name: "Verkhoyansk", wmo: "24266", lat: 67.55, lon: 133.383, elev: 136,
		tmean:     [12]float64{-44.7, -42.2, -28.8, -10.8, 4.2, 14.2, 16.7, 12.1, 2.7, -13.6, -33.7, -43.4},
		tmin:      [12]float64{-47.7, -46.4, -37.3, -20.3, -2, 7.5, 10.1, 5.7, -2, -18.1, -37.2, -46.2},
		tmax:      [12]float64{-41.6, -36.9, -18.6, -1.7, 10.4, 21, 23.7, 19.3, 8.7, -8.6, -30, -40.4},
		precip:    [12]float64{5.9, 5.3, 4.6, 4, 15.7, 30.1, 34.7, 29.4, 22.1, 12.8, 11.1, 6.1},
		wantMacro: MacroBoreal, wantThermotype: "supraboreal",
	},
	{ // WMO 70274 Palmer, AK, US, 70 m. Step 13 (Ic 24.3 in 21–28): T 2.86 <
		// 4.8 and Tp 663 < 740 → boreal. 580 < Tp 663 ≤ 680 → mesoboreal.
		name: "Palmer", wmo: "70274", lat: 61.596, lon: -149.092, elev: 70,
		tmean:     [12]float64{-9.2, -6.2, -3.6, 3.8, 9.4, 13.5, 15.1, 13.5, 9.1, 1.9, -5.4, -7.6},
		tmin:      [12]float64{-13.2, -10.4, -8.2, -1.2, 3.8, 8.3, 10.4, 9, 4.9, -1.9, -9.1, -11.4},
		tmax:      [12]float64{-5.3, -2, 1.1, 8.8, 15.1, 18.7, 19.8, 18.1, 13.2, 5.7, -1.7, -3.8},
		precip:    [12]float64{16.4, 17.7, 12, 7.5, 15.8, 25.3, 40.6, 58.9, 52.8, 31.8, 16.8, 24.3},
		wantMacro: MacroBoreal, wantThermotype: "mesoboreal",
	},
	{ // WMO 2836 Sodankylä Tähtelä, FI, 179 m. Step 13 (Ic 27.5): T 0.33 < 4.8,
		// Tp 519 < 740 → boreal. 480 < 519 ≤ 580 → supraboreal.
		name: "Sodankylä Tähtelä", wmo: "2836", lat: 67.367, lon: 26.629, elev: 179,
		tmean:     [12]float64{-12.5, -12.1, -7.1, -0.8, 5.6, 11.9, 15, 12.4, 7, 0, -5.8, -9.6},
		tmin:      [12]float64{-17.5, -17.2, -12.9, -6, 0.7, 6.9, 10.1, 7.8, 3.3, -2.8, -9.2, -14},
		tmax:      [12]float64{-8.1, -7.6, -1.8, 4, 10.5, 16.9, 20.2, 17.4, 11.1, 2.9, -2.8, -5.7},
		precip:    [12]float64{36.1, 30.8, 30, 31.6, 40.1, 60.8, 75.5, 55.8, 51.7, 47.4, 42.4, 40.9},
		wantMacro: MacroBoreal, wantThermotype: "supraboreal",
	},
	{ // WMO 94970 Hobart, AU, 51 m. Southern summer quarter DJF: Ios2 2.30 > 2
		// → temperate (step 14). Ic 8.9 (8–18: Itc = It) → Itc 314 → 290 < 314 ≤
		// 410 → thermotemperate.
		name: "Hobart", wmo: "94970", lat: -42.89, lon: 147.328, elev: 51,
		tmean:     [12]float64{17.9, 17.6, 16.2, 13.8, 11.6, 9.2, 9, 9.8, 11.4, 13.1, 14.7, 16.4},
		tmin:      [12]float64{12.9, 12.8, 11.6, 9.4, 7.6, 5.5, 5.2, 5.6, 7, 8.4, 10, 11.6},
		tmax:      [12]float64{22.8, 22.3, 20.8, 18.2, 15.6, 13, 12.8, 13.9, 15.9, 17.8, 19.4, 21.2},
		precip:    [12]float64{43.7, 37.8, 37, 42.6, 39.2, 46, 44.5, 63, 55.6, 52.8, 49.4, 53.7},
		wantMacro: MacroTemperate, wantThermotype: "thermotemperate",
	},
	{ // WMO 94120 Darwin Airport, AU, 30 m. Step 1 (lat 12.4 < 23). It 783 > 710
		// → infratropical.
		name: "Darwin Airport", wmo: "94120", lat: -12.424, lon: 130.893, elev: 30,
		tmean:     [12]float64{28.5, 28.3, 28.5, 28.6, 27.3, 25.6, 25.2, 25.9, 28, 29.3, 29.6, 29.3},
		tmin:      [12]float64{25, 25, 24.8, 24.2, 22.3, 20, 19.3, 19.8, 22.9, 24.8, 25.4, 25.5},
		tmax:      [12]float64{32, 31.7, 32.2, 33, 32.3, 31.1, 31.2, 31.9, 33.1, 33.8, 33.8, 33},
		precip:    [12]float64{470.7, 412.4, 313.7, 105.1, 20.7, 2.2, 0.9, 0.8, 14.3, 68.9, 143.5, 279.3},
		wantMacro: MacroTropical, wantThermotype: "infratropical",
	},
	{ // WMO 48455 Bangkok Metropolis, TH, 3 m. Step 1. It 844 > 710 → infratropical.
		name: "Bangkok Metropolis", wmo: "48455", lat: 13.726, lon: 100.56, elev: 3,
		tmean:     [12]float64{27.4, 28.6, 29.7, 30.7, 30.3, 29.7, 29.2, 29, 28.6, 28.4, 28.4, 27.3},
		tmin:      [12]float64{23.4, 24.8, 26.4, 27.2, 26.9, 26.4, 26.1, 25.9, 25.4, 25.2, 24.7, 23.2},
		tmax:      [12]float64{32.7, 33.7, 34.7, 35.7, 35.1, 34.1, 33.5, 33.3, 33.2, 33, 33.1, 32.3},
		precip:    [12]float64{23.6, 21.4, 51, 93.3, 216.8, 198.5, 189.7, 227.1, 335.9, 288.7, 44.6, 11.6},
		wantMacro: MacroTropical, wantThermotype: "infratropical",
	},
	{ // WMO 10379 Potsdam, DE, 81 m. Step 13 (Ic 18.7 < 21): T 9.8 ≥ 5.3 → not
		// boreal; step 14: Ios2 3.54 > 2 → temperate. Itc = 111 + 5·0.7 = 114 <
		// 120 → Tp 1175 → supratemperate (800–1400).
		name: "Potsdam", wmo: "10379", lat: 52.381, lon: 13.062, elev: 80.9,
		tmean:     [12]float64{0.7, 1.6, 4.7, 9.9, 14.2, 17.4, 19.4, 18.9, 14.6, 9.6, 4.8, 1.7},
		tmin:      [12]float64{-1.9, -1.4, 0.9, 4.7, 8.8, 12.1, 14.4, 14.1, 10.4, 6.2, 2.2, -0.7},
		tmax:      [12]float64{3.2, 4.9, 9.2, 15.7, 20, 23, 25.1, 24.9, 19.9, 13.7, 7.6, 3.9},
		precip:    [12]float64{45.3, 36.1, 39.3, 29.2, 53.3, 60.8, 76.2, 59.2, 47.1, 42.8, 42.3, 46.1},
		wantMacro: MacroTemperate, wantThermotype: "supratemperate",
	},
	{ // WMO 11035 Wien Hohe Warte, AT, 198 m. Step 14: Ios2 3.37 > 2 → temperate.
		// Thermotype not asserted: Ic 20.8 is 0.2 below the It/Tp switch at 21
		// and Tp 1383 is 17 below the supra/meso bound 1400.
		name: "Wien Hohe Warte", wmo: "11035", lat: 48.249, lon: 16.356, elev: 198,
		tmean:     [12]float64{1.1, 2.8, 6.9, 11.9, 16.3, 20, 21.9, 21.6, 16.6, 11.2, 6.2, 1.8},
		tmin:      [12]float64{-1.3, -0.5, 2.6, 6.7, 10.7, 14.7, 15.9, 15.6, 12, 7.3, 3.7, -0.4},
		tmax:      [12]float64{3.5, 6.5, 10.7, 17.2, 20.7, 25.1, 26.4, 26.1, 21.1, 14.3, 8.8, 4},
		precip:    [12]float64{42.1, 38.1, 51.6, 41.8, 78.9, 70, 77.7, 69.1, 64.1, 46.9, 46, 46.8},
		wantMacro: MacroTemperate,
	},
	{ // WMO 2978 Helsinki Kaisaniemi, FI, 3 m. Step 13 (Ic 21.9 in 21–28): T 6.48
		// ≥ 4.8 → not boreal; step 14: Ios2 3.96 → temperate. Ic ≥ 21 → Tp 860 →
		// supratemperate.
		name: "Helsinki Kaisaniemi", wmo: "2978", lat: 60.175, lon: 24.945, elev: 3,
		tmean:     [12]float64{-3.1, -3.8, -0.7, 4.4, 10.4, 14.9, 18.1, 16.9, 12.3, 6.6, 2.4, -0.7},
		tmin:      [12]float64{-5.6, -6.3, -3.6, 1.1, 6.4, 11.2, 14.5, 13.5, 9.3, 4.2, 0.4, -2.9},
		tmax:      [12]float64{-0.7, -1.3, 2.3, 8.1, 14.6, 18.8, 21.9, 20.5, 15.4, 9.2, 4.4, 1.4},
		precip:    [12]float64{52.8, 38.3, 34.4, 34.1, 37.9, 60.1, 57.1, 81.4, 55.7, 73.3, 69.3, 58.3},
		wantMacro: MacroTemperate, wantThermotype: "supratemperate",
	},
	{ // WMO 8391 Sevilla San Pablo, ES, 34 m. Step 14: Io 2.17 (limit 1.9), Ios2
		// 0.06 → mediterranean. Itc 416: 350 < 416 ≤ 450 → thermomediterranean.
		name: "Sevilla San Pablo", wmo: "8391", lat: 37.417, lon: -5.879, elev: 34,
		tmean:     [12]float64{11.2, 12.6, 15.5, 17.6, 21.3, 25.3, 28, 28.1, 24.7, 20.4, 15.1, 12.1},
		tmin:      [12]float64{6.1, 7, 9.5, 11.5, 14.7, 18.2, 20.4, 20.6, 18.2, 14.7, 10, 7.4},
		tmax:      [12]float64{16.2, 18.2, 21.5, 23.7, 27.8, 32.4, 35.7, 35.6, 31.1, 26, 20.1, 16.8},
		precip:    [12]float64{56.3, 46.2, 46.9, 54, 33.9, 5.8, 0.6, 2.5, 33.1, 75.4, 72.2, 77.2},
		wantMacro: MacroMediterranean, wantThermotype: "thermomediterranean",
	},
	{ // WMO 16239 Roma Ciampino, IT, 129 m. Step 14: Io 3.96 (limit 1.8), Ios2
		// 0.99 → mediterranean. Ic 17.96 → Itc = It = 312: 220 < 312 ≤ 350 →
		// mesomediterranean.
		name: "Roma Ciampino", wmo: "16239", lat: 41.808, lon: 12.585, elev: 129,
		tmean:     [12]float64{7.47, 8, 10.7, 13.6, 17.96, 22.48, 25.11, 25.43, 21.02, 16.97, 12.42, 8.52},
		tmin:      [12]float64{3.42, 3.37, 5.86, 8.63, 12.56, 16.74, 19.26, 19.77, 15.99, 12.4, 8.47, 4.67},
		tmax:      [12]float64{12, 12.95, 15.75, 18.81, 23.33, 28.1, 31.02, 31.55, 26.69, 22.16, 16.87, 12.73},
		precip:    [12]float64{65.62, 62.76, 58.58, 68.63, 56.85, 30.08, 19.79, 30.2, 64.86, 88.11, 108.24, 98.31},
		wantMacro: MacroMediterranean, wantThermotype: "mesomediterranean",
	},
	{ // WMO 71018 Resolute, CA, 30 m. Step 11: lat 74.7 > 72 → polar. Tp 77:
		// 1–100 → suprapolar (Fig. 19: the polar sequence is thermo-, meso-,
		// supra- and gelid; there is no oro- or cryoro-polar).
		name: "Resolute", wmo: "71018", lat: 74.716, lon: -94.983, elev: 30,
		tmean:     [12]float64{-31.4, -32.1, -29.4, -21.2, -10, 0.5, 4.8, 2.4, -3.7, -12.2, -21.5, -27.3},
		tmin:      [12]float64{-34.7, -35.4, -32.8, -24.7, -12.9, -1.7, 2, 0.2, -5.6, -15, -24.6, -30.7},
		tmax:      [12]float64{-28, -28.7, -26, -17.7, -7.1, 2.8, 7.6, 4.6, -1.7, -9.4, -18.3, -23.8},
		precip:    [12]float64{4.9, 4.4, 7.4, 8.2, 9.5, 15.7, 29.3, 32.7, 19, 15, 16.4, 6.7},
		wantMacro: MacroPolar, wantThermotype: "suprapolar",
	},
	{ // Amundsen-Scott South Pole, 2835 m: NOT an official WMO normal. 1991–2020
		// mean of the BAS READER monthly means (legacy.bas.ac.uk/met/READER),
		// which carry no Tmin/Tmax and no precipitation; Tmin = Tmax = Tmean and
		// P = 0 stand in (polar thermotypes use Tp only, so neither matters).
		// Step 11: lat −90 < −56 → polar. Tp = 0 → gelid (Fig. 19 "Gélido … 0").
		name: "South Pole", wmo: "", lat: -90, lon: 0, elev: 2835,
		tmean:     [12]float64{-28.1, -40.7, -53.6, -57.4, -57.7, -58.1, -60.2, -59.6, -58.9, -50.9, -37.2, -27.3},
		tmin:      [12]float64{-28.1, -40.7, -53.6, -57.4, -57.7, -58.1, -60.2, -59.6, -58.9, -50.9, -37.2, -27.3},
		tmax:      [12]float64{-28.1, -40.7, -53.6, -57.4, -57.7, -58.1, -60.2, -59.6, -58.9, -50.9, -37.2, -27.3},
		wantMacro: MacroPolar, wantThermotype: "gelid",
	},
}

// TestMacrobioclimate_ReferenceStations pins the WBCS08 key 5.1 result for
// real stations, with the station elevation fed into the altitude reduction.
func TestMacrobioclimate_ReferenceStations(t *testing.T) {
	for _, s := range rmReferenceStations {
		t.Run(s.name, func(t *testing.T) {
			if got := RivasMartinezMacrobioclimate(s.climate(), s.site()); got != s.wantMacro {
				t.Errorf("%s (WMO %s): macrobioclimate = %q, want %q", s.name, s.wmo, got, s.wantMacro)
			}
		})
	}
}

// TestThermotype_ReferenceStations pins the WBCS08 Fig. 19 / P14 Table 7
// thermotype for the stations that are not borderline.
func TestThermotype_ReferenceStations(t *testing.T) {
	for _, s := range rmReferenceStations {
		if s.wantThermotype == "" {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			got := RivasMartinez(s.climate(), s.site())
			if got.Macrobioclimate != s.wantMacro || got.Thermotype.Code != s.wantThermotype {
				t.Errorf("%s (WMO %s): = %q/%q, want %q/%q", s.name, s.wmo,
					got.Macrobioclimate, got.Thermotype.Code, s.wantMacro, s.wantThermotype)
			}
			if got.Thermotype.De == "" || got.Thermotype.En != got.Thermotype.Code {
				t.Errorf("%s: names de=%q en=%q, want de set and en = code", s.name, got.Thermotype.De, got.Thermotype.En)
			}
		})
	}
}

// TestMacrobioclimate_WithoutElevation documents the fallback: without an
// elevation no altitude reduction applies, so a high summit is read as if it
// lay at or below 200 m. Säntis then falls to polar (Tp 234 < 380).
func TestMacrobioclimate_WithoutElevation(t *testing.T) {
	s := rmReferenceStations[0]
	site := s.site()
	site.ElevationKnown = false
	if got := RivasMartinezMacrobioclimate(s.climate(), site); got != MacroPolar {
		t.Errorf("Säntis without elevation = %q, want polar (no altitude reduction)", got)
	}
}

// TestRMIndices_Sevilla cross-checks the computed indices against the
// independent computation documented in the research note.
func TestRMIndices_Sevilla(t *testing.T) {
	var s rmStation
	for _, x := range rmReferenceStations {
		if x.wmo == "8391" {
			s = x
		}
	}
	ix := computeRMIndices(s.climate(), s.lat)
	checks := []struct {
		name      string
		got, want float64
		tol       float64
	}{
		// Hand-checkable: JA is the warmer JJA pair (56.1 vs 53.3 °C), MJJA adds May.
		{"T", ix.T, 231.9 / 12, 1e-9}, {"Ic", ix.Ic, 16.9, 1e-9}, {"It", ix.It, 416.25, 1e-9},
		{"Itc", ix.Itc, 416.25, 1e-9}, {"Tp", ix.Tp, 2319, 1e-9}, {"Io", ix.Io, 504.1 / 2319 * 10, 1e-9},
		{"Ios2", ix.Ios2, 3.1 / 56.1, 1e-9}, {"Ios3", ix.Ios3, 8.9 / 81.4, 1e-9}, {"Ios4", ix.Ios4, 42.8 / 102.7, 1e-9},
		{"Tps", ix.Tps, 814, 1e-9}, {"Tmax", ix.Tmax, 28.1, 1e-9},
		// Pss = Apr–Sep (the warmer half), Psw = Oct–Mar; Cm1 = Jun–Sep is the
		// warmest four-month run, Cm2 = Oct–Jan follows it, Cm3 = Feb–May precedes it.
		{"Pss", ix.Pss, 129.9, 1e-9}, {"Psw", ix.Psw, 374.2, 1e-9},
		{"Pcm1", ix.Pcm1, 42.0, 1e-9}, {"Pcm2", ix.Pcm2, 281.1, 1e-9}, {"Pcm3", ix.Pcm3, 181.0, 1e-9},
		{"m", ix.m, 6.1, 1e-9}, {"M", ix.M, 16.2, 1e-9},
	}
	for _, c := range checks {
		if d := c.got - c.want; d > c.tol || d < -c.tol {
			t.Errorf("%s = %.4f, want %.4f ± %g", c.name, c.got, c.want, c.tol)
		}
	}
}
