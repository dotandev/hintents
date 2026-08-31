// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry configuration
type Config struct {
	Enabled        bool
	ExporterURL    string
	ServiceName    string
	ServiceVersion string
}

// noopErrorHandler swallows all errors.
type noopErrorHandler struct{}

func (noopErrorHandler) Handle(err error) {}

// Init initializes OpenTelemetry with the given configuration.
// Graceful degradation: if the metrics collector is unreachable or init fails,
// a no-op provider is used instead so the application never blocks or errors.
// Export failures are swallowed; telemetry fails silently.
func Init(ctx context.Context, config Config) (func(), error) {
	if !config.Enabled {
		return func() {}, nil
	}

	// Set global no-op error handler to swallow export failures.
	otel.SetErrorHandler(noopErrorHandler{})

	// Create OTLP HTTP exporter (best-effort; short timeout to avoid blocking)
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.ExporterURL),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithTimeout(5*time.Second),
	)
	if err != nil {
		// Collector unreachable at init: use no-op so core paths are unaffected
		otel.SetTracerProvider(trace.NewTracerProvider())
		return func() {}, nil
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		otel.SetTracerProvider(trace.NewTracerProvider())
		return func() {}, nil
	}

	// Create trace provider with exporter
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}, nil
}

// GetTracer returns the global tracer instance
func GetTracer() oteltrace.Tracer {
	return otel.Tracer("erst")
}
