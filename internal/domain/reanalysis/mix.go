package reanalysis

// LapseRate is the temperature change per metre of height (K/m).
const LapseRate float32 = 0.0065

// Correct shifts temperatures from the cell's model elevation to the target
// elevation in place: T += (model − target) · 0.0065.
func Correct(series []float32, modelElevation, targetElevation float32) {
	if modelElevation == targetElevation {
		return
	}
	c := float32(float32(modelElevation-targetElevation) * LapseRate)
	for i, v := range series {
		series[i] = v + c
	}
}

// smoothWidth is the number of hours before a gap of the higher-priority
// series that are blended with the lower-priority one.
const smoothWidth = 3

// Mix combines series ordered from highest to lowest priority: the first
// series wins, lower ones only fill its NaN hours. Mixing stops as soon as no
// NaN is left. With smooth set (all but categorical variables), the hours just
// before a filled gap fade linearly from the higher- to the lower-priority
// value so the series has no jump at the gap. The inputs are not modified.
func Mix(byPriority [][]float32, smooth bool) []float32 {
	var out []float32
	for _, s := range byPriority {
		if s == nil {
			continue
		}
		if out == nil {
			out = append([]float32(nil), s...)
		} else {
			fill(out, s, smooth)
		}
		if !hasNaN(out) {
			break
		}
	}
	return out
}

// fill writes lower into the NaN hours of out, walking backwards in time so
// the distance to the next filled gap is known.
func fill(out, lower []float32, smooth bool) {
	since := smoothWidth
	for x := len(lower) - 1; x >= 0; x-- {
		since++
		if isNaN(lower[x]) {
			continue
		}
		if isNaN(out[x]) {
			out[x] = lower[x]
			since = 0
			continue
		}
		if !smooth || since > smoothWidth {
			continue
		}
		a := float32(lower[x] * float32(smoothWidth+1-since))
		b := float32(out[x] * float32(since))
		out[x] = (a + b) / float32(smoothWidth+1)
	}
}

func hasNaN(s []float32) bool {
	for _, v := range s {
		if isNaN(v) {
			return true
		}
	}
	return false
}

func isNaN(v float32) bool { return v != v }
