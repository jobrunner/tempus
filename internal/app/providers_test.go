package app

import (
	"context"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/clock"
	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

func TestBuildReanalysis_DisabledIsNil(t *testing.T) {
	cfg := testConfig(t)
	if archive, keepListed := buildReanalysis(cfg, nil, clock.System{}, nil); archive == nil || keepListed == nil {
		t.Fatal("enabled archive must be built")
	}
	cfg.Providers.Reanalysis.Enabled = false
	if archive, keepListed := buildReanalysis(cfg, nil, clock.System{}, nil); archive != nil || keepListed != nil {
		t.Fatal("disabled archive must be nil, so its providers are not registered")
	}
}

// Fitness function: every provider the composition root registers must declare a
// COMPLETE static licence. That block is published verbatim on
// /api/v1/providers, and unlike Feature.License it is never validated at
// request time — so without this test an incomplete one would be served to
// clients and nothing would notice.
//
// Kept as a test rather than a startup check on purpose: a misconfigured
// provider is a programming error, and failing CI is more useful than failing
// to boot in production.
func TestEveryRegisteredProviderDeclaresACompleteLicense(t *testing.T) {
	cfg := testConfig(t)
	reg := buildRegistry(cfg, nil, clock.System{}, archiveOnly(buildReanalysis(cfg, nil, clock.System{}, nil)))

	all := reg.All()
	if len(all) == 0 {
		t.Fatal("no providers registered — this assertion would be vacuous")
	}
	for _, p := range all {
		if err := p.Attribution().Validate(); err != nil {
			t.Errorf("provider %q (%s) declares an incomplete licence: %v", p.ID(), p.Kind(), err)
		}
	}
}

func TestCachingOptions_ResultsTurnFinalAfterTheSeriesCache(t *testing.T) {
	o := cachingOptions(72*time.Hour, nil)
	if o.ArchiveDelay != 72*time.Hour+10*time.Minute {
		t.Errorf("ArchiveDelay = %v, want maturity + 10m", o.ArchiveDelay)
	}
	if o.Version != "3" || o.MatureTTL != 365*24*time.Hour || o.ImmatureTTL != time.Hour {
		t.Errorf("options = %+v", o)
	}
}

func TestGDDKeyParam(t *testing.T) {
	if got := gddKeyParam(domain.QueryRequest{}); got != "" {
		t.Errorf("no override: %q, want empty", got)
	}
	base := 7.5
	if got := gddKeyParam(domain.QueryRequest{GDDBaseCelsius: &base}); got != "gdd=7.50" {
		t.Errorf("override: %q, want gdd=7.50", got)
	}
}

// archiveOnly drops the background task of buildReanalysis.
func archiveOnly(archive output.Reanalysis, _ func(context.Context)) output.Reanalysis {
	return archive
}
