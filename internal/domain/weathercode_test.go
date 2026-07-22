package domain

import "testing"

func TestWeatherCodeDescription_KnownCodes(t *testing.T) {
	tests := []struct {
		code int
		de   string
		en   string
	}{
		{0, "Klarer Himmel", "Clear sky"},
		{53, "Mäßiger Nieselregen", "Moderate drizzle"},
		{95, "Gewitter", "Thunderstorm"},
	}
	for _, tc := range tests {
		de, en, ok := WeatherCodeDescription(tc.code)
		if !ok {
			t.Errorf("code %d: expected ok=true", tc.code)
			continue
		}
		if de != tc.de {
			t.Errorf("code %d DE: got %q, want %q", tc.code, de, tc.de)
		}
		if en != tc.en {
			t.Errorf("code %d EN: got %q, want %q", tc.code, en, tc.en)
		}
	}
}

func TestWeatherCodeDescription_UnknownCode(t *testing.T) {
	_, _, ok := WeatherCodeDescription(42)
	if ok {
		t.Error("code 42: expected ok=false for unknown code")
	}
}
