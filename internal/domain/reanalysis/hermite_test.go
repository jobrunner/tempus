package reanalysis

import (
	"math"
	"testing"
)

// denseMinMax is an independent reference for DailyMinMax: it samples the
// Catmull-Rom segment of every interval of the day densely instead of solving
// for the stationary points, with the same NaN rules (a gap in b or c drops
// the interval, a missing outer neighbour repeats the interval's end value).
func denseMinMax(series []float32, start int) (lo, hi float64) {
	at := func(i int) float64 {
		if i < 0 || i >= len(series) {
			return math.NaN()
		}
		return float64(series[i])
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for h := start; h < start+HoursPerDay; h++ {
		a, b, c, d := at(h-1), at(h), at(h+1), at(h+2)
		if math.IsNaN(b) || math.IsNaN(c) {
			continue
		}
		if math.IsNaN(a) {
			a = b
		}
		if math.IsNaN(d) {
			d = c
		}
		m0, m1 := (c-a)/2, (d-b)/2
		const steps = 5000
		for k := 0; k <= steps; k++ {
			t := float64(k) / steps
			v := b + m0*t + (3*(c-b)-2*m0-m1)*t*t + (2*(b-c)+m0+m1)*t*t*t
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	return lo, hi
}

func TestDailyMinMax_MatchesDenseHermite(t *testing.T) {
	// A diurnal temperature curve with an asymmetric peak, a sharp front in the
	// evening and different values just outside the day (index 0 and 25-26),
	// so the neighbours and the window bounds both matter.
	diurnal := []float32{
		-3.5, // start-1
		2.1, 1.6, 1.2, 0.9, 0.7, 1.4, 3.2, 5.8, 8.9, 11.6, 13.4, 14.9,
		15.3, 15.1, 14.2, 12.8, 11.3, 6.2, 5.9, 5.1, 4.4, 3.6, 3.1, 2.8,
		2.6,        // start+24, the end point of the last interval
		-7.0, 21.0, // outside the day
	}
	gappy := append([]float32(nil), diurnal...)
	gappy[0] = nan         // missing left neighbour of the first interval
	gappy[10] = nan        // drops the intervals that touch hour 10
	gappy[26] = nan        // missing right neighbour of the last interval
	short := diurnal[1:26] // series ends at start+24: the last d is outside
	tests := []struct {
		name   string
		series []float32
		start  int
	}{
		{"diurnal", diurnal, 1},
		{"gappy", gappy, 1},
		{"short", short, 0},
	}
	for _, tt := range tests {
		lo, hi := DailyMinMax(tt.series, tt.start)
		wlo, whi := denseMinMax(tt.series, tt.start)
		if math.Abs(float64(lo)-wlo) > 1e-4 || math.Abs(float64(hi)-whi) > 1e-4 {
			t.Errorf("%s: got %v/%v, dense reference %v/%v", tt.name, lo, hi, wlo, whi)
		}
	}
}

func TestDailyMinMax_QuadraticIsExact(t *testing.T) {
	// Catmull-Rom reproduces a parabola exactly, so the cubic term vanishes and
	// the stationary point comes from the linear branch: 10 − (h − 12.25)²
	// peaks at 10 between hours 12 and 13. All values are exact in binary.
	series := make([]float32, 27)
	for i := range series {
		d := float32(i) - 12.25
		series[i] = 10 - d*d
	}
	lo, hi := DailyMinMax(series, 1)
	if hi != 10 {
		t.Fatalf("max: got %v want 10", hi)
	}
	if want := float32(10 - 12.75*12.75); lo != want { // hour 25, the day's last end point
		t.Fatalf("min: got %v want %v", lo, want)
	}
}

func TestDerivativeRoots(t *testing.T) {
	var got []float64
	collect := func(t float64) { got = append(got, t) }
	// (t − 0.25)(t − 0.75) = t² − t + 0.1875
	derivativeRoots(1, -1, 0.1875, collect)
	if len(got) != 2 || math.Abs(got[0]-0.75) > 1e-12 || math.Abs(got[1]-0.25) > 1e-12 {
		t.Fatalf("two roots: %v", got)
	}
	got = nil
	// −(t − 0.5)(t + 2) = −t² − 1.5t + 1, qb < 0 flips the sign branch.
	derivativeRoots(-1, -1.5, 1, collect)
	if len(got) != 2 || math.Abs(got[0]-(-2)) > 1e-12 || math.Abs(got[1]-0.5) > 1e-12 {
		t.Fatalf("negative qb: %v", got)
	}
	got = nil
	derivativeRoots(1, 0, 1, collect) // no real root
	derivativeRoots(0, 0, 1, collect) // constant derivative
	if len(got) != 0 {
		t.Fatalf("no roots expected: %v", got)
	}
	derivativeRoots(0, 2, -1, collect) // linear: t = 0.5
	if len(got) != 1 || got[0] != 0.5 {
		t.Fatalf("linear: %v", got)
	}
	got = nil
	derivativeRoots(1, 0, 0, collect) // double root at 0: q = 0 skips qc/q
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("double root: %v", got)
	}
}
