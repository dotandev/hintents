// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"testing"
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
