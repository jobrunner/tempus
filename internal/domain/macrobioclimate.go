package domain

// RivasMartinezSite is the location input of the Rivas-Martínez key: latitude
// and longitude (degrees) and the elevation (m) the monthly normals refer to.
// ElevationKnown is false when no elevation is available.
type RivasMartinezSite struct {
	Lat, Lon       float64
	ElevationM     float64
	ElevationKnown bool
}

// Macrobioclimate is one of the five Rivas-Martínez macrobioclimates.
type Macrobioclimate string

// The five macrobioclimates of the Worldwide Bioclimatic Classification System.
const (
	MacroTropical      Macrobioclimate = "tropical"
	MacroMediterranean Macrobioclimate = "mediterranean"
	MacroTemperate     Macrobioclimate = "temperate"
	MacroBoreal        Macrobioclimate = "boreal"
	MacroPolar         Macrobioclimate = "polar"
)

// De returns the German name of the macrobioclimate.
func (m Macrobioclimate) De() string { return "" }

// RivasMartinezMacrobioclimate returns the macrobioclimate of the site.
func RivasMartinezMacrobioclimate(c MonthlyClimate, s RivasMartinezSite) Macrobioclimate {
	return ""
}

// rmIndices bundles the bioclimatic parameters and indices the key reads.
type rmIndices struct {
	T, Ic, It, Itc, Tp, Io float64
	m, M                   float64
	Tps, Tmax              float64
	Ios2, Ios3, Ios4       float64
	Pss, Psw               float64
	Pcm1, Pcm2, Pcm3       float64
}

func computeRMIndices(c MonthlyClimate, latDeg float64) rmIndices { return rmIndices{} }

func macrobioclimateFrom(ix rmIndices, s RivasMartinezSite) Macrobioclimate { return "" }

func itcCompensation(ic float64) float64 { return 0 }

func mediterraneanSummer(io, ios2, ios3, ios4 float64) bool { return false }
func iosLimits(io float64) (lim2, lim3 float64)             { return 0, 0 }
