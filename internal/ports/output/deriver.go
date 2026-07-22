package output

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureDeriver computes derived features from already-fetched source features.
type FeatureDeriver interface {
	ID() string
	Kind() string
	Attribution() domain.License
	Derive(ctx context.Context, req domain.QueryRequest, sources []domain.Feature) ([]domain.Feature, error)
}
