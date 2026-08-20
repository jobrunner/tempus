package domain

import (
	"math"
	"testing"
)

func TestBeaufort(t *testing.T) {
	// One row per force, listing speeds at both ends of the published km/h
	// class (0: <1, 1: 1–5, 2: 6–11, 3: 12–19, 4: 20–28, 5: 29–38, 6: 39–49,
	// 7: 50–61, 8: 62–74, 9: 75–88, 10: 89–102, 11: 103–117, 12: ≥118).
	tests := []struct {
		force  int
		de, en string
		speeds []float64
	}{
		{0, "Windstille", "Calm", []float64{0, 0.9}},
		{1, "leiser Zug", "Light air", []float64{1, 5}},
		{2, "leichte Brise", "Light breeze", []float64{6, 11}},
		{3, "schwache Brise", "Gentle breeze", []float64{12, 19}},
		{4, "mäßige Brise", "Moderate breeze", []float64{20, 28}},
		{5, "frische Brise", "Fresh breeze", []float64{29, 38}},
		{6, "starker Wind", "Strong breeze", []float64{39, 49}},
		{7, "steifer Wind", "Near gale", []float64{50, 61}},
		{8, "stürmischer Wind", "Gale", []float64{62, 74}},
		{9, "Sturm", "Strong gale", []float64{75, 88}},
		{10, "schwerer Sturm", "Storm", []float64{89, 102}},
		{11, "orkanartiger Sturm", "Violent storm", []float64{103, 117}},
		{12, "Orkan", "Hurricane force", []float64{118, 250}},
	}
	for _, tc := range tests {
		t.Run(tc.en, func(t *testing.T) {
			for _, kmh := range tc.speeds {
				force, de, en := Beaufort(kmh)
				if force != tc.force {
					t.Errorf("Beaufort(%v) force: got %d, want %d", kmh, force, tc.force)
				}
				if de != tc.de {
					t.Errorf("Beaufort(%v) DE: got %q, want %q", kmh, de, tc.de)
				}
				if en != tc.en {
					t.Errorf("Beaufort(%v) EN: got %q, want %q", kmh, en, tc.en)
				}
			}
		})
	}
}

func TestBeaufortNegativeSpeedIsCalm(t *testing.T) {
	_, calmDE, calmEN := Beaufort(0)
	force, de, en := Beaufort(-3)
	if force != 0 || de != calmDE || en != calmEN {
		t.Errorf("Beaufort(-3): got (%d, %q, %q), want (0, %q, %q)", force, de, en, calmDE, calmEN)
	}
}

func TestBeaufortNonFiniteIsUnclassified(t *testing.T) {
	// A non-finite speed must never be published as a force: NaN compares
	// false against every bound and would otherwise fall into the open-ended
	// top class, reporting garbage as "Orkan".
	for _, kmh := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, _, _, ok := BeaufortFor(kmh, "km/h"); ok {
			t.Errorf("BeaufortFor(%v, \"km/h\"): got ok=true, want false", kmh)
		}
	}
	for _, kmh := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if force, _, _ := Beaufort(kmh); force != 0 {
			t.Errorf("Beaufort(%v) force: got %d, want 0", kmh, force)
		}
	}
}

func TestBeaufortFor(t *testing.T) {
	tests := []struct {
		name  string
		speed float64
		unit  string
		force int
		ok    bool
	}{
		{"km/h", 30, "km/h", 5, true},
		{"kmh without slash", 30, "kmh", 5, true},
		{"m/s", 10, "m/s", 5, true},            // 36 km/h
		{"ms", 10, "ms", 5, true},              // 36 km/h
		{"miles per hour", 20, "mph", 5, true}, // 32.19 km/h
		{"knots kn", 16, "kn", 5, true},        // 29.63 km/h
		{"knots kt", 16, "kt", 5, true},
		{"knots spelled out", 16, "knots", 5, true},
		{"uppercase unit", 30, "KM/H", 5, true},
		{"empty unit is unknown", 30, "", 0, false},
		{"unknown unit", 30, "furlongs/fortnight", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			force, de, en, ok := BeaufortFor(tc.speed, tc.unit)
			if ok != tc.ok {
				t.Fatalf("BeaufortFor(%v, %q) ok: got %v, want %v", tc.speed, tc.unit, ok, tc.ok)
			}
			if !tc.ok {
				return
			}
			if force != tc.force {
				t.Errorf("BeaufortFor(%v, %q) force: got %d, want %d", tc.speed, tc.unit, force, tc.force)
			}
			if de == "" || en == "" {
				t.Errorf("BeaufortFor(%v, %q): empty label(s) de=%q en=%q", tc.speed, tc.unit, de, en)
			}
		})
	}
}
