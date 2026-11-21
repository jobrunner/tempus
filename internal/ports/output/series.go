package output

import (
	"context"
	"time"
)

// SeriesReader is a driven port onto a gridded model archive: it returns the
// hourly values of one variable at one grid location. Hours the archive does
// not cover are NaN.
type SeriesReader interface {
	ReadSeries(ctx context.Context, domain, variable string, coord []uint64, from, to time.Time) ([]float32, error)
}
