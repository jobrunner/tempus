package reanalysis

import ra "github.com/jobrunner/tempus/internal/domain/reanalysis"

// mixAll mixes every variable's model series; a variable no model delivered
// is n NaN hours.
func mixAll(vars []string, perVar [][][]float32, n int) map[string][]float32 {
	values := map[string][]float32{}
	for vi, v := range vars {
		mixed := ra.Mix(perVar[vi], true)
		if mixed == nil {
			mixed = nanSeries(n)
		}
		values[v] = mixed
	}
	return values
}

// derive applies the snow rule to the mixed snowfall and adds the derived
// variables (relative humidity, weather code) of n hours to values.
func derive(values map[string][]float32, need map[string]bool, ifsModelElevation, target float32, n int) {
	if need[VarSnowfall] {
		ra.SnowRule(values[VarSnowfall], values[VarTemperature], ifsModelElevation, target)
	}
	if need[VarHumidity] {
		t, td := values[VarTemperature], values[VarDewPoint]
		rh := make([]float32, n)
		for i := range rh {
			rh[i] = ra.RelativeHumidity(t[i], td[i])
		}
		values[VarHumidity] = rh
	}
	if need[VarWeatherCode] {
		wc := make([]float32, n)
		for i := range wc {
			wc[i] = ra.WeatherCode(values[VarCloud][i], values[VarPrecip][i], values[VarSnowfall][i])
		}
		values[VarWeatherCode] = wc
	}
}
