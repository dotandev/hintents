// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package types

// SnapshotStatus represents the availability state of snapshots for a simulation.
type SnapshotStatus string

const (
	// SnapshotEnabled indicates snapshots are fully available.
	SnapshotEnabled SnapshotStatus = "ENABLED"

	// SnapshotDisabled indicates snapshots are not available.
	SnapshotDisabled SnapshotStatus = "DISABLED"

	// SnapshotPartial indicates some snapshots could not be captured (e.g. oversized frames).
	SnapshotPartial SnapshotStatus = "PARTIAL"

	// SnapshotErrorOOM indicates snapshot capture failed due to out-of-memory conditions.
	SnapshotErrorOOM SnapshotStatus = "ERROR_OOM"
)

// StatusMessage returns a human-readable message describing the snapshot status.
func (s SnapshotStatus) StatusMessage() string {
	switch s {
	case SnapshotEnabled:
		return "Snapshots are fully available"
	case SnapshotDisabled:
		return "Snapshots are not available for this simulation"
	case SnapshotPartial:
		return "Warning: Some snapshots could not be captured due to oversized frames"
	case SnapshotErrorOOM:
		return "Error: Snapshot capture failed due to memory pressure (OOM)"
	default:
		return "Unknown snapshot status"
	}
}

// IsHealthy returns true if snapshots are fully operational.
func (s SnapshotStatus) IsHealthy() bool {
	return s == SnapshotEnabled
}

// SimulationMetadata holds metadata about a simulation run, including snapshot state.
type SimulationMetadata struct {
	TransactionHash string         `json:"transaction_hash"`
	Network         string         `json:"network"`
	SnapshotStatus  SnapshotStatus `json:"snapshot_status"`
	TotalSteps      int            `json:"total_steps"`
	SnapshotCount   int            `json:"snapshot_count"`
}

// DetermineSnapshotStatus evaluates the current simulation state and returns the
// appropriate SnapshotStatus based on snapshot availability and error conditions.
func DetermineSnapshotStatus(totalSteps, snapshotCount int, hasOOMError bool) SnapshotStatus {
	if hasOOMError {
		return SnapshotErrorOOM
	}
	if snapshotCount == 0 {
		return SnapshotDisabled
	}
	// If we have significantly fewer snapshots than expected, mark as partial.
	// Expected: roughly totalSteps / interval, but at minimum 1.
	if totalSteps > 0 && snapshotCount > 0 && snapshotCount < totalSteps/200 {
		return SnapshotPartial
	}
	return SnapshotEnabled
}
