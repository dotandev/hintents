// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

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

type noopMethodTelemetry struct{}

func (noopMethodTelemetry) StartMethodTimer(_ context.Context, _ string, _ map[string]string) MethodTimer {
	return noopMethodTimer{}
}

type noopMethodTimer struct{}

func (noopMethodTimer) Stop(_ error) {}

func defaultMethodTelemetry() MethodTelemetry {
	return noopMethodTelemetry{}
}
