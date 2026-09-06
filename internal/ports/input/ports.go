// Package input holds the driving ports the HTTP adapter depends on.
package input

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureService is the primary business port the HTTP adapter calls.
type FeatureService interface {
	Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error)
}

// BatchPoint is one resolved batch input: either a valid QueryRequest or the
// parse error that keeps it from the query path. ID is the opaque echo id.
type BatchPoint struct {
	ID         string
	Req        *domain.QueryRequest
	ParseError string
}

// BatchItemError is the per-item error object for unprocessable points.
type BatchItemError struct {
	Message string `json:"message"`
}

// BatchItem is one batch result: the single-query envelope plus the echo id,
// or (for unparsable points) just id + error. The embedded nil pointer keeps
// query/features/providers absent on error items.
type BatchItem struct {
	ID string `json:"id"`
	*domain.QueryResult
	Error *BatchItemError `json:"error,omitempty"`
}

// BatchService processes batch points in input order; emit is called once per
// point. A non-nil emit error (e.g. a broken stream) aborts the batch.
type BatchService interface {
	QueryBatch(ctx context.Context, points []BatchPoint, emit func(BatchItem) error) error
}

// ProviderInfo describes an available provider for GET /api/v1/providers.
type ProviderInfo struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"`
	License domain.License `json:"license"`
}

// ProviderLister lists the registered providers and their attribution.
type ProviderLister interface {
	Providers(ctx context.Context) []ProviderInfo
}

// HealthChecker backs the readiness probe.
type HealthChecker interface {
	Ready(ctx context.Context) bool
}
