package main

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracing(ctx context.Context) (func(context.Context) error, error) {
	if os.Getenv("OTEL_SDK_DISABLED") == "true" ||
		(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "") {
		return func(context.Context) error { return nil }, nil
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", "api")),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing resource: %w", err)
	}
	var exporter sdktrace.SpanExporter
	protocol := env("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", env("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc"))
	switch protocol {
	case "grpc":
		exporter, err = otlptracegrpc.New(ctx)
	case "http/protobuf":
		exporter, err = otlptracehttp.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTLP traces protocol %q; use grpc or http/protobuf", protocol)
	}
	if err != nil {
		return nil, fmt.Errorf("tracing exporter: %w", err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}
