package domain

// WMOCodeSource cites the origin of the weather-code interpretations.
const (
	WMOCodeSource    = "WMO Code Table 4677 (WW – present weather); weather-interpretation codes as used by Open-Meteo"
	WMOCodeSourceURL = "https://open-meteo.com/en/docs"
)

type wmoEntry struct{ de, en string }

// wmoCodeTable maps WMO code table 4677 codes (Open-Meteo subset) to bilingual descriptions.
var wmoCodeTable = map[int]wmoEntry{
	0:  {"Klarer Himmel", "Clear sky"},
	1:  {"Überwiegend klar", "Mainly clear"},
	2:  {"Teilweise bewölkt", "Partly cloudy"},
	3:  {"Bedeckt", "Overcast"},
	45: {"Nebel", "Fog"},
	48: {"Gefrierender Nebel (Reif)", "Depositing rime fog"},
	51: {"Leichter Nieselregen", "Light drizzle"},
	53: {"Mäßiger Nieselregen", "Moderate drizzle"},
	55: {"Dichter Nieselregen", "Dense drizzle"},
	56: {"Leichter gefrierender Nieselregen", "Light freezing drizzle"},
	57: {"Dichter gefrierender Nieselregen", "Dense freezing drizzle"},
	61: {"Leichter Regen", "Slight rain"},
	63: {"Mäßiger Regen", "Moderate rain"},
	65: {"Starker Regen", "Heavy rain"},
	66: {"Leichter gefrierender Regen", "Light freezing rain"},
	67: {"Starker gefrierender Regen", "Heavy freezing rain"},
	71: {"Leichter Schneefall", "Slight snowfall"},
	73: {"Mäßiger Schneefall", "Moderate snowfall"},
	75: {"Starker Schneefall", "Heavy snowfall"},
	77: {"Schneegriesel", "Snow grains"},
	80: {"Leichte Regenschauer", "Slight rain showers"},
	81: {"Mäßige Regenschauer", "Moderate rain showers"},
	82: {"Heftige Regenschauer", "Violent rain showers"},
	85: {"Leichte Schneeschauer", "Slight snow showers"},
	86: {"Starke Schneeschauer", "Heavy snow showers"},
	95: {"Gewitter", "Thunderstorm"},
	96: {"Gewitter mit leichtem Hagel", "Thunderstorm with slight hail"},
	99: {"Gewitter mit starkem Hagel", "Thunderstorm with heavy hail"},
}

// WeatherCodeDescription returns German and English descriptions for a WMO
// weather-interpretation code (WMO code table 4677, Open-Meteo subset).
// ok is false for unknown codes.
func WeatherCodeDescription(code int) (de, en string, ok bool) {
	entry, found := wmoCodeTable[code]
	if !found {
		return "", "", false
	}
	return entry.de, entry.en, true
}
