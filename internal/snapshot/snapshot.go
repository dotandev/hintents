// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
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
	LinearMemory  string             `json:"linearMemory,omitempty"`
	// Fingerprint is the SHA-256 hex digest of the sorted (Key, Value) pairs.
	// It is computed deterministically so any two snapshots with identical ledger
	// state produce the same fingerprint, enabling drift detection.
	Fingerprint string `json:"fingerprint,omitempty"`
}

type BuildOptions struct {
	LinearMemory []byte
}

// ComputeFingerprint returns the SHA-256 hex digest of the snapshot's ledger
// entries. Entries are sorted by key before hashing to guarantee a deterministic
// result regardless of insertion order.
//
// The hash input is the concatenation of each (key, value) pair encoded as:
//
//	<key-len-4-bytes-big-endian><key-bytes><value-len-4-bytes-big-endian><value-bytes>
//
// This framing prevents collisions between adjacent keys/values.
func ComputeFingerprint(snap *Snapshot) string {
	if snap == nil {
		return ""
	}

	// Work on a sorted copy so the caller's slice is not mutated.
	entries := make([]LedgerEntryTuple, len(snap.LedgerEntries))
	copy(entries, snap.LedgerEntries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i][0] < entries[j][0]
	})

	h := sha256.New()
	buf := make([]byte, 4)
	for _, entry := range entries {
		if len(entry) < 2 {
			continue
		}
		writeFramed(h, buf, []byte(entry[0]))
		writeFramed(h, buf, []byte(entry[1]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeFramed writes a length-prefixed byte slice into w.
func writeFramed(w interface{ Write([]byte) (int, error) }, buf []byte, data []byte) {
	n := uint32(len(data))
	buf[0] = byte(n >> 24)
	buf[1] = byte(n >> 16)
	buf[2] = byte(n >> 8)
	buf[3] = byte(n)
	_, _ = w.Write(buf)
	_, _ = w.Write(data)
}

// FromMap converts the internal map representation to a Snapshot.
// Enforces deterministic ordering by sorting keys.
func FromMap(m map[string]string) *Snapshot {
	return FromMapWithOptions(m, BuildOptions{})
}

func FromMapWithOptions(m map[string]string, opts BuildOptions) *Snapshot {
	if m == nil {
		s := &Snapshot{LedgerEntries: make([]LedgerEntryTuple, 0), LinearMemory: encodeMemory(opts.LinearMemory)}
		s.Fingerprint = ComputeFingerprint(s)
		return s
	}

	entries := make([]LedgerEntryTuple, 0, len(m))
	for k, v := range m {
		entries = append(entries, LedgerEntryTuple{k, v})
	}

	// Sort by key for deterministic serialization
	sort.Slice(entries, func(i, j int) bool {
		return entries[i][0] < entries[j][0]
	})

	s := &Snapshot{LedgerEntries: entries, LinearMemory: encodeMemory(opts.LinearMemory)}
	s.Fingerprint = ComputeFingerprint(s)
	return s
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

func (s *Snapshot) DecodeLinearMemory() ([]byte, error) {
	if s == nil || s.LinearMemory == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(s.LinearMemory)
	if err != nil {
		return nil, fmt.Errorf("failed to decode linear memory: %w", err)
	}
	return decoded, nil
}

// Load reads a snapshot from a JSON file.
// If the file contains a fingerprint, it is verified against the loaded entries.
// A mismatch is logged immediately as a drift warning.
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot JSON: %w", err)
	}

	computed := ComputeFingerprint(&snap)
	if snap.Fingerprint == "" {
		// Back-fill fingerprint for snapshots saved before this feature.
		snap.Fingerprint = computed
	} else if snap.Fingerprint != computed {
		log.Printf("DRIFT DETECTED: snapshot %q fingerprint mismatch: stored=%s computed=%s",
			path, snap.Fingerprint, computed)
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

	normalized := &Snapshot{LedgerEntries: entries, LinearMemory: snap.LinearMemory}
	normalized.Fingerprint = ComputeFingerprint(normalized)
	return normalized
}

func encodeMemory(memory []byte) string {
	if len(memory) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(memory)
}
