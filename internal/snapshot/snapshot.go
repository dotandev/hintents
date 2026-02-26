// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LedgerEntryTuple represents a (Key, Value) pair where both are Base64 XDR strings.
// Using a slice []string of length 2 ensures strict ordering and JSON array serialization ["key", "val"].
type LedgerEntryTuple []string

// Snapshot represents the structure of a soroban-cli compatible snapshot file.
// strict schema compatibility: "ledgerEntries" key containing list of tuples.
type Snapshot struct {
	LedgerEntries []LedgerEntryTuple `json:"ledgerEntries"`
}

// FromMap converts the internal map representation to a Snapshot.
// Enforces deterministic ordering by sorting keys.
func FromMap(m map[string]string) *Snapshot {
	if m == nil {
		return &Snapshot{LedgerEntries: make([]LedgerEntryTuple, 0)}
	}

	entries := make([]LedgerEntryTuple, 0, len(m))
	for k, v := range m {
		entries = append(entries, LedgerEntryTuple{k, v})
	}

	// Sort by key for deterministic serialization
	sort.Slice(entries, func(i, j int) bool {
		return entries[i][0] < entries[j][0]
	})

	return &Snapshot{LedgerEntries: entries}
}

// ToMap converts the Snapshot back to the internal map representation.
func (s *Snapshot) ToMap() map[string]string {
	m := make(map[string]string)
	if s.LedgerEntries == nil {
		return m
	}
	for _, entry := range s.LedgerEntries {
		if len(entry) >= 2 {
			m[entry[0]] = entry[1]
		}
	}
	return m
}

// Load reads a snapshot from a JSON file.
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot JSON: %w", err)
	}

	return &snap, nil
}

// Save writes a snapshot to a JSON file with indentation for readability.
func Save(path string, snap *Snapshot) error {
	stable := normalizedForSave(snap)

	data, err := json.MarshalIndent(stable, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

func normalizedForSave(snap *Snapshot) *Snapshot {
	if snap == nil {
		return &Snapshot{LedgerEntries: make([]LedgerEntryTuple, 0)}
	}

	entries := make([]LedgerEntryTuple, 0, len(snap.LedgerEntries))
	for _, entry := range snap.LedgerEntries {
		copied := append(LedgerEntryTuple(nil), entry...)
		entries = append(entries, copied)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left := ""
		right := ""
		if len(entries[i]) > 0 {
			left = entries[i][0]
		}
		if len(entries[j]) > 0 {
			right = entries[j][0]
		}
		return left < right
	})

	return &Snapshot{LedgerEntries: entries}
}
