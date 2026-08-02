package domain

// KoppenGeiger classifies the Köppen-Geiger climate from the monthly climate
// normals and latitude, following the criteria of Beck et al. (2018). It
// returns the code (e.g. "Cfb") and a bilingual description.
func KoppenGeiger(c MonthlyClimate, latDeg float64) (code, de, en string) {
	mat := mean12(c.Tmean)
	mapp := sum12(c.Precip)
	thot := max12(c.Tmean)
	tcold := min12(c.Tmean)
	pdry := min12(c.Precip)

	tmon10 := 0
	for _, t := range c.Tmean {
		if t >= 10 {
			tmon10++
		}
	}

	// Summer is the warm half-year (Apr–Sep north, Oct–Mar south).
	summer, winter := halfYears(latDeg)
	pSummer, pSummerMin, pSummerMax := seasonStats(c.Precip, summer)
	pWinter, pWinterMin, pWinterMax := seasonStats(c.Precip, winter)

	// Aridity threshold for B climates.
	var pth float64
	switch {
	case pWinter >= 0.7*mapp:
		pth = 2 * mat
	case pSummer >= 0.7*mapp:
		pth = 2*mat + 28
	default:
		pth = 2*mat + 14
	}

	switch {
	case mapp < 10*pth:
		code = koppenB(mapp, pth, mat)
	case thot < 10:
		code = koppenE(thot)
	case tcold >= 18:
		code = koppenA(pdry, mapp, c.Precip, summer)
	case tcold > 0:
		code = "C" + seasonLetter(pSummerMin, pSummerMax, pWinterMin, pWinterMax) + heatLetter(thot, tmon10, tcold, false)
	default:
		code = "D" + seasonLetter(pSummerMin, pSummerMax, pWinterMin, pWinterMax) + heatLetter(thot, tmon10, tcold, true)
	}

	desc := koppenDescriptions[code]
	return code, desc.de, desc.en
}

func koppenB(mapp, pth, mat float64) string {
	code := "BS"
	if mapp < 5*pth {
		code = "BW"
	}
	if mat >= 18 {
		return code + "h"
	}
	return code + "k"
}

func koppenE(thot float64) string {
	if thot > 0 {
		return "ET"
	}
	return "EF"
}

func koppenA(pdry, mapp float64, precip [12]float64, summer [6]int) string {
	switch {
	case pdry >= 60:
		return "Af"
	case pdry >= 100-mapp/25:
		return "Am"
	default:
		// Dry season: As if the driest month is in the summer half, else Aw.
		if inSeason(argMin12(precip), summer) {
			return "As"
		}
		return "Aw"
	}
}

// seasonLetter returns the precipitation sub-type: s (dry summer), w (dry
// winter) or f (no dry season).
func seasonLetter(pSummerMin, pSummerMax, pWinterMin, pWinterMax float64) string {
	if pSummerMin < 40 && pSummerMin < pWinterMax/3 {
		return "s"
	}
	if pWinterMin < pSummerMax/10 {
		return "w"
	}
	return "f"
}

// heatLetter returns the temperature sub-type: a/b/c, plus d for very cold
// winters (D climates only).
func heatLetter(thot float64, tmon10 int, tcold float64, allowD bool) string {
	if allowD && tcold < -38 {
		return "d"
	}
	switch {
	case thot >= 22:
		return "a"
	case tmon10 >= 4:
		return "b"
	default:
		return "c"
	}
}

// halfYears returns the summer and winter month indices (0=Jan) for the
// hemisphere. Northern summer is Apr–Sep; southern summer is Oct–Mar.
func halfYears(latDeg float64) (summer, winter [6]int) {
	north := [6]int{3, 4, 5, 6, 7, 8}
	south := [6]int{9, 10, 11, 0, 1, 2}
	if latDeg < 0 {
		return south, north
	}
	return north, south
}

func seasonStats(precip [12]float64, months [6]int) (total, minVal, maxVal float64) {
	minVal, maxVal = precip[months[0]], precip[months[0]]
	for _, m := range months {
		v := precip[m]
		total += v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	return total, minVal, maxVal
}

func inSeason(month int, months [6]int) bool {
	for _, m := range months {
		if m == month {
			return true
		}
	}
	return false
}

type koppenDesc struct{ de, en string }

// koppenDescriptions maps each Köppen-Geiger code to a bilingual description.
var koppenDescriptions = map[string]koppenDesc{
	"Af":  {"tropisches Regenwaldklima", "tropical rainforest"},
	"Am":  {"tropisches Monsunklima", "tropical monsoon"},
	"Aw":  {"tropisches Savannenklima (Wintertrocken)", "tropical savanna, dry winter"},
	"As":  {"tropisches Savannenklima (Sommertrocken)", "tropical savanna, dry summer"},
	"BWh": {"heißes Wüstenklima", "hot desert"},
	"BWk": {"kaltes Wüstenklima", "cold desert"},
	"BSh": {"heißes Steppenklima", "hot semi-arid (steppe)"},
	"BSk": {"kaltes Steppenklima", "cold semi-arid (steppe)"},
	"Csa": {"warm-gemäßigt, trockener heißer Sommer (mediterran)", "hot-summer Mediterranean"},
	"Csb": {"warm-gemäßigt, trockener warmer Sommer", "warm-summer Mediterranean"},
	"Csc": {"warm-gemäßigt, trockener kühler Sommer", "cold-summer Mediterranean"},
	"Cwa": {"warm-gemäßigt, wintertrocken, heißer Sommer", "monsoon-influenced humid subtropical"},
	"Cwb": {"warm-gemäßigt, wintertrocken, warmer Sommer", "subtropical highland, dry winter"},
	"Cwc": {"warm-gemäßigt, wintertrocken, kühler Sommer", "cold subtropical highland, dry winter"},
	"Cfa": {"warm-gemäßigt, feucht, heißer Sommer", "humid subtropical"},
	"Cfb": {"warm-gemäßigt, feucht, warmer Sommer (ozeanisch)", "temperate oceanic"},
	"Cfc": {"warm-gemäßigt, feucht, kühler Sommer (subpolar-ozeanisch)", "subpolar oceanic"},
	"Dsa": {"kontinental, trockener heißer Sommer", "hot-summer humid continental, dry summer"},
	"Dsb": {"kontinental, trockener warmer Sommer", "warm-summer humid continental, dry summer"},
	"Dsc": {"kontinental, trockener kühler Sommer (subarktisch)", "dry-summer subarctic"},
	"Dsd": {"kontinental, trockener Sommer, sehr kalter Winter", "dry-summer subarctic, very cold winter"},
	"Dwa": {"kontinental, wintertrocken, heißer Sommer", "monsoon humid continental, dry winter"},
	"Dwb": {"kontinental, wintertrocken, warmer Sommer", "warm-summer humid continental, dry winter"},
	"Dwc": {"kontinental, wintertrocken, kühler Sommer (subarktisch)", "dry-winter subarctic"},
	"Dwd": {"kontinental, wintertrocken, sehr kalter Winter", "dry-winter subarctic, very cold winter"},
	"Dfa": {"kontinental, feucht, heißer Sommer", "hot-summer humid continental"},
	"Dfb": {"kontinental, feucht, warmer Sommer", "warm-summer humid continental"},
	"Dfc": {"kontinental, feucht, kühler Sommer (subarktisch)", "subarctic"},
	"Dfd": {"kontinental, feucht, sehr kalter Winter", "subarctic, very cold winter"},
	"ET":  {"Tundrenklima", "tundra"},
	"EF":  {"Eisklima (Dauerfrost)", "ice cap"},
}
