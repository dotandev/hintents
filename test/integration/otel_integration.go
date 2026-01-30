// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dotandev/hintents/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Simple integration test to verify OpenTelemetry spans are created
func main() {
	ctx := context.Background()

	// Initialize telemetry
	cleanup, err := telemetry.Init(ctx, telemetry.Config{
		Enabled:     true,
		ExporterURL: "http://localhost:4318",
		ServiceName: "erst-integration-test",
	})
	if err != nil {
		log.Printf("Failed to initialize telemetry (expected if no OTLP endpoint): %v", err)
		return
	}
	defer cleanup()

	// Create test spans
	tracer := telemetry.GetTracer()
	ctx, rootSpan := tracer.Start(ctx, "integration_test")
	rootSpan.SetAttributes(attribute.String("test.type", "integration"))
	defer rootSpan.End()

	// Simulate nested operations
	_, childSpan := tracer.Start(ctx, "child_operation")
	childSpan.SetAttributes(
		attribute.String("operation.name", "test_operation"),
		attribute.Int("operation.count", 42),
	)
	childSpan.End()

	fmt.Println("Integration test completed successfully")
	fmt.Println("Check Jaeger UI at http://localhost:16686 for traces")
}
