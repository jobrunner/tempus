package reanalysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	ra "github.com/jobrunner/tempus/internal/domain/reanalysis"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// upstreamRetry is the retry hint when the bucket fails.
const upstreamRetry = 30 * time.Second

// Service implements output.Reanalysis on top of a SeriesReader.
type Service struct {
	assembler *Assembler
}

var _ output.Reanalysis = (*Service)(nil)

// NewService wires the service onto a series reader (the open-data store).
func NewService(r output.SeriesReader) *Service {
	return &Service{assembler: NewAssembler(r)}
}

func location(p domain.GeoPoint) Location {
	return Location{Lat: float32(p.Lat), Lon: float32(p.Lon), Elevation: float32(p.Elevation)}
}

// Hourly satisfies output.Reanalysis.
func (s *Service) Hourly(ctx context.Context, p domain.GeoPoint, from, to time.Time, variables ...string) (domain.HourlySeries, error) {
	from, to = from.UTC().Truncate(time.Hour), to.UTC().Truncate(time.Hour)
	if !to.After(from) {
		return domain.HourlySeries{}, output.NewPermanentError(fmt.Errorf("reanalysis: empty range %s … %s", from, to))
	}
	h, err := s.assembler.Hourly(ctx, location(p), from, to, variables...)
	if err != nil {
		return domain.HourlySeries{}, classify(err)
	}
	out := domain.HourlySeries{From: from, Values: map[string][]float64{}, Cells: cells(h.Cells)}
	for _, v := range variables {
		out.Values[v] = widen(h.Values[v])
	}
	return out, nil
}

// Daily satisfies output.Reanalysis. The hourly window reaches one hour
// before the first day and two hours past the last, which the Hermite
// extrema of the first and last interval need.
func (s *Service) Daily(ctx context.Context, p domain.GeoPoint, firstDay, lastDay time.Time) (domain.DailySeries, error) {
	first := firstDay.UTC().Truncate(24 * time.Hour)
	last := lastDay.UTC().Truncate(24 * time.Hour)
	if last.Before(first) {
		return domain.DailySeries{}, output.NewPermanentError(fmt.Errorf("reanalysis: empty day range %s … %s", first, last))
	}
	from := first.Add(-time.Hour)
	to := last.Add(24*time.Hour + 2*time.Hour)
	h, err := s.assembler.Hourly(ctx, location(p), from, to, VarTemperature, VarPrecip)
	if err != nil {
		return domain.DailySeries{}, classify(err)
	}
	days := int(last.Sub(first)/(24*time.Hour)) + 1
	out := domain.DailySeries{
		FirstDay: first, Cells: cells(h.Cells),
		TMax: make([]float64, days), TMin: make([]float64, days),
		TMean: make([]float64, days), PrecipSum: make([]float64, days),
	}
	t, pr := h.Values[VarTemperature], h.Values[VarPrecip]
	for d := range days {
		start := 1 + d*ra.HoursPerDay // index of 00:00 of day d
		lo, hi := ra.DailyMinMax(t, start)
		out.TMin[d], out.TMax[d] = float64(lo), float64(hi)
		out.TMean[d] = float64(ra.DailyMean(t, start))
		out.PrecipSum[d] = float64(ra.DailySum(pr, start))
	}
	return out, nil
}

// classify turns bucket failures into retryable provider errors; anything
// else (corrupt data, unsupported format) is permanent and loud.
func classify(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return output.NewTransientError(err, upstreamRetry)
	}
	var pe output.ProviderError
	if errors.As(err, &pe) {
		return err
	}
	return output.NewPermanentError(err)
}

func widen(s []float32) []float64 {
	out := make([]float64, len(s))
	for i, v := range s {
		out[i] = float64(v)
	}
	return out
}

func cells(cs []SelectedCell) []domain.ModelCell {
	out := make([]domain.ModelCell, len(cs))
	for i, c := range cs {
		out[i] = domain.ModelCell{Model: c.Model, Lat: float64(c.Lat), Lon: float64(c.Lon), Elevation: float64(c.ModelElevation)}
	}
	return out
}
