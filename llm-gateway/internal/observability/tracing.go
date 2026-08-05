package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.34.0"
)

func InitTracing(ctx context.Context, endpoint string, sampleRate float64) (*tracesdk.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName("llm-gateway")))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(tracesdk.WithBatcher(exporter), tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(sampleRate))), tracesdk.WithResource(res))
	otel.SetTracerProvider(tp)
	return tp, nil
}
