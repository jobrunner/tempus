package bioclim

import "github.com/jobrunner/tempus/internal/domain"

// rmSite is the Rivas-Martínez site of the request: its coordinate and the
// client-supplied elevation. The reanalysis temperatures are corrected to
// exactly that elevation, so it is the one the WBCS altitude reduction must
// start from.
func rmSite(req domain.QueryRequest) domain.RivasMartinezSite {
	return domain.RivasMartinezSite{
		Lat: req.Coordinate.Lat, Lon: req.Coordinate.Lon,
		ElevationM: req.Elevation, ElevationKnown: true,
	}
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
