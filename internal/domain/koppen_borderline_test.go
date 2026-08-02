package domain

import (
	"math"
	"testing"
)

func TestColdestMonthMeanC(t *testing.T) {
	c := climate([12]float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}, constPrecip(50))
	if got := ColdestMonthMeanC(c); math.Abs(got-1) > 1e-9 {
		t.Errorf("ColdestMonthMeanC = %.2f, want 1", got)
	}
}

func TestKoppenBorderline(t *testing.T) {
	// Coldest month ≈ +1.0 °C (within 1.5 °C of the 0 °C C/D boundary): a colder
	// dataset would flip Cfb → Dfb, so this must be flagged borderline.
	near := climate([12]float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}, constPrecip(50))
	if code, _, _ := KoppenGeiger(near, 49.7); code[:1] != "C" {
		t.Fatalf("precondition: base main class = %q, want C*", code)
	}
	bl, adj, de, en := KoppenBorderline(near, 49.7)
	if !bl {
		t.Fatalf("expected borderline near the C/D boundary")
	}
	if adj[:1] != "D" || de == "" || en == "" {
		t.Errorf("adjacent = %q (%q/%q), want a D* class with descriptions", adj, de, en)
	}

	// Warm oceanic well inside C (coldest month ≈ +7 °C): not borderline.
	far := climate([12]float64{7, 8, 10, 12, 15, 18, 20, 19, 16, 12, 9, 7}, constPrecip(60))
	if bl, _, _, _ := KoppenBorderline(far, 45); bl {
		t.Errorf("warm oceanic flagged borderline unexpectedly")
	}
}
