package domain

import "math"

// Threshold comparisons robust to float summation roundoff.
//
// Climate normals carry one decimal, and the classifiers compare sums and
// means of them (Tp, T, Ic, Io, Ios, seasonal totals …) with published bounds.
// Such a value can be exact in decimal and still miss the bound in binary:
// Craiova's one-decimal months give Tp = 1400.0000000000002 for an exact 1400,
// and months totalling 404 mm sum to 403.99999999999994. A value within
// roundoffTolerance of a bound therefore counts as ON the bound, and the
// source's own strict or non-strict inequality decides. Exact values keep their meaning: above(b, b) is false,
// atLeast(b, b) is true. 1e-9 (relative to the bound, but at least absolute)
// is far below any real difference in normals (0.1 on 1400 is 7e-5).
const roundoffTolerance = 1e-9

// above reports x > bound beyond roundoff. bound must be finite.
func above(x, bound float64) bool {
	return x-bound > roundoffTolerance*max(1, math.Abs(bound))
}

// atLeast reports x ≥ bound up to roundoff. bound must be finite.
func atLeast(x, bound float64) bool {
	return bound-x <= roundoffTolerance*max(1, math.Abs(bound))
}

// below reports x < bound beyond roundoff (not atLeast).
func below(x, bound float64) bool { return !atLeast(x, bound) }

// atMost reports x ≤ bound up to roundoff (not above).
func atMost(x, bound float64) bool { return !above(x, bound) }
