package domain

// Steps 11–14 of the WBCS08 macrobioclimate key (beyond 35° N and S) and the
// altitude reduction to 200 m of its footnote (1). Sources:
// docs/research/rivas-martinez-reference.md.

// extratropicalKey is steps 11–14 (beyond 35° N and S).
func extratropicalKey(ix rmIndices, s RivasMartinezSite) Macrobioclimate {
	if s.Lat > polarLatN || s.Lat < polarLatS { // step 11, latitude
		return MacroPolar
	}
	t, tp := reducedTTp(ix, s)
	if tp < polarTp { // step 11, Tp at 200 m; Tp = 380 goes on to step 12
		return MacroPolar
	}
	if ix.Ic < step12Ic && ix.Tps <= step12Tps && ix.Tmax <= step12Tmax { // step 12
		return MacroBoreal
	}
	if borealByContinentality(ix.Ic, t, tp) { // step 13
		return MacroBoreal
	}
	return temperateOrMediterranean(ix) // step 14
}

// borealByContinentality is step 13 (T and Tp at 200 m): "Ic < 21, T < 5.3º y
// Tp < 720; … Ic = 21-28, T < 4.8º y Tp < 740; … Ic > 28, T < 3.8º y Tp < 800".
func borealByContinentality(ic, t, tp float64) bool {
	switch {
	case ic < 21:
		return t < 5.3 && tp < 720
	case ic <= 28:
		return t < 4.8 && tp < 740
	default:
		return t < 3.8 && tp < 800
	}
}

// reducedTTp returns T and Tp reduced to 200 m (footnote (1)).
func reducedTTp(ix rmIndices, s RivasMartinezSite) (t, tp float64) {
	e := hundredsAbove200m(s)
	if s.Lat > highReductionLatN || s.Lat < highReductionLatS {
		return ix.T + reduceTHigh*e, ix.Tp + reduceTpHigh*e
	}
	return ix.T + reduceT*e, ix.Tp + reduceTp*e
}

// hundredsAbove200m is the number of 100 m steps the site lies above 200 m
// (footnote (1): "cada 100 m que supere dicha altitud"); 0 at or below 200 m
// and when the elevation is unknown.
func hundredsAbove200m(s RivasMartinezSite) float64 {
	if !s.ElevationKnown || s.ElevationM <= reductionBaseM {
		return 0
	}
	return (s.ElevationM - reductionBaseM) / 100
}
