package domain

import "strings"

// Rivas-Martínez thermotype horizon boundaries by annual positive temperature
// Tp (°C×10), from the Worldwide Bioclimatic Classification System (Rivas-
// Martínez 2011; boundaries as tabulated in "Bioclimate of Italy"). The same Tp
// ladder is applied across macrobioclimates (a documented v1 simplification);
// the macrobioclimate only sets the name suffix.
const (
	thermoTypeMesoTp  = 2000.0 // Tp > 2000 → thermo-
	thermoTypeSupraTp = 1400.0 // 1400 < Tp ≤ 2000 → meso-
	thermoTypeOroTp   = 800.0  // 800 < Tp ≤ 1400 → supra-
	thermoTypeCryoTp  = 380.0  // 380 < Tp ≤ 800 → oro-;  ≤ 380 → cryoro-
)

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

// thermotypeHorizon maps Tp to the horizon prefix.
func thermotypeHorizon(tp float64) string {
	switch {
	case tp > thermoTypeMesoTp:
		return "thermo"
	case tp > thermoTypeSupraTp:
		return "meso"
	case tp > thermoTypeOroTp:
		return "supra"
	case tp > thermoTypeCryoTp:
		return "oro"
	default:
		return "cryoro"
	}
}

// macrobioclimateSuffix derives the Rivas-Martínez macrobioclimate name suffix
// from the Köppen class (via KoppenGeiger, which already accounts for the
// hemisphere when defining the summer/winter half-years).
func macrobioclimateSuffix(c MonthlyClimate, latDeg float64) (de, en string) {
	code, _, _ := KoppenGeiger(c, latDeg)
	switch {
	case strings.HasPrefix(code, "A"):
		return "tropisch", "tropical"
	case len(code) >= 2 && code[1] == 's': // Köppen dry-summer (Cs/Ds) → Mediterranean
		return "mediterran", "mediterranean"
	case strings.HasPrefix(code, "E"):
		return "polar", "polar"
	case strings.HasPrefix(code, "D") && len(code) >= 3 && (code[2] == 'c' || code[2] == 'd'):
		return "boreal", "boreal"
	default:
		return "temperat", "temperate"
	}
}

// RivasMartinezThermotype returns the thermotype code (canonical English, e.g.
// "supratemperate") and its bilingual name, from the monthly normals.
func RivasMartinezThermotype(c MonthlyClimate, latDeg float64) (code, de, en string) {
	prefix := thermotypeHorizon(PositiveTemperature(c))
	sufDe, sufEn := macrobioclimateSuffix(c, latDeg)
	return prefix + sufEn, prefix + sufDe, prefix + sufEn
}
