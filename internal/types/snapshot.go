// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"encoding/json"
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/dotandev/hintents/internal/snapshot"
)

// Fingerprint returns a 64-bit xxHash digest of the canonical serialization of
// snap's ledger entries and linear memory.
//
// xxHash-64 is used rather than SHA-256 because registry verification runs
// once per entry on every load: at thousands of snapshots the difference in
// throughput is meaningful. The goal is integrity detection (corruption,
// truncation, accidental edit), not cryptographic resistance.
//
// The input to the hash is the JSON encoding of the snapshot as produced by
// json.Marshal. Because snapshot.FromMap sorts entries by key before
// construction, this output is deterministic across equivalent snapshots.
//
// A nil snapshot returns (0, nil).
func Fingerprint(snap *snapshot.Snapshot) (uint64, error) {
	if snap == nil {
		return 0, nil
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return 0, fmt.Errorf("marshal snapshot for fingerprint: %w", err)
	}
	return xxhash.Sum64(data), nil
}
