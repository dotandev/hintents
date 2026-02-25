// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config holds OpenTelemetry configuration
type Config struct {
	Enabled     bool
	ExporterURL string
	ServiceName string
}

// MethodTelemetry provides duration tracking for SDK methods
type MethodTelemetry interface {
	// RecordMethodDuration records the execution duration of a method
	RecordMethodDuration(ctx context.Context, methodName string, duration time.Duration, attrs ...attribute.KeyValue)

	// WithMethodTiming creates a span with automatic timing for a method
	WithMethodTiming(ctx context.Context, methodName string, attrs ...attribute.KeyValue) (context.Context, func())
}

// methodTelemetry implements MethodTelemetry interface
type methodTelemetry struct {
	tracer oteltrace.Tracer
	meter  metric.Meter

	// Histogram for method durations
	durationHistogram metric.Float64Histogram
}

// Global method telemetry instance
var globalMethodTelemetry MethodTelemetry = &noOpMethodTelemetry{}

// noOpMethodTelemetry provides a no-op implementation when telemetry is disabled
type noOpMethodTelemetry struct{}

func (t *noOpMethodTelemetry) RecordMethodDuration(ctx context.Context, methodName string, duration time.Duration, attrs ...attribute.KeyValue) {
	// No-op when telemetry is disabled
}

func (t *noOpMethodTelemetry) WithMethodTiming(ctx context.Context, methodName string, attrs ...attribute.KeyValue) (context.Context, func()) {
	return ctx, func() {} // No-op when telemetry is disabled
}

// Init initializes OpenTelemetry with the given configuration
func Init(ctx context.Context, config Config) (func(), error) {
	if !config.Enabled {
		return func() {}, nil
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.ExporterURL),
		otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for local development
	)
	if err != nil {
		return nil, err
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String("dev"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	// Initialize method telemetry
	tracer := otel.Tracer("erst")
	meter := otel.Meter("erst")

	durationHistogram, err := meter.Float64Histogram(
		"method_duration_seconds",
		metric.WithDescription("Duration of SDK method executions in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	globalMethodTelemetry = &methodTelemetry{
		tracer:           tracer,
		meter:            meter,
		durationHistogram: durationHistogram,
	}

	// Return cleanup function
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx) // Ignore error in cleanup
	}, nil
}

// GetTracer returns the global tracer instance
func GetTracer() oteltrace.Tracer {
	return otel.Tracer("erst")
}

// GetMethodTelemetry returns the global method telemetry instance
func GetMethodTelemetry() MethodTelemetry {
	return globalMethodTelemetry
}

// RecordMethodDuration records the execution duration of a method
func (t *methodTelemetry) RecordMethodDuration(ctx context.Context, methodName string, duration time.Duration, attrs ...attribute.KeyValue) {
	if t.durationHistogram != nil {
		// Add method name attribute
		allAttrs := append([]attribute.KeyValue{attribute.String("method.name", methodName)}, attrs...)

		// Record duration in seconds
		t.durationHistogram.Record(ctx, duration.Seconds(), metric.WithAttributes(allAttrs...))
	}
}

// WithMethodTiming creates a span with automatic timing for a method
func (t *methodTelemetry) WithMethodTiming(ctx context.Context, methodName string, attrs ...attribute.KeyValue) (context.Context, func()) {
	// Add method name attribute
	allAttrs := append([]attribute.KeyValue{attribute.String("method.name", methodName)}, attrs...)

	// Create span for method timing
	ctx, span := t.tracer.Start(ctx, methodName, oteltrace.WithAttributes(allAttrs...))
	startTime := time.Now()

	return ctx, func() {
		duration := time.Since(startTime)

		// Record duration as metric
		t.RecordMethodDuration(ctx, methodName, duration, attrs...)

		// End the span with duration attribute
		span.SetAttributes(attribute.Float64("method.duration_seconds", duration.Seconds()))
		span.End()
	}
}
