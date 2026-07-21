package telemetry

import (
	"context"
	"testing"

	"github.com/jobrunner/tempus/internal/config"
)

func TestNewTracerProvider_HTTPTransport(t *testing.T) {
	// Use an unreachable endpoint — OTel HTTP exporter is lazy and does not
	// dial at construction time, so NewTracerProvider succeeds here.
	cfg := config.TracingConfig{
		Enabled:     true,
		Endpoint:    "localhost:19999",
		Transport:   "http",
		SampleRatio: 1.0,
	}
	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-svc")
	if err != nil {
		t.Fatalf("NewTracerProvider(http): %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	// Shutdown with a context that won't wait for a real backend.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = shutdown(ctx)
}

func TestNewTracerProvider_GRPCTransport(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:     true,
		Endpoint:    "localhost:19998",
		Transport:   "grpc",
		SampleRatio: 0.5,
	}
	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-svc")
	if err != nil {
		t.Fatalf("NewTracerProvider(grpc): %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = shutdown(ctx)
}

func TestNewTracerProvider_EmptyEndpointDefaultsToLocalhost(t *testing.T) {
	// When Endpoint is empty, buildOTLPExporter falls back to localhost:4318.
	// This must not error (just won't export if no collector is running).
	cfg := config.TracingConfig{
		Enabled:     true,
		Endpoint:    "",
		Transport:   "http",
		SampleRatio: 1.0,
	}
	tp, shutdown, err := NewTracerProvider(context.Background(), cfg, "test-svc")
	if err != nil {
		t.Fatalf("NewTracerProvider(empty endpoint): %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = shutdown(ctx)
	_ = tp
}
