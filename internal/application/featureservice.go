package application

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// FeatureService orchestrates providers and assembles the response envelope.
type FeatureService struct {
	registry *Registry
	logger   *slog.Logger
	timeout  time.Duration
}

// NewFeatureService builds the service.
func NewFeatureService(reg *Registry, logger *slog.Logger, timeout time.Duration) *FeatureService {
	return &FeatureService{registry: reg, logger: logger, timeout: timeout}
}

// Query fetches from the selected providers concurrently and assembles the
// result. Provider failures are encoded in Providers[]; the call itself only
// errors on a caller-canceled context.
func (s *FeatureService) Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error) {
	providers := s.selected(req)

	type outcome struct {
		feature *domain.Feature
		status  domain.ProviderStatus
	}
	outcomes := make([]outcome, len(providers))

	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p output.FeatureProvider) {
			defer wg.Done()
			fctx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()
			outcomes[i] = s.fetchOne(fctx, p, req)
		}(i, p)
	}
	wg.Wait()

	res := domain.QueryResult{
		Query:     s.echo(req),
		Features:  []domain.Feature{},
		Providers: make([]domain.ProviderStatus, 0, len(outcomes)),
	}
	for _, o := range outcomes {
		if o.feature != nil {
			res.Features = append(res.Features, *o.feature)
		}
		res.Providers = append(res.Providers, o.status)
	}
	return res, nil
}

func (s *FeatureService) selected(req domain.QueryRequest) []output.FeatureProvider {
	if len(req.Providers) == 0 {
		return s.registry.All()
	}
	var out []output.FeatureProvider
	for _, id := range req.Providers {
		if p, ok := s.registry.Get(id); ok {
			out = append(out, p)
		}
	}
	return out
}

func (s *FeatureService) fetchOne(ctx context.Context, p output.FeatureProvider, req domain.QueryRequest) (o struct {
	feature *domain.Feature
	status  domain.ProviderStatus
}) {
	res, err := p.Fetch(ctx, req)
	if err == nil {
		f := res.Feature
		o.feature = &f
		o.status = domain.ProviderStatus{ID: p.ID(), Kind: p.Kind(), Status: domain.StatusOK, Cached: res.Cached}
		return o
	}
	o.status = s.statusFor(p, err)
	return o
}

func (s *FeatureService) statusFor(p output.FeatureProvider, err error) domain.ProviderStatus {
	st := domain.ProviderStatus{ID: p.ID(), Kind: p.Kind(), Error: err.Error()}
	pe, ok := output.AsProviderError(err)
	if !ok {
		// Unknown error: be conservative and let the client retry.
		s.logger.Warn("unclassified provider error", "provider", p.ID(), "error", err)
		st.Status = domain.StatusUnavailable
		st.Retryable = true
		return st
	}
	switch pe.Class {
	case output.ClassPermanent:
		st.Status = domain.StatusError
		st.Retryable = false
	default: // transient, not-yet-available
		st.Status = domain.StatusUnavailable
		st.Retryable = true
	}
	if pe.RetryAfter > 0 {
		st.RetryAfter = pe.RetryAfter.String()
	}
	return st
}

func (s *FeatureService) echo(req domain.QueryRequest) domain.QueryEcho {
	return domain.QueryEcho{
		Coordinate: req.Coordinate,
		Datetime:   req.Instant.UTC().Format(time.RFC3339),
		Timezone:   req.TimezoneID,
		LocalTime:  req.Instant.In(req.Timezone).Format(time.RFC3339),
	}
}
