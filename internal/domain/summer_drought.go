package domain

import "math"

// The summer-drought test that separates the Mediterranean from the temperate
// (and, in the subtropical belt, the tropical) macrobioclimate, and the
// ombrothermic indices it reads. Sources: WBCS08 ch. 2a/2c and P14 Tables 1–2;
// docs/research/rivas-martinez-reference.md.

// temperateOrMediterranean is steps 8, 10 and 14: "Verano sin déficit hídrico;
// Ios2 > 2 o … Iosc > 2" → temperate, otherwise Mediterranean.
func temperateOrMediterranean(ix rmIndices) Macrobioclimate {
	if mediterraneanSummer(ix.Io, ix.Ios2, ix.Ios3, ix.Ios4) {
		return MacroMediterranean
	}
	return MacroTemperate
}

// iosCompensation is P14 Table 2 ("Compensation threshold values derived from
// Rivas-Martínez et al. (2011)"): per upper bound of the annual Io, the Ios2
// and Ios3 limits; the Ios4 limit is 2.0 in every row. WBCS08 Fig. 6 has the
// same values from Io 2.0 up. "–" cells (Io ≤ 2.8 for Ios3, Io ≤ 2.0 for both)
// are taken as the uncompensated 2.0.
var iosCompensation = []struct{ ioMax, ios2, ios3 float64 }{
	{2.0, 2.0, 2.0}, {2.8, 1.9, 2.0}, {3.6, 1.8, 1.9}, {4.8, 1.8, 1.9},
	{6.0, 1.7, 1.9}, {7.0, 1.5, 1.8}, {8.0, 1.4, 1.8}, {9.0, 1.3, 1.8},
	{10.0, 1.2, 1.8}, {11.0, 1.1, 1.7}, {12.0, 1.0, 1.7}, {math.Inf(1), 0.9, 1.7},
}

// iosLimits returns the Ios2 and Ios3 limits for the annual Io. P14 writes the
// rows as "2.01–2.8", so each upper bound is inclusive.
func iosLimits(io float64) (lim2, lim3 float64) {
	for _, r := range iosCompensation {
		if io <= r.ioMax {
			return r.ios2, r.ios3
		}
	}
	return 2, 2 // only a NaN Io gets here: no compensation
}

// mediterraneanSummer reports a Mediterranean summer drought, following the
// procedure under P14 Table 2: "Ios2 values >2 always identify the temperate
// macrobioclimate. … Ios2 values lower than the value indicated … identifies
// the mediterranean … between 2 and the value …: if Ios3 is lower than those
// shown …, mediterranean, otherwise if it is >2, temperate, and if Ios3 is
// between … consider Ios4. If this is <2, … mediterranean, if not … temperate".
func mediterraneanSummer(io, ios2, ios3, ios4 float64) bool {
	lim2, lim3 := iosLimits(io)
	switch {
	case ios2 > 2:
		return false
	case ios2 < lim2, ios3 < lim3:
		return true
	case ios3 > 2:
		return false
	default:
		return ios4 < 2
	}
}

// annualOmbrothermic is Io = (Pp/Tp)·10: precipitation of the months above
// 0 °C over their positive temperature (tenths). 0 when Tp is 0.
func annualOmbrothermic(c MonthlyClimate, tp float64) float64 {
	if tp <= 0 {
		return 0
	}
	var pp float64
	for i, t := range c.Tmean {
		if t > 0 {
			pp += c.Precip[i]
		}
	}
	return pp / tp * 10
}

// ombrothermic is the ombrothermic index of the given months, (Pp/Tp)·10 with
// Tp in tenths, i.e. their positive precipitation over their positive
// temperature in °C. Months at or below 0 °C count neither. Without a positive
// month it is +Inf: the Mediterranean drought test "P ≤ 2T" (WBCS08 ch. 2c)
// cannot hold when T ≤ 0.
func ombrothermic(c MonthlyClimate, months ...int) float64 {
	var p, t float64
	for _, i := range months {
		if c.Tmean[i] > 0 {
			p += c.Precip[i]
			t += c.Tmean[i]
		}
	}
	if t <= 0 {
		return math.Inf(1)
	}
	return p / t
}

// summerIndices fills Tps, Ios2, Ios3 and Ios4. The summer quarter Tr3 is June–
// August in the north and December–February in the south, the month before it
// May or November (WBCS08 ch. 2a/2c; P14 Table 1). Ios2 takes the warmer of
// the two consecutive two-month runs in Tr3 (the earlier on a tie).
func summerIndices(c MonthlyClimate, latDeg float64, ix *rmIndices) {
	pre, q := 4, [3]int{5, 6, 7}
	if latDeg < 0 {
		pre, q = 10, [3]int{11, 0, 1}
	}
	a, b := q[0], q[1]
	if c.Tmean[q[1]]+c.Tmean[q[2]] > c.Tmean[q[0]]+c.Tmean[q[1]] {
		a, b = q[1], q[2]
	}
	ix.Ios2 = ombrothermic(c, a, b)
	ix.Ios3 = ombrothermic(c, q[0], q[1], q[2])
	ix.Ios4 = ombrothermic(c, pre, q[0], q[1], q[2])
	for _, i := range q {
		ix.Tps += math.Max(c.Tmean[i], 0) * 10
	}
}
