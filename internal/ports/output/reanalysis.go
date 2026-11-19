package output

import (
	"context"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

// Reanalysis is a driven port: point series from the reanalysis archive
// (ECMWF IFS, ERA5, ERA5-Land), selected and corrected for the point's
// elevation. Failures are ProviderErrors (transient for upstream trouble).
type Reanalysis interface {
	// Hourly returns the given variables (domain.Var*) for the hours
	// [from, to). Derived variables pull in what they need.
	Hourly(ctx context.Context, p domain.GeoPoint, from, to time.Time, variables ...string) (domain.HourlySeries, error)
	// Daily returns the UTC-day aggregates for firstDay … lastDay inclusive.
	Daily(ctx context.Context, p domain.GeoPoint, firstDay, lastDay time.Time) (domain.DailySeries, error)
}
