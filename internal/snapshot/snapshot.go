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
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}
