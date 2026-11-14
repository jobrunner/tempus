package domain

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

// Thermotype is a Rivas-Martínez thermotype with the index that decided it.
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

// RivasMartinez classifies the site.
func RivasMartinez(c MonthlyClimate, s RivasMartinezSite) RivasMartinezClass {
	return RivasMartinezClass{}
}

func thermotypeFor(ix rmIndices, mb Macrobioclimate, latDeg float64) Thermotype {
	return Thermotype{}
}
