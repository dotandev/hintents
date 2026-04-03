// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package debug

import (
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/types"
)

// FileExtension is the conventional extension for snapshot registry files.
const FileExtension = ".erstsnap"

// Registry stores all state required to replay a time-travel debug session
// without reconnecting to the Stellar network.
type Registry struct {
	// Version is the Erst release that created this file.
	Version string `json:"version"`
	// CreatedAt is when the session was saved.
	CreatedAt time.Time `json:"created_at"`
	// TxHash is the transaction that was debugged.
	TxHash string `json:"tx_hash"`
	// Network is the Stellar network the transaction was fetched from.
	Network string `json:"network"`
	// EnvelopeXdr is the base64-encoded transaction envelope.
	EnvelopeXdr string `json:"envelope_xdr"`
	// ResultMetaXdr is the base64-encoded transaction result metadata.
	ResultMetaXdr string `json:"result_meta_xdr"`
	// Entries holds one ledger snapshot per simulated timestamp.
	Entries []Entry `json:"entries"`
}

// Entry pairs a simulated timestamp with the ledger snapshot used at that point.
type Entry struct {
	Timestamp int64              `json:"timestamp"`
	Snapshot  *snapshot.Snapshot `json:"snapshot"`
	// Fingerprint is the xxHash-64 digest of the snapshot computed when the entry
	// was saved.  A value of zero means the fingerprint was not recorded (e.g.
	// legacy files) and integrity verification is skipped for that entry.
	Fingerprint uint64 `json:"fingerprint,omitempty"`
}

// IntegrityWarning describes a fingerprint mismatch detected by VerifyIntegrity.
type IntegrityWarning struct {
	// Timestamp is the simulated ledger timestamp of the affected entry.
	Timestamp int64
	// Stored is the fingerprint that was saved alongside the snapshot.
	Stored uint64
	// Computed is the fingerprint re-derived from the snapshot data on load.
	Computed uint64
}

// Error returns a human-readable description of the mismatch.
func (w IntegrityWarning) Error() string {
	return fmt.Sprintf(
		"snapshot at ts=%d: stored fingerprint %016x does not match computed %016x",
		w.Timestamp, w.Stored, w.Computed,
	)
}

// New returns an empty Registry for the given transaction.
func New(version, txHash, network, envelopeXdr, resultMetaXdr string) *Registry {
	return &Registry{
		Version:       version,
		CreatedAt:     time.Now().UTC(),
		TxHash:        txHash,
		Network:       network,
		EnvelopeXdr:   envelopeXdr,
		ResultMetaXdr: resultMetaXdr,
	}
}

// Add appends a ledger snapshot captured at the given simulation timestamp.
// A fingerprint is computed and stored so that VerifyIntegrity can detect
// corruption when the registry is loaded from disk later.
func (r *Registry) Add(timestamp int64, snap *snapshot.Snapshot) {
	fp, _ := types.Fingerprint(snap) // 0 on error; treated as "fingerprint unavailable"
	r.Entries = append(r.Entries, Entry{
		Timestamp:   timestamp,
		Snapshot:    snap,
		Fingerprint: fp,
	})
}

// VerifyIntegrity recomputes the xxHash fingerprint for every entry and returns
// a warning for each entry whose stored fingerprint does not match the computed
// one.  Entries with a stored fingerprint of zero are skipped because they
// pre-date fingerprint support.
func (r *Registry) VerifyIntegrity() []IntegrityWarning {
	var warnings []IntegrityWarning
	for _, entry := range r.Entries {
		if entry.Fingerprint == 0 {
			continue
		}
		computed, err := types.Fingerprint(entry.Snapshot)
		if err != nil || computed != entry.Fingerprint {
			warnings = append(warnings, IntegrityWarning{
				Timestamp: entry.Timestamp,
				Stored:    entry.Fingerprint,
				Computed:  computed,
			})
		}
	}
	return warnings
}

// SnapshotAt returns the snapshot whose timestamp is closest to ts.
// Returns nil when the registry is empty.
func (r *Registry) SnapshotAt(ts int64) *snapshot.Snapshot {
	if len(r.Entries) == 0 {
		return nil
	}
	best := &r.Entries[0]
	bestDiff := absDiff(r.Entries[0].Timestamp, ts)
	for i := 1; i < len(r.Entries); i++ {
		if d := absDiff(r.Entries[i].Timestamp, ts); d < bestDiff {
			best = &r.Entries[i]
			bestDiff = d
		}
	}
	return best.Snapshot
}

func absDiff(a, b int64) int64 {
	if d := a - b; d < 0 {
		return -d
	} else {
		return d
	}
}
