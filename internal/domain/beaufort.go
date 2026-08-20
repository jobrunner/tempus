package domain

import (
	"math"
	"strings"
)

// BeaufortSource cites the origin of the Beaufort wind-force classification.
const (
	BeaufortSource    = "Beaufort-Skala (WMO), abgeleitet aus der Windgeschwindigkeit in 10 m, tempus"
	BeaufortSourceURL = "https://en.wikipedia.org/wiki/Beaufort_scale"
)

type beaufortClass struct {
	// maxKmh is the exclusive upper bound of the class in km/h.
	maxKmh float64
	de, en string
}

// beaufortScale lists forces 0..12 with the bounds of the published km/h table
// (0: <1, 1: 1–5, 2: 6–11, 3: 12–19, 4: 20–28, 5: 29–38, 6: 39–49, 7: 50–61,
// 8: 62–74, 9: 75–88, 10: 89–102, 11: 103–117, 12: ≥118).
var beaufortScale = []beaufortClass{
	{1, "Windstille", "Calm"},
	{6, "leiser Zug", "Light air"},
	{12, "leichte Brise", "Light breeze"},
	{20, "schwache Brise", "Gentle breeze"},
	{29, "mäßige Brise", "Moderate breeze"},
	{39, "frische Brise", "Fresh breeze"},
	{50, "starker Wind", "Strong breeze"},
	{62, "steifer Wind", "Near gale"},
	{75, "stürmischer Wind", "Gale"},
	{89, "Sturm", "Strong gale"},
	{103, "schwerer Sturm", "Storm"},
	{118, "orkanartiger Sturm", "Violent storm"},
	{0, "Orkan", "Hurricane force"}, // force 12: open-ended, maxKmh unused
}

// Beaufort classifies a wind speed in km/h into a Beaufort force (0..12) and
// returns German and English labels. Speeds below 1 km/h — including
// physically impossible negative values — are force 0 (calm). A non-finite
// speed is force 0 as well: NaN compares false against every bound and would
// otherwise fall through to the open-ended top class, reporting garbage as a
// hurricane. Callers that must not publish such a value use BeaufortFor,
// which rejects it outright.
func Beaufort(kmh float64) (force int, de, en string) {
	if math.IsNaN(kmh) || math.IsInf(kmh, 0) {
		c := beaufortScale[0]
		return 0, c.de, c.en
	}
	for i, c := range beaufortScale[:len(beaufortScale)-1] {
		if kmh < c.maxKmh {
			return i, c.de, c.en
		}
	}
	last := beaufortScale[len(beaufortScale)-1]
	return len(beaufortScale) - 1, last.de, last.en
}

// beaufortUnitFactors converts a supported wind-speed unit to km/h. Keys are
// lower-cased with "/" removed, so both "m/s" and "ms" match.
var beaufortUnitFactors = map[string]float64{
	"kmh":   1,
	"ms":    3.6,
	"mph":   1.609344,
	"kn":    1.852,
	"kt":    1.852,
	"kts":   1.852,
	"knots": 1.852,
}

// BeaufortFor classifies a wind speed given in unit into a Beaufort force,
// converting to km/h first. ok is false when the speed is not finite or the
// unit is missing or not recognised, so callers never publish a force derived
// from an unknown scale. An empty unit is treated as unknown rather than
// assumed to be km/h: "the provider said km/h" and "the provider said
// nothing" must not be the same case.
func BeaufortFor(speed float64, unit string) (force int, de, en string, ok bool) {
	if math.IsNaN(speed) || math.IsInf(speed, 0) {
		return 0, "", "", false
	}
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(unit)), "/", "")
	factor, known := beaufortUnitFactors[key]
	if !known {
		return 0, "", "", false
	}
	f, d, e := Beaufort(speed * factor)
	return f, d, e, true
}
