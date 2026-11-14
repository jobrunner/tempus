package bioclim

import (
	"math"

	"github.com/jobrunner/tempus/internal/domain"
)

// rmSite is the Rivas-Martínez site of the archive response: the resolved grid
// cell and its elevation. The archive's temperatures are downscaled to exactly
// that elevation, so it is the one the WBCS altitude reduction must start from.
// Without it the key runs unreduced (documented fallback, see
// docs/research/rivas-martinez-reference.md).
func rmSite(data dailyResponse) domain.RivasMartinezSite {
	s := domain.RivasMartinezSite{Lat: data.Latitude, Lon: data.Longitude}
	if data.Elevation != nil && !math.IsNaN(*data.Elevation) {
		s.ElevationM, s.ElevationKnown = *data.Elevation, true
	}
	return s
}

// rivasMartinezProps returns the thermotype and macrobioclimate objects of the
// altitudinal belt. elevationM is present only when the elevation was known.
func rivasMartinezProps(clim domain.MonthlyClimate, site domain.RivasMartinezSite) (thermotype, macro map[string]any) {
	rm := domain.RivasMartinez(clim, site)
	tt := rm.Thermotype
	thermotype = map[string]any{
		keyCode: tt.Code, "de": tt.De, "en": tt.En, "index": tt.Index, "value": rmm(tt.Value),
	}
	macro = map[string]any{
		keyCode: string(rm.Macrobioclimate), "de": rm.Macrobioclimate.De(), "en": string(rm.Macrobioclimate),
	}
	if site.ElevationKnown {
		macro["elevationM"] = rmm(site.ElevationM)
	}
	return thermotype, macro
}
