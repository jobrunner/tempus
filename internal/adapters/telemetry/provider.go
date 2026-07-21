package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/jobrunner/tempus/internal/config"
)

// NewTracerProvider builds an OTel TracerProvider with an OTLP exporter and a
// ParentBased(TraceIDRatioBased) sampler. It also installs the W3C TraceContext +
// Baggage global propagator.
//
// The caller must call the returned shutdown function (even on error = nil) to
// flush spans and release resources.
//
// When cfg.Enabled is false, the caller should use a NoOp provider instead and
// NOT call this function.
func NewTracerProvider(ctx context.Context, cfg config.TracingConfig, serviceName string) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	exp, err := buildOTLPExporter(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build OTLP exporter: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(buildResource(serviceName)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	return tp, shutdown, nil
}

func buildResource(serviceName string) *sdkresource.Resource {
	r, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		// Fallback to a minimal resource rather than failing startup.
		return sdkresource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName))
	}
	return r
}

func buildOTLPExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	switch cfg.Transport {
	case "grpc":
		return otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(),
		)
	default: // "http" or anything else
		return otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
		)
	}
}
