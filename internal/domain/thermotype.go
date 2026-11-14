package domain

import "math"

// Rivas-Martínez thermotypes per macrobioclimate, from WBCS08 Fig. 19
// (the thermotypic horizons of the tropical, Mediterranean, temperate, boreal
// and polar macrobioclimates; each thermotype is the union of its lower and
// upper horizon) and, for the Mediterranean and temperate columns, P14
// Table 7 ("according to Rivas-Martínez et al. (2011)"), which agrees. P14
// writes the intervals with an exclusive lower bound (thermotemperate Itc
// 291–350, Tp 2001–2175), so a value equal to a bound belongs to the colder
// thermotype. Sources and URLs: docs/research/rivas-martinez-reference.md.

// Thermotype prefixes (WBCS08 ch. 3: "infra-, termo-, meso-, supra-, oro-,
// crioro- y gélido"), in English.
const (
	prefixInfra  = "infra"
	prefixThermo = "thermo"
	prefixMeso   = "meso"
	prefixSupra  = "supra"
	prefixOro    = "oro"
	prefixCryoro = "cryoro"
	gelid        = "gelid" // "Gélido": Tp = 0 in every macrobioclimate

	// thermotypeTpIt, thermotypeTpIc: at any latitude, Tp decides the thermotype
	// when It (Itc outside 23° N/S) is below 120 or Ic is 21 or more (WBCS08
	// ch. 3; P14 Table 7: "Tp is used if Ic ≥ 21 or It, Itc < 120").
	thermotypeTpIt = 120.0
	thermotypeTpIc = 21.0

	indexIt, indexItc, indexTp = "It", "Itc", "Tp"
)

// thermoBound is one rung of a ladder: values above lower get prefix.
type thermoBound struct {
	lower  float64
	prefix string
}

// itLadders are the It/Itc columns of Fig. 19. Boreal and polar have none
// ("-"): their thermotype always comes from Tp. The lowest rung is the
// thermotype that reaches down to It/Itc 120, below which Tp takes over; its
// bound 0 only closes the ladder (the It ladder is entered at It/Itc ≥ 120).
var itLadders = map[Macrobioclimate][]thermoBound{
	MacroTropical:      {{710, prefixInfra}, {490, prefixThermo}, {320, prefixMeso}, {160, prefixSupra}, {0, prefixOro}},
	MacroMediterranean: {{450, prefixInfra}, {350, prefixThermo}, {220, prefixMeso}, {0, prefixSupra}},
	MacroTemperate:     {{410, prefixInfra}, {290, prefixThermo}, {190, prefixMeso}, {0, prefixSupra}},
}

// tpLadders are the Tp columns of Fig. 19; every ladder ends with Tp > 0, and
// Tp = 0 is gelid. The polar sequence is "termo-, meso-, supra- y gélido".
var tpLadders = map[Macrobioclimate][]thermoBound{
	MacroTropical:      {{2900, prefixInfra}, {2300, prefixThermo}, {1700, prefixMeso}, {950, prefixSupra}, {450, prefixOro}, {0, prefixCryoro}},
	MacroMediterranean: {{2400, prefixInfra}, {2100, prefixThermo}, {1500, prefixMeso}, {900, prefixSupra}, {450, prefixOro}, {0, prefixCryoro}},
	MacroTemperate:     {{2350, prefixInfra}, {2000, prefixThermo}, {1400, prefixMeso}, {800, prefixSupra}, {380, prefixOro}, {0, prefixCryoro}},
	MacroBoreal:        {{680, prefixThermo}, {580, prefixMeso}, {480, prefixSupra}, {380, prefixOro}, {0, prefixCryoro}},
	MacroPolar:         {{280, prefixThermo}, {100, prefixMeso}, {0, prefixSupra}},
}

// ThermicityIndex returns the Rivas-Martínez thermicity index
// It = (T + M + m) × 10, where T is the annual mean temperature and M and m are
// the mean daily maximum and minimum of the coldest month.
func ThermicityIndex(c MonthlyClimate) float64 {
	ci := argMin12(c.Tmean)
	return (mean12(c.Tmean) + c.Tmax[ci] + c.Tmin[ci]) * 10
}

// PositiveTemperature returns the annual positive temperature Tp = 10 × Σ of the
// monthly mean temperatures that are above 0 °C.
func PositiveTemperature(c MonthlyClimate) float64 {
	var s float64
	for _, t := range c.Tmean {
		if t > 0 {
			s += t
		}
	}
	return s * 10
}

// Thermotype is a Rivas-Martínez thermotype: the code (canonical English, e.g.
// "supratemperate" or "gelid"), its German and English names, and the index
// that decided it ("It", "Itc" or "Tp") with its value.
type Thermotype struct {
	Code, De, En string
	Index        string
	Value        float64
}

// RivasMartinezClass is the macrobioclimate and thermotype of a site.
type RivasMartinezClass struct {
	Macrobioclimate Macrobioclimate
	Thermotype      Thermotype
}

// RivasMartinez classifies the site: the macrobioclimate by WBCS08 key 5.1,
// then the thermotype on that macrobioclimate's ladder. The thermotype uses
// the site's own values; only the key reduces them to 200 m.
func RivasMartinez(c MonthlyClimate, s RivasMartinezSite) RivasMartinezClass {
	ix := computeRMIndices(c, s.Lat)
	mb := macrobioclimateFrom(ix, s)
	return RivasMartinezClass{Macrobioclimate: mb, Thermotype: thermotypeFor(ix, mb, s.Lat)}
}

// thermotypeFor selects the deciding index and walks the ladder.
func thermotypeFor(ix rmIndices, mb Macrobioclimate, latDeg float64) Thermotype {
	index, value := indexIt, ix.It
	if math.Abs(latDeg) >= rmTropicalLatDeg {
		index, value = indexItc, ix.Itc
	}
	ladder, hasIt := itLadders[mb]
	if !hasIt || below(value, thermotypeTpIt) || atLeast(ix.Ic, thermotypeTpIc) {
		index, value, ladder = indexTp, ix.Tp, tpLadders[mb]
	}
	t := Thermotype{Code: gelid, De: gelid, En: gelid, Index: index, Value: value}
	for _, b := range ladder {
		if above(value, b.lower) {
			t.Code, t.De, t.En = b.prefix+string(mb), b.prefix+mb.De(), b.prefix+string(mb)
			break
		}
	}
	return t
}
