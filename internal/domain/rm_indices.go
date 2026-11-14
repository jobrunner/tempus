package domain

import "math"

// Bioclimatic parameters and indices of the Rivas-Martínez Worldwide
// Bioclimatic Classification System, as defined in WBCS08 (Rivas-Martínez,
// Global Bioclimatics, versión 01-12-2008, ch. 2) and P14 (Pesaresi et al.
// 2014, J. Maps 10(4), Table 1, "according to Rivas-Martínez et al. 2011").
// Criteria, quotes and URLs: docs/research/rivas-martinez-reference.md.

// rmTropicalLatDeg bounds the intertropical belt ("entre los paralelos 23º N y
// S"); beyond it Itc replaces It ("zonas extratropicales … al norte y al sur
// del paralelo 23º N y S", WBCS08 ch. 2d).
const rmTropicalLatDeg = 23.0

// rmIndices bundles the parameters and indices the macrobioclimate key and the
// thermotype read. Temperatures in °C; It, Itc, Tp and Tps in tenths of °C.
type rmIndices struct {
	T, Ic, It, Itc, Tp, Io float64 // annual mean, continentality, thermicity, compensated thermicity, positive temperature, ombrothermic index
	m, M                   float64 // mean minimum / maximum of the coldest month
	Tps, Tmax              float64 // positive temperature of the summer quarter, mean of the warmest month
	Ios2, Ios3, Ios4       float64 // summer ombrothermic indices (warmest summer bimonth, summer quarter, + month before)
	Pss, Psw               float64 // precipitation of the warmer / colder half-year
	Pcm1, Pcm2, Pcm3       float64 // precipitation of the warmest four months, the four after, the four before
}

// computeRMIndices derives the indices from the monthly normals. latDeg picks
// the hemisphere's summer quarter and whether Itc is compensated.
func computeRMIndices(c MonthlyClimate, latDeg float64) rmIndices {
	ci := argMin12(c.Tmean)
	ix := rmIndices{
		T:    mean12(c.Tmean),
		Ic:   max12(c.Tmean) - min12(c.Tmean),
		It:   ThermicityIndex(c),
		Tp:   PositiveTemperature(c),
		m:    c.Tmin[ci],
		M:    c.Tmax[ci],
		Tmax: max12(c.Tmean),
	}
	ix.Itc = ix.It
	if math.Abs(latDeg) >= rmTropicalLatDeg {
		ix.Itc += itcCompensation(ix.Ic)
	}
	ix.Io = annualOmbrothermic(c, ix.Tp)
	summerIndices(c, latDeg, &ix)
	ix.Pss, ix.Psw = warmColdHalfPrecip(c)
	ix.Pcm1, ix.Pcm2, ix.Pcm3 = fourMonthPrecip(c)
	return ix
}

// itcCompensation is the compensation value Ci of WBCS08 Fig. 7: Itc = It + Ci.
// Ic < 8: C0 = −10·(8 − Ic); 8–18: 0; 18–21: 5·(Ic − 18); 21–28: 15 + 15·(Ic − 21);
// 28–46: 120 + 25·(Ic − 28); above 46: 570 + 30·(Ic − 46). Fig. 7 ends at Ic 65;
// the last factor is applied beyond it.
func itcCompensation(ic float64) float64 {
	switch {
	case ic < 8:
		return -10 * (8 - ic)
	case ic <= 18:
		return 0
	case ic <= 21:
		return 5 * (ic - 18)
	case ic <= 28:
		return 15 + 15*(ic-21)
	case ic <= 46:
		return 120 + 25*(ic-28)
	default:
		return 570 + 30*(ic-46)
	}
}

// warmColdHalfPrecip returns the precipitation of the warmer and the colder
// half-year (Pss, Psw). The half-years are April–September and October–March,
// the semesters of WBCS08's semestral ombrothermic index Iosm ("N: 4-9",
// "N: 10-3"); the warmer one by temperature is Pss. That is the split
// halfYears already makes for Köppen (April–September on a tie).
func warmColdHalfPrecip(c MonthlyClimate) (pss, psw float64) {
	warm, cold := halfYears(c.Tmean)
	return seasonStats(c.Precip, warm).total, seasonStats(c.Precip, cold).total
}

// fourMonthPrecip returns Pcm1, Pcm2, Pcm3: the precipitation of the warmest
// run of four consecutive months (Cm1, wrapping the year; the earliest start
// on a tie), of the four months after it (Cm2) and of the four before it (Cm3)
// (WBCS08 ch. 2a).
func fourMonthPrecip(c MonthlyClimate) (pcm1, pcm2, pcm3 float64) {
	start, best := 0, sum4(c.Tmean, 0)
	for i := 1; i < 12; i++ {
		if t := sum4(c.Tmean, i); above(t, best) {
			start, best = i, t
		}
	}
	return sum4(c.Precip, start), sum4(c.Precip, start+4), sum4(c.Precip, start+8)
}

// sum4 sums four consecutive months of x from start, wrapping the year.
func sum4(x [12]float64, start int) float64 {
	var s float64
	for k := range 4 {
		s += x[(start+k)%12]
	}
	return s
}
