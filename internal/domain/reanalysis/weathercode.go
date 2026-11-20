package reanalysis

import "math"

// codeStep maps "input at or above min" to a WMO weather code.
type codeStep struct {
	min, code float32
}

// The thresholds of WeatherCode, each table checked from the highest step
// down. The float32 fields keep the comparisons the reference's float32 ones.
var (
	snowSteps   = []codeStep{{0.8, 75}, {0.2, 73}, {0.01, 71}}
	precipSteps = []codeStep{{7.6, 65}, {2.5, 63}, {1.3, 61}, {1.0, 55}, {0.5, 53}, {0.01, 51}}
	cloudSteps  = []codeStep{{80, 3}, {50, 2}, {20, 1}, {0, 0}}
)

// WeatherCode derives the WMO code from cloud cover (%), precipitation (mm)
// and snowfall (cm) as the archive does; NaN if an input is missing. Snowfall
// decides first, then precipitation, then cloud cover; a negative cloud cover
// without precipitation has no code (NaN).
func WeatherCode(cloud, precip, snow float32) float32 {
	if isNaN(cloud) || isNaN(precip) || isNaN(snow) {
		return float32(math.NaN())
	}
	for _, in := range [...]struct {
		v     float32
		steps []codeStep
	}{{snow, snowSteps}, {precip, precipSteps}, {cloud, cloudSteps}} {
		if code, ok := firstStep(in.v, in.steps); ok {
			return code
		}
	}
	return float32(math.NaN())
}

// firstStep returns the code of the first step v reaches.
func firstStep(v float32, steps []codeStep) (float32, bool) {
	for _, s := range steps {
		if v >= s.min {
			return s.code, true
		}
	}
	return 0, false
}
