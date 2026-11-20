package reanalysis

import "math"

// intervalExtrema bounds the Hermite curve between b and c on t ∈ [0, 1],
// with tangents (c−a)/2 and (d−b)/2. The cubic's derivative is solved with
// the numerically stable quadratic formula.
func intervalExtrema(a, b, c, d float64) (lo, hi float64, ok bool) {
	if math.IsNaN(b) || math.IsNaN(c) {
		return 0, 0, false
	}
	if math.IsNaN(a) {
		a = b
	}
	if math.IsNaN(d) {
		d = c
	}
	// float64(…) around every product keeps the compiler from fusing it with
	// the following addition (FMA), which would change the last bit.
	m0, m1 := (c-a)/2, (d-b)/2
	k3 := float64(2*(b-c)) + m0 + m1
	k2 := float64(3*(c-b)) - float64(2*m0) - m1
	val := func(t float64) float64 {
		v := float64(k3*t) + k2
		v = float64(v*t) + m0
		return float64(v*t) + b
	}
	// The end points come from the polynomial too, not from b and c: the
	// rounding of val(1) is part of the reference result.
	lo, hi = math.Min(val(0), val(1)), math.Max(val(0), val(1))
	derivativeRoots(3*k3, 2*k2, m0, func(t float64) {
		if t > 0 && t < 1 {
			v := val(t)
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	})
	return lo, hi, true
}

// derivativeRoots calls visit with each real root t of qa·t² + qb·t + qc —
// the stationary points of the Hermite cubic — using the numerically stable
// form of the quadratic formula.
func derivativeRoots(qa, qb, qc float64, visit func(t float64)) {
	switch {
	case qa == 0 && qb != 0:
		visit(-qc / qb)
	case qa != 0:
		disc := float64(qb*qb) - float64(4*qa*qc)
		if disc < 0 {
			return
		}
		sign := 1.0
		if qb < 0 {
			sign = -1
		}
		q := -0.5 * (qb + float64(sign*math.Sqrt(disc)))
		visit(q / qa)
		if q != 0 {
			visit(qc / q)
		}
	}
}
