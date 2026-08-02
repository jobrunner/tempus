package domain

import "testing"

// climate builds a MonthlyClimate from monthly means and a per-month precip
// value (constant unless overridden). Tmin/Tmax are unused by Köppen.
func climate(tmean [12]float64, precip [12]float64) MonthlyClimate {
	var tmin, tmax [12]float64
	for i := range tmean {
		tmin[i] = tmean[i] - 5
		tmax[i] = tmean[i] + 5
	}
	return MonthlyClimate{Tmin: tmin, Tmax: tmax, Tmean: tmean, Precip: precip}
}

func constPrecip(v float64) [12]float64 {
	var p [12]float64
	for i := range p {
		p[i] = v
	}
	return p
}

func TestKoppenGeiger(t *testing.T) {
	cases := []struct {
		name string
		c    MonthlyClimate
		lat  float64
		want string
	}{
		{
			name: "Berlin-like temperate oceanic",
			c:    climate([12]float64{1, 2, 5, 9, 14, 17, 19, 18, 14, 9, 5, 2}, constPrecip(50)),
			lat:  52.5, want: "Cfb",
		},
		{
			name: "hot desert",
			c:    climate([12]float64{20, 22, 25, 28, 31, 34, 35, 34, 31, 28, 24, 21}, constPrecip(5)),
			lat:  24, want: "BWh",
		},
		{
			name: "tropical rainforest",
			c:    climate([12]float64{26, 26, 27, 27, 26, 26, 25, 26, 26, 26, 26, 26}, constPrecip(120)),
			lat:  2, want: "Af",
		},
		{
			name: "tundra",
			c:    climate([12]float64{-20, -18, -12, -4, 2, 6, 9, 8, 3, -5, -12, -18}, constPrecip(25)),
			lat:  71, want: "ET",
		},
		{
			name: "warm-summer humid continental",
			c:    climate([12]float64{-8, -6, -1, 6, 13, 18, 20, 19, 13, 6, -1, -6}, constPrecip(50)),
			lat:  50, want: "Dfb",
		},
		{
			name: "cold steppe",
			c:    climate([12]float64{3, 4, 7, 11, 15, 18, 20, 19, 15, 10, 6, 3}, constPrecip(21)),
			lat:  45, want: "BSk",
		},
		{
			name: "Mediterranean warm dry summer",
			c: climate([12]float64{7, 8, 10, 13, 16, 19, 21, 20, 17, 13, 9, 7},
				[12]float64{80, 70, 60, 30, 15, 5, 3, 8, 25, 60, 80, 90}),
			lat: 40, want: "Csb",
		},
		{
			name: "monsoon continental dry winter hot summer",
			c: climate([12]float64{-6, -3, 4, 12, 18, 23, 25, 24, 18, 11, 3, -4},
				[12]float64{5, 5, 10, 40, 90, 140, 160, 150, 80, 25, 8, 4}),
			lat: 40, want: "Dwa",
		},
		{
			name: "very cold subarctic (d)",
			c:    climate([12]float64{-45, -42, -30, -10, 5, 14, 17, 15, 7, -8, -28, -42}, constPrecip(30)),
			lat:  67, want: "Dfd",
		},
		{
			name: "tropical savanna dry winter",
			c: climate([12]float64{27, 27, 28, 28, 27, 26, 26, 26, 27, 27, 27, 27},
				[12]float64{10, 15, 40, 90, 160, 200, 180, 160, 120, 50, 15, 8}),
			lat: 12, want: "Aw",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, de, en := KoppenGeiger(tc.c, tc.lat)
			if code != tc.want {
				t.Errorf("code = %q, want %q", code, tc.want)
			}
			if de == "" || en == "" {
				t.Errorf("missing description for %q: de=%q en=%q", code, de, en)
			}
		})
	}
}

// Every description must be bilingual, and the table must cover the full set of
// Köppen-Geiger codes the classifier can emit.
func TestKoppenDescriptionsComplete(t *testing.T) {
	if len(koppenDescriptions) != 31 {
		t.Errorf("koppenDescriptions has %d entries, want 31", len(koppenDescriptions))
	}
	for code, d := range koppenDescriptions {
		if d.de == "" || d.en == "" {
			t.Errorf("code %q missing a description (de=%q en=%q)", code, d.de, d.en)
		}
	}
}
