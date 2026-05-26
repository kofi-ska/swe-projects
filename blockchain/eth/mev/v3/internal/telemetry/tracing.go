package telemetry

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type TraceConfig struct {
	ServiceName string
	Endpoint    string
	Insecure    bool
	SampleRatio float64
}

func InitTracing(ctx context.Context, cfg TraceConfig) (func(context.Context) error, error) {
	if cfg.ServiceName == "" || cfg.Endpoint == "" {
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	if !cfg.Insecure {
		return nil, errors.New("secure OTLP transport is not wired in this build")
	}
	if cfg.SampleRatio <= 0 || cfg.SampleRatio > 1 {
		cfg.SampleRatio = 0.25
	}
	res, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceInstanceIDKey.String(cfg.ServiceName+"-"+time.Now().UTC().Format("20060102T150405Z")),
		),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithBatcher(exp),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}
