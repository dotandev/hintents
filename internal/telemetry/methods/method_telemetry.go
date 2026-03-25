// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package methods

import "context"

// MethodTelemetry is an optional SDK hook for timing method execution.
// Implementations can forward timings to metrics/telemetry backends.
type MethodTelemetry interface {
	StartMethodTimer(ctx context.Context, method string, attributes map[string]string) MethodTimer
}

// MethodTimer represents a started method execution timer.
type MethodTimer interface {
	Stop(err error)
}

var (
	_ MethodTelemetry = (*noopMethodTelemetry)(nil)
	_ MethodTimer     = (*NoopMethodTimer)(nil)
)

type noopMethodTelemetry struct{}

func (noopMethodTelemetry) StartMethodTimer(_ context.Context, _ string, _ map[string]string) MethodTimer {
	return NoopMethodTimer{}
}

type NoopMethodTimer struct{}

func (NoopMethodTimer) Stop(_ error) {}

// DefaultMethodTelemetry returns a no-op implementation of MethodTelemetry.
// Use this when you want to disable method telemetry.
func DefaultMethodTelemetry() MethodTelemetry {
	return noopMethodTelemetry{}
}
