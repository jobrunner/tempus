package domain

import (
	"math"
	"testing"
)

func beltClimate(tmean [12]float64) MonthlyClimate {
	return MonthlyClimate{Tmean: tmean}
}

func TestAltitudinalBelt_Ladder(t *testing.T) {
	cases := []struct {
		name  string
		tmean [12]float64
		want  string
	}{
		{"nival", [12]float64{-5, -5, -5, -5, -5, -5, -5, -5, -5, -5, -5, -5}, nameNival},
		{"subnival", [12]float64{-10, -8, -5, -2, 1, 3, 3, 2, -1, -4, -7, -9}, nameSubnival},
		{"alpine", [12]float64{-8, -7, -4, 0, 3, 5, 6, 5, 3, -1, -5, -7}, nameAlpin},
		{"subalpine", [12]float64{-6, -5, -2, 2, 6, 9, 10, 9, 6, 1, -3, -5}, nameSubalpin},
		{"high-montane", [12]float64{-10, -8, -3, 3, 9, 13, 15, 14, 8, 2, -5, -8}, nameHochmontan},
		{"montane", [12]float64{-4, -2, 2, 6, 11, 15, 17, 16, 11, 5, 0, -3}, nameMontan},
		{"colline", [12]float64{0, 2, 5, 9, 14, 18, 20, 19, 15, 9, 4, 1}, nameKollin},
		{"planar", [12]float64{8, 9, 12, 15, 19, 23, 25, 24, 20, 15, 11, 8}, namePlanar},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := AltitudinalBelt(beltClimate(tc.tmean))
			if b.De != tc.want {
				t.Errorf("belt = %q, want %q (Twarm=%.1f Twq=%.1f MAT=%.1f)",
					b.De, tc.want, b.WarmestMonthC, b.WarmestQuarterC, b.MATC)
			}
		})
	}
}

func TestAltitudinalBelt_Borderline(t *testing.T) {
	// MAT ≈ 6.67 (within 0.5 of the montane|colline boundary at 7), Twq > 10.
	b := AltitudinalBelt(beltClimate([12]float64{2, 2, 4, 6, 9, 12, 13, 12, 9, 6, 3, 2}))
	if b.De != nameMontan {
		t.Fatalf("belt = %q, want montan", b.De)
	}
	if !b.Borderline || b.AdjDe != nameKollin {
		t.Errorf("borderline=%v adj=%q, want true / submontan-kollin", b.Borderline, b.AdjDe)
	}

	// Exactly 0.5 °C from the montane|colline boundary (MAT = 6.5) → borderline
	// (the contract is "within 0.5 °C", inclusive).
	eb := AltitudinalBelt(beltClimate([12]float64{2, 2, 4, 6, 9, 12, 13, 12, 9, 6, 3, 0}))
	if math.Abs(eb.MATC-6.5) > 1e-9 || !eb.Borderline {
		t.Errorf("MAT=%.2f borderline=%v, want 6.5 / true (boundary inclusive)", eb.MATC, eb.Borderline)
	}

	// Comfortably inside colline (MAT ≈ 9.7) → not borderline.
	nb := AltitudinalBelt(beltClimate([12]float64{0, 2, 5, 9, 14, 18, 20, 19, 15, 9, 4, 1}))
	if nb.Borderline {
		t.Errorf("colline MAT=%.1f flagged borderline unexpectedly", nb.MATC)
	}
}

func TestAltitudinalBelt_Biotemperature(t *testing.T) {
	// 35 → clamped 30, −10 → clamped 0, ten months at 15 → (30 + 0 + 150)/12 = 15.
	b := AltitudinalBelt(beltClimate([12]float64{35, -10, 15, 15, 15, 15, 15, 15, 15, 15, 15, 15}))
	if math.Abs(b.BiotemperatureC-15) > 1e-9 {
		t.Errorf("biotemperature = %.4f, want 15", b.BiotemperatureC)
	}
}
