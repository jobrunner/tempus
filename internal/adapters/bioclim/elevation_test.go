package bioclim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// saentisBody is the Säntis WMO 1991–2020 normal (WMO 6680, 2501 m) served as
// an archive response, with the given top-level elevation (nil = absent).
func saentisBody(elev any) []byte {
	return cannedBody(47.249, 9.343, elev,
		[12]float64{-6.8, -7.5, -5.6, -3, 1, 4.5, 6.4, 6.8, 3.6, 1.1, -3.3, -5.9},
		[12]float64{-9.3, -10, -8, -5.3, -1.3, 2.1, 4, 4.5, 1.3, -1.3, -5.7, -8.4},
		[12]float64{-4.1, -4.8, -3.3, -1, 3.2, 6.9, 9.2, 9.6, 6.2, 3.7, -0.7, -3.2},
		[12]float64{269.5, 221.9, 254.6, 176.2, 224, 242.8, 278.6, 266.3, 214.3, 193.4, 204.9, 293.6})
}

func fetchAltitudinalBelt(t *testing.T, body []byte) map[string]any {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	p := New(Options{ArchiveBaseURL: srv.URL, Timeout: 2 * time.Second, Cache: newMemCache()})
	res, err := p.Fetch(context.Background(), req("1991-1992"))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	ab, ok := res.Feature.Properties["altitudinalBelt"].(map[string]any)
	if !ok {
		t.Fatalf("altitudinalBelt missing")
	}
	return ab
}

// TestFetch_ElevationFeedsTheMacrobioclimateKey: the archive response's
// elevation drives the WBCS08 altitude reduction to 200 m, so a summit with a
// polar-looking Tp (234) stays temperate (cryorotemperate).
func TestFetch_ElevationFeedsTheMacrobioclimateKey(t *testing.T) {
	ab := fetchAltitudinalBelt(t, saentisBody(2501.0))
	mb, _ := ab["macrobioclimate"].(map[string]any)
	tt, _ := ab["thermotype"].(map[string]any)
	if mb[keyCode] != "temperate" || mb["elevationM"] != 2501.0 {
		t.Errorf("macrobioclimate = %v, want temperate at 2501 m", mb)
	}
	if tt[keyCode] != "cryorotemperate" {
		t.Errorf("thermotype = %v, want cryorotemperate", tt)
	}
}

// TestFetch_MissingElevationFallsBackToNoReduction documents the fallback:
// without an elevation the key is evaluated as if the site lay at or below
// 200 m, and the response says so by omitting elevationM.
func TestFetch_MissingElevationFallsBackToNoReduction(t *testing.T) {
	ab := fetchAltitudinalBelt(t, saentisBody(nil))
	mb, _ := ab["macrobioclimate"].(map[string]any)
	if mb[keyCode] != "polar" {
		t.Errorf("macrobioclimate = %v, want polar (no altitude reduction)", mb)
	}
	if _, ok := mb["elevationM"]; ok {
		t.Errorf("elevationM = %v, want absent", mb["elevationM"])
	}
}
