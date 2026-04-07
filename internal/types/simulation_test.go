// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package types

import "testing"

func TestSnapshotStatus_StatusMessage(t *testing.T) {
	tests := []struct {
		status SnapshotStatus
		want   string
	}{
		{SnapshotEnabled, "Snapshots are fully available"},
		{SnapshotDisabled, "Snapshots are not available for this simulation"},
		{SnapshotPartial, "Warning: Some snapshots could not be captured due to oversized frames"},
		{SnapshotErrorOOM, "Error: Snapshot capture failed due to memory pressure (OOM)"},
		{SnapshotStatus("UNKNOWN"), "Unknown snapshot status"},
	}
	for _, tt := range tests {
		if got := tt.status.StatusMessage(); got != tt.want {
			t.Errorf("SnapshotStatus(%q).StatusMessage() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSnapshotStatus_IsHealthy(t *testing.T) {
	if !SnapshotEnabled.IsHealthy() {
		t.Error("SnapshotEnabled should be healthy")
	}
	for _, s := range []SnapshotStatus{SnapshotDisabled, SnapshotPartial, SnapshotErrorOOM} {
		if s.IsHealthy() {
			t.Errorf("SnapshotStatus(%q) should not be healthy", s)
		}
	}
}

func TestDetermineSnapshotStatus(t *testing.T) {
	tests := []struct {
		name          string
		totalSteps    int
		snapshotCount int
		hasOOM        bool
		want          SnapshotStatus
	}{
		{"OOM overrides all", 100, 10, true, SnapshotErrorOOM},
		{"No snapshots", 100, 0, false, SnapshotDisabled},
		{"Healthy", 100, 5, false, SnapshotEnabled},
		{"Partial - too few snapshots", 1000, 2, false, SnapshotPartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineSnapshotStatus(tt.totalSteps, tt.snapshotCount, tt.hasOOM)
			if got != tt.want {
				t.Errorf("DetermineSnapshotStatus(%d, %d, %v) = %q, want %q",
					tt.totalSteps, tt.snapshotCount, tt.hasOOM, got, tt.want)
			}
		})
	}
}
