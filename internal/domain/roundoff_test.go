package domain

import (
	"math"
	"testing"
)

// TestRoundoffHelpers pins the comparison helpers: exact values keep their
// meaning, roundoff-sized differences count as equal, real differences count.
func TestRoundoffHelpers(t *testing.T) {
	const b = 1400.0
	cases := []struct {
		name                          string
		x, bound                      float64
		above, atLeast, below, atMost bool
	}{
		{"exactly on the bound", b, b, false, true, false, true},
		{"roundoff above", 1400.0000000000002, b, false, true, false, true},
		{"roundoff below", 1399.9999999999998, b, false, true, false, true},
		{"0.001 above", 1400.001, b, true, true, false, false},
		{"0.001 below", 1399.999, b, false, false, true, true},
		{"small bound: absolute slack 1e-9", 2 + 5e-10, 2, false, true, false, true},
		{"small bound: 1e-8 is real", 2 + 1e-8, 2, true, true, false, false},
		{"zero bound", 0, 0, false, true, false, true},
		{"+Inf is above any finite bound", math.Inf(1), 2, true, true, false, false},
	}
	for _, tc := range cases {
		if got := above(tc.x, tc.bound); got != tc.above {
			t.Errorf("%s: above = %v", tc.name, got)
		}
		if got := atLeast(tc.x, tc.bound); got != tc.atLeast {
			t.Errorf("%s: atLeast = %v", tc.name, got)
		}
		if got := below(tc.x, tc.bound); got != tc.below {
			t.Errorf("%s: below = %v", tc.name, got)
		}
		if got := atMost(tc.x, tc.bound); got != tc.atMost {
			t.Errorf("%s: atMost = %v", tc.name, got)
		}
	}
}
