// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestInit(t *testing.T) {
	ctx := context.Background()

	// Test with tracing disabled
	cleanup, err := Init(ctx, Config{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to initialize telemetry with disabled config: %v", err)
	}
	cleanup()

	// Test with tracing enabled (will fail if no OTLP endpoint, but shouldn't crash)
	cleanup, err = Init(ctx, Config{
		Enabled:     true,
		ExporterURL: "http://localhost:4318",
		ServiceName: "test-service",
	})
	if err != nil {
		// This is expected if no OTLP endpoint is running
		t.Logf("Expected error when no OTLP endpoint available: %v", err)
		return
	}

	// Test that tracer is available
	tracer := GetTracer()
	if tracer == nil {
		t.Fatal("Tracer should not be nil after initialization")
	}

	// Test creating a span
	_, span := tracer.Start(ctx, "test-span")
	span.End()

	cleanup()
}

func TestGetTracer(t *testing.T) {
	// Should not panic even if not initialized
	tracer := GetTracer()
	if tracer == nil {
		t.Fatal("GetTracer should never return nil")
	}

	// Should be able to create spans (no-op if not initialized)
	ctx := context.Background()
	_, span := tracer.Start(ctx, "test-span")
	span.End()
}

func TestGetMethodTelemetry(t *testing.T) {
	// Should not panic and should never return nil
	methodTelemetry := GetMethodTelemetry()
	if methodTelemetry == nil {
		t.Fatal("GetMethodTelemetry should never return nil")
	}
}

func TestNoOpMethodTelemetry(t *testing.T) {
	// Test no-op implementation when telemetry is disabled
	ctx := context.Background()
	methodTelemetry := GetMethodTelemetry()

	// Should not panic
	methodTelemetry.RecordMethodDuration(ctx, "test_method", 100*time.Millisecond)

	// Should not panic and return valid context and no-op function
	ctx2, endTiming := methodTelemetry.WithMethodTiming(ctx, "test_method", attribute.String("test", "value"))
	if ctx2 == nil {
		t.Fatal("WithMethodTiming should return non-nil context")
	}
	if endTiming == nil {
		t.Fatal("WithMethodTiming should return non-nil endTiming function")
	}

	// Should not panic when called
	endTiming()
}

func TestMethodTelemetryWithInitialization(t *testing.T) {
	ctx := context.Background()

	// Initialize telemetry (will fail if no OTLP endpoint, but that's ok for testing)
	cleanup, err := Init(ctx, Config{
		Enabled:     true,
		ExporterURL: "http://localhost:4318",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Logf("Expected error when no OTLP endpoint available: %v", err)
		// Continue with no-op implementation
	}

	methodTelemetry := GetMethodTelemetry()

	// Test RecordMethodDuration
	methodTelemetry.RecordMethodDuration(ctx, "test_method", 50*time.Millisecond,
		attribute.String("test_attr", "test_value"))

	// Test WithMethodTiming
	ctx2, endTiming := methodTelemetry.WithMethodTiming(ctx, "test_method",
		attribute.Int("test_int", 42))

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// End timing
	endTiming()

	if cleanup != nil {
		cleanup()
	}
}

func TestMethodTelemetryAttributes(t *testing.T) {
	ctx := context.Background()
	methodTelemetry := GetMethodTelemetry()

	// Test with various attribute types
	attrs := []attribute.KeyValue{
		attribute.String("string_attr", "test"),
		attribute.Int("int_attr", 42),
		attribute.Bool("bool_attr", true),
		attribute.Float64("float_attr", 3.14),
	}

	// Should not panic with various attributes
	methodTelemetry.RecordMethodDuration(ctx, "test_with_attrs", 25*time.Millisecond, attrs...)

	ctx2, endTiming := methodTelemetry.WithMethodTiming(ctx, "test_with_attrs", attrs...)
	endTiming()
}
