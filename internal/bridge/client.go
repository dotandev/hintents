// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package bridge wires snapshot compression into the IPC request pipeline.
// CompressRequest replaces the plain ledger_entries map with a Zstd-compressed,
// base64-encoded blob in ledger_entries_zstd so the Rust simulator can detect
// and decompress it automatically.
package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ipcRequest is a minimal view of the simulator.SimulationRequest used for
// compression surgery without importing the simulator package (avoids cycles).
type ipcRequest struct {
	LedgerEntries     map[string]string `json:"ledger_entries,omitempty"`
	LedgerEntriesZstd string            `json:"ledger_entries_zstd,omitempty"`
}

// CompressRequest takes the raw JSON bytes of a SimulationRequest, compresses
// the ledger_entries map with Zstd, and returns updated JSON bytes.
// If ledger_entries is absent or empty the input is returned unchanged.
func CompressRequest(reqJSON []byte) ([]byte, error) {
	// Unmarshal only the fields we care about.
	var partial ipcRequest
	if err := json.Unmarshal(reqJSON, &partial); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal for compression: %w", err)
	}

	if len(partial.LedgerEntries) == 0 {
		return reqJSON, nil
	}

	compressed, err := CompressLedgerEntries(partial.LedgerEntries)
	if err != nil {
		return nil, err
	}

	// Patch the raw JSON: remove ledger_entries, inject ledger_entries_zstd.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(reqJSON, &raw); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal raw map: %w", err)
	}

	delete(raw, "ledger_entries")

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(compressed))
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal zstd field: %w", err)
	}
	raw["ledger_entries_zstd"] = encoded

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bridge: re-marshal compressed request: %w", err)
	}
	return out, nil
}
