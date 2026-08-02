package domain

import "math"

// BeltSource is the attribution for the altitudinal-belt computation.
const BeltSource = "Höhenstufe thermoklimatisch berechnet (Paulsen & Körner 2014; Rivas-Martínez 2004), tempus"

// Altitudinal-belt isotherm thresholds (°C), calibrated to the Central-European
// (Alpine) convention so the classic names match FHL/Hegi usage, then applied
// globally. The treeline value 6.4 °C is Paulsen & Körner (2014); Köppen's tree
// limit informs the upper (10 °C) bound.
const (
	beltNivalTwarm       = 0.0  // warmest month ≤ 0 → permanent snow
	beltSubnivalTwarm    = 5.0  // warmest month ≤ 5 → sparse pioneer vegetation
	beltTreelineTwq      = 6.4  // warmest-quarter mean ≤ 6.4 → above treeline (alpine)
	beltSubalpineTwq     = 10.0 // warmest-quarter mean ≤ 10 → subalpine (upper forest)
	beltHighMontaneMAT   = 4.0
	beltMontaneMAT       = 7.0
	beltCollineMAT       = 11.0
	beltBorderlineMargin = 0.5 // °C to the nearest boundary → flagged borderline
)

// Belt is the classic altitudinal belt at a location, derived from climate.
// AdjDe/AdjEn name the nearer neighbouring belt when Borderline is true.
type Belt struct {
	De, En          string
	Borderline      bool
	AdjDe, AdjEn    string
	WarmestMonthC   float64
	WarmestQuarterC float64
	MATC            float64
	BiotemperatureC float64
}

type beltName struct{ de, en string }

// German belt names (constants so the literals live in one place, shared with tests).
const (
	nameNival      = "nival"
	nameSubnival   = "subnival"
	nameAlpin      = "alpin"
	nameSubalpin   = "subalpin"
	nameHochmontan = "hochmontan"
	nameMontan     = "montan"
	nameKollin     = "submontan-kollin"
	namePlanar     = "planar"
)

// beltsColdToWarm lists the belts from coldest to warmest.
var beltsColdToWarm = []beltName{
	{nameNival, "nival"},
	{nameSubnival, "subnival"},
	{nameAlpin, "alpine"},
	{nameSubalpin, "subalpine"},
	{nameHochmontan, "high-montane"},
	{nameMontan, "montane"},
	{nameKollin, "colline"},
	{namePlanar, "lowland"},
}

// beltIndicator selects which indicator governs a boundary.
type beltIndicator int

const (
	indTwarm beltIndicator = iota
	indTwq
	indMAT
)

// beltBoundary k separates beltsColdToWarm[k] (colder) from [k+1] (warmer).
type beltBoundary struct {
	ind       beltIndicator
	threshold float64
}

var beltBoundaries = []beltBoundary{
	{indTwarm, beltNivalTwarm},    // nival | subnival
	{indTwarm, beltSubnivalTwarm}, // subnival | alpine
	{indTwq, beltTreelineTwq},     // alpine | subalpine
	{indTwq, beltSubalpineTwq},    // subalpine | high-montane
	{indMAT, beltHighMontaneMAT},  // high-montane | montane
	{indMAT, beltMontaneMAT},      // montane | colline
	{indMAT, beltCollineMAT},      // colline | planar
}

// AltitudinalBelt classifies the climate-derived altitudinal belt from the
// monthly normals, with the deciding indicators and a borderline flag.
func AltitudinalBelt(c MonthlyClimate) Belt {
	twarm := max12(c.Tmean)
	twq := warmestQuarterMean(c.Tmean)
	mat := mean12(c.Tmean)
	tbio := biotemperature(c.Tmean)

	idx := beltIndex(twarm, twq, mat)
	b := Belt{
		De: beltsColdToWarm[idx].de, En: beltsColdToWarm[idx].en,
		WarmestMonthC: twarm, WarmestQuarterC: twq, MATC: mat, BiotemperatureC: tbio,
	}

	// Borderline: nearest adjacent boundary within the margin.
	best := math.Inf(1)
	for _, k := range []int{idx - 1, idx} { // lower (to colder) and upper (to warmer) boundary
		if k < 0 || k >= len(beltBoundaries) {
			continue
		}
		val := indicatorValue(beltBoundaries[k].ind, twarm, twq, mat)
		if d := math.Abs(val - beltBoundaries[k].threshold); d < beltBorderlineMargin && d < best {
			best = d
			neighbour := k // boundary k borders belt k and k+1; the neighbour is the other one
			if k == idx {
				neighbour = idx + 1
			}
			b.Borderline = true
			b.AdjDe, b.AdjEn = beltsColdToWarm[neighbour].de, beltsColdToWarm[neighbour].en
		}
	}
	return b
}

func beltIndex(twarm, twq, mat float64) int {
	switch {
	case twarm <= beltNivalTwarm:
		return 0 // nival
	case twarm <= beltSubnivalTwarm:
		return 1 // subnival
	case twq <= beltTreelineTwq:
		return 2 // alpine
	case twq <= beltSubalpineTwq:
		return 3 // subalpine
	case mat <= beltHighMontaneMAT:
		return 4 // high-montane
	case mat <= beltMontaneMAT:
		return 5 // montane
	case mat <= beltCollineMAT:
		return 6 // colline
	default:
		return 7 // planar
	}
}

func indicatorValue(ind beltIndicator, twarm, twq, mat float64) float64 {
	switch ind {
	case indTwarm:
		return twarm
	case indTwq:
		return twq
	default:
		return mat
	}
}

// warmestQuarterMean returns the mean temperature of the warmest quarter (the
// warmest run of three consecutive months, wrapping around the year).
func warmestQuarterMean(tmean [12]float64) float64 {
	best := math.Inf(-1)
	for i := 0; i < 12; i++ {
		q := (tmean[i] + tmean[(i+1)%12] + tmean[(i+2)%12]) / 3
		if q > best {
			best = q
		}
	}
	return best
}

// biotemperature is Holdridge's mean annual biotemperature: the mean of monthly
// mean temperatures clamped to [0, 30] °C.
func biotemperature(tmean [12]float64) float64 {
	var sum float64
	for _, t := range tmean {
		sum += math.Max(0, math.Min(30, t))
	}
	return sum / 12
}
