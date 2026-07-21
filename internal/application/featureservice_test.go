package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func okProv(id string) output.FeatureProvider {
	return okProvider{id: id, feat: domain.NewPointFeature(
		domain.Coordinate{Lat: 1, Lon: 2}, map[string]any{"v": 1.0}, domain.License{Name: id, Attribution: "by " + id})}
}

type okProvider struct {
	id   string
	feat domain.Feature
}

func (p okProvider) ID() string                  { return p.id }
func (p okProvider) Kind() string                { return "weather" }
func (p okProvider) Attribution() domain.License { return p.feat.License }
func (p okProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{Feature: p.feat}, nil
}

type failProvider struct {
	id  string
	err error
}

func (p failProvider) ID() string                  { return p.id }
func (p failProvider) Kind() string                { return "weather" }
func (p failProvider) Attribution() domain.License { return domain.License{} }
func (p failProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, p.err
}

func sampleReq() domain.QueryRequest {
	loc, _ := time.LoadLocation("Europe/Berlin")
	return domain.QueryRequest{
		Coordinate: domain.Coordinate{Lat: 49.79, Lon: 9.93},
		Instant:    time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC),
		Timezone:   loc, TimezoneID: "Europe/Berlin",
	}
}

func TestFeatureService_PartialFailure(t *testing.T) {
	reg := NewRegistry()
	reg.Register(okProv("open-meteo"))
	reg.Register(failProvider{"astro", output.NewTransientError(errors.New("dial timeout"), 30*time.Second)})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error on provider failure: %v", err)
	}
	if len(res.Features) != 1 || res.Features[0].License.Attribution == "" {
		t.Fatalf("want 1 attributed feature, got %+v", res.Features)
	}
	if len(res.Providers) != 2 {
		t.Fatalf("want 2 provider statuses, got %d", len(res.Providers))
	}
	byID := map[string]domain.ProviderStatus{}
	for _, p := range res.Providers {
		byID[p.ID] = p
	}
	if byID["open-meteo"].Status != domain.StatusOK {
		t.Errorf("open-meteo status = %q", byID["open-meteo"].Status)
	}
	if s := byID["astro"]; s.Status != domain.StatusUnavailable || !s.Retryable || s.RetryAfter == "" {
		t.Errorf("astro status = %+v, want unavailable+retryable+retryAfter", s)
	}
	// localTime echoes the target instant in Berlin (CEST = +02:00).
	if res.Query.LocalTime != "2025-06-15T15:00:00+02:00" {
		t.Errorf("localTime = %q", res.Query.LocalTime)
	}
}

func TestFeatureService_AllFailStill200Shape(t *testing.T) {
	reg := NewRegistry()
	reg.Register(failProvider{"open-meteo", output.NewNotYetAvailableError(2 * time.Hour)})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error: %v", err)
	}
	if len(res.Features) != 0 {
		t.Errorf("want 0 features, got %d", len(res.Features))
	}
	if !res.Providers[0].Retryable {
		t.Error("not-yet-available must be retryable")
	}
}

func TestFeatureService_PermanentErrorStatus(t *testing.T) {
	reg := NewRegistry()
	reg.Register(failProvider{"p1", output.NewPermanentError(errors.New("bad request"))})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error: %v", err)
	}
	if len(res.Providers) != 1 {
		t.Fatalf("want 1 provider status, got %d", len(res.Providers))
	}
	st := res.Providers[0]
	if st.Status != domain.StatusError {
		t.Errorf("status = %q, want %q", st.Status, domain.StatusError)
	}
	if st.Retryable {
		t.Error("permanent error must not be retryable")
	}
}

func TestFeatureService_UnclassifiedErrorStatus(t *testing.T) {
	reg := NewRegistry()
	reg.Register(failProvider{"p2", errors.New("boom")})
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	res, err := svc.Query(context.Background(), sampleReq())
	if err != nil {
		t.Fatalf("Query must not error: %v", err)
	}
	if len(res.Providers) != 1 {
		t.Fatalf("want 1 provider status, got %d", len(res.Providers))
	}
	st := res.Providers[0]
	if st.Status != domain.StatusUnavailable {
		t.Errorf("status = %q, want %q", st.Status, domain.StatusUnavailable)
	}
	if !st.Retryable {
		t.Error("unclassified error must be retryable")
	}
}

func TestFeatureService_ProviderFilter(t *testing.T) {
	reg := NewRegistry()
	reg.Register(okProv("open-meteo"))
	reg.Register(okProv("astro"))
	svc := NewFeatureService(reg, discard(), 5*time.Second)

	r := sampleReq()
	r.Providers = []string{"astro"}
	res, _ := svc.Query(context.Background(), r)
	if len(res.Providers) != 1 || res.Providers[0].ID != "astro" {
		t.Fatalf("filter ignored: %+v", res.Providers)
	}
}
