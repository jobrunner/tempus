package domain

import "math"

// The Rivas-Martínez macrobioclimate key, implemented per the 2008 online
// edition of the Worldwide Bioclimatic Classification System (WBCS08:
// Rivas-Martínez, Global Bioclimatics, versión 01-12-2008, ch. 5.1 "Clave para
// los macrobioclimas" and its footnote (1)), read from Wayback snapshots of
// globalbioclimatics.org because the site is offline. The print edition
// (Rivas-Martínez, Rivas-Sáenz & Penas 2011, Global Geobotany 1) could not be
// obtained; only its summer-drought compensation table is used, as tabulated
// by Pesaresi et al. 2014 (P14). Every criterion with its quote, URL and the
// interpretations made where the source is silent is in
// docs/research/rivas-martinez-reference.md.

// RivasMartinezSite is the location input of the key: latitude and longitude
// (degrees) and the elevation (m) the monthly normals refer to. ElevationKnown
// is false when no elevation is available; the key then applies no altitude
// reduction (documented fallback).
type RivasMartinezSite struct {
	Lat, Lon       float64
	ElevationM     float64
	ElevationKnown bool
}

// Macrobioclimate is one of the five Rivas-Martínez macrobioclimates; the
// value is the canonical English name.
type Macrobioclimate string

// The five macrobioclimates of the Worldwide Bioclimatic Classification System.
const (
	MacroTropical      Macrobioclimate = "tropical"
	MacroMediterranean Macrobioclimate = "mediterranean"
	MacroTemperate     Macrobioclimate = "temperate"
	MacroBoreal        Macrobioclimate = "boreal"
	MacroPolar         Macrobioclimate = "polar"
)

var macrobioclimateDe = map[Macrobioclimate]string{
	MacroTropical: "tropisch", MacroMediterranean: "mediterran", MacroTemperate: "temperat",
	MacroBoreal: "boreal", MacroPolar: "polar",
}

// De returns the German name of the macrobioclimate.
func (m Macrobioclimate) De() string { return macrobioclimateDe[m] }

// Thresholds of key 5.1, verbatim from WBCS08 (step numbers of the key).
const (
	subtropicalLatDeg = 35.0 // step 2: "entre los paralelos 23° y 35° N y S"

	highAsiaLatMin, highAsiaLatMax = 26.0, 35.0  // step 3: "Eurasia: 26 a 35° N,
	highAsiaLonMin, highAsiaLonMax = 70.0, 120.0 // longitud 70° E a 120° E,
	highAsiaElevM                  = 2000.0      // altitud >= 2.000 m"

	step4T, step4m, step4Itc = 25.0, 10.0, 580.0 // "T >= 25°, … m >= 10°, … It, Itc >= 580"
	step5T, step5M, step5Itc = 21.0, 18.0, 470.0 // "T > 21°, … M > 18°, … It, Itc >= 470"

	polarLatN, polarLatS = 72.0, -56.0 // step 11: "latitud Norte > 72º o … Sur > 56º"
	polarTp              = 380.0       // step 11: "Tp < 380"

	step12Ic, step12Tps, step12Tmax = 11.0, 290.0, 10.0 // "Ic < 11, … Tps <= 290, y … Tmax <= 10º"

	// Footnote (1): "incrementando T en 0.6º, M en 0.5º, e It o Itc en 13
	// unidades, cada 100 m"; north of 48° N / south of 51° S "T en 0.4º y Tp
	// en 12 unidades"; Tp otherwise "incrementando 55 unidades cada 100 m".
	reductionBaseM                       = 200.0
	reduceT, reduceM, reduceItc          = 0.6, 0.5, 13.0
	reduceTHigh, reduceTpHigh            = 0.4, 12.0
	reduceTp                             = 55.0
	highReductionLatN, highReductionLatS = 48.0, -51.0
)

// RivasMartinezMacrobioclimate returns the macrobioclimate of the site from
// its monthly normals (WBCS08 key 5.1).
func RivasMartinezMacrobioclimate(c MonthlyClimate, s RivasMartinezSite) Macrobioclimate {
	return macrobioclimateFrom(computeRMIndices(c, s.Lat), s)
}

// macrobioclimateFrom runs key 5.1 on precomputed indices.
func macrobioclimateFrom(ix rmIndices, s RivasMartinezSite) Macrobioclimate {
	lat := math.Abs(s.Lat)
	switch {
	case lat < rmTropicalLatDeg: // step 1
		return MacroTropical
	case lat <= subtropicalLatDeg: // step 2 → 3
		return subtropicalKey(ix, s)
	default: // step 2 → 11
		return extratropicalKey(ix, s)
	}
}

// subtropicalKey is steps 3–10 (23°–35° N and S).
func subtropicalKey(ix rmIndices, s RivasMartinezSite) Macrobioclimate {
	if inHighAsia(s) { // step 3 → 10
		return temperateOrMediterranean(ix)
	}
	if countTrue(atLeast(ix.T, step4T), atLeast(ix.m, step4m), atLeast(ix.Itc, step4Itc)) >= 2 { // step 4
		return MacroTropical
	}
	e := hundredsAbove200m(s)
	if countTrue(above(ix.T+reduceT*e, step5T), above(ix.M+reduceM*e, step5M), atLeast(ix.Itc+reduceItc*e, step5Itc)) < 2 { // step 5
		return temperateOrMediterranean(ix) // step 8
	}
	if above(ix.Pss, ix.Psw) || !mediterraneanSummer(ix.Io, ix.Ios2, ix.Ios3, ix.Ios4) { // steps 6, 7
		return MacroTropical
	}
	if above(ix.Pcm1, ix.Pcm3) && above(ix.Pcm1, ix.Pcm2) { // step 9: "Pcm3 < Pcm1 > Pcm2"
		return MacroTropical
	}
	return MacroMediterranean
}

// inHighAsia is step 3; it needs a known elevation.
func inHighAsia(s RivasMartinezSite) bool {
	return s.ElevationKnown && s.ElevationM >= highAsiaElevM &&
		s.Lat >= highAsiaLatMin && s.Lat <= highAsiaLatMax &&
		s.Lon >= highAsiaLonMin && s.Lon <= highAsiaLonMax
}

// countTrue counts the true conditions ("al menos dos de …").
func countTrue(conds ...bool) int {
	n := 0
	for _, c := range conds {
		if c {
			n++
		}
	}
	return n
}
