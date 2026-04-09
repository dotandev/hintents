// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"encoding/json"
	"testing"
)

func TestStateSnapshotUnmarshal(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"ledger_entries": {
			"AAAAAQAAAAAAAAA=": "AAAAAgAAAAAAAAA=",
			"AAAAAQAAAAAAAAE=": "AAAAAgAAAAAAAAE="
		},
		"timestamp": 1711651200,
		"instruction_index": 42,
		"events": [
			"ev:invoke:contract",
			"ev:return:ok"
		]
	}`)

	var got StateSnapshot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal StateSnapshot: %v", err)
	}

	if got.Timestamp != 1711651200 {
		t.Fatalf("unexpected timestamp: got %d", got.Timestamp)
	}

	if got.InstructionIndex != 42 {
		t.Fatalf("unexpected instruction_index: got %d", got.InstructionIndex)
	}

	if len(got.LedgerEntries) != 2 {
		t.Fatalf("unexpected ledger_entries length: got %d", len(got.LedgerEntries))
	}

	if got.LedgerEntries["AAAAAQAAAAAAAAA="] != "AAAAAgAAAAAAAAA=" {
		t.Fatalf("unexpected ledger entry value for first key")
	}

	if len(got.Events) != 2 {
		t.Fatalf("unexpected events length: got %d", len(got.Events))
	}
}

func TestLedgerDeltaUnmarshal(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"new_keys": ["deadbeef", "cafebabe"],
		"modified_keys": ["010203"],
		"deleted_keys": ["f00d"]
	}`)

	var got LedgerDelta
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal LedgerDelta: %v", err)
	}

	if len(got.NewKeys) != 2 || got.NewKeys[0] != "deadbeef" || got.NewKeys[1] != "cafebabe" {
		t.Fatalf("unexpected new_keys: %#v", got.NewKeys)
	}

	if len(got.ModifiedKeys) != 1 || got.ModifiedKeys[0] != "010203" {
		t.Fatalf("unexpected modified_keys: %#v", got.ModifiedKeys)
	}

	if len(got.DeletedKeys) != 1 || got.DeletedKeys[0] != "f00d" {
		t.Fatalf("unexpected deleted_keys: %#v", got.DeletedKeys)
	}
package types_test

import (
	"testing"

	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint_NilSnapshot(t *testing.T) {
	fp, err := types.Fingerprint(nil)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), fp)
}

func TestFingerprint_Deterministic(t *testing.T) {
	entries := map[string]string{
		"key_b": "val_b",
		"key_a": "val_a",
	}
	snap := snapshot.FromMap(entries)

	fp1, err := types.Fingerprint(snap)
	require.NoError(t, err)

	fp2, err := types.Fingerprint(snap)
	require.NoError(t, err)

	assert.Equal(t, fp1, fp2, "identical snapshots must produce identical fingerprints")
}

func TestFingerprint_ChangedEntryProducesDifferentHash(t *testing.T) {
	snap1 := snapshot.FromMap(map[string]string{"key_a": "val_a"})
	snap2 := snapshot.FromMap(map[string]string{"key_a": "val_b"})

	fp1, err := types.Fingerprint(snap1)
	require.NoError(t, err)

	fp2, err := types.Fingerprint(snap2)
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "different ledger values must produce different fingerprints")
}

func TestFingerprint_LinearMemoryAffectsHash(t *testing.T) {
	entries := map[string]string{"k": "v"}

	snapWithoutMem := snapshot.FromMapWithOptions(entries, snapshot.BuildOptions{})
	snapWithMem := snapshot.FromMapWithOptions(entries, snapshot.BuildOptions{LinearMemory: []byte("memory data")})

	fp1, err := types.Fingerprint(snapWithoutMem)
	require.NoError(t, err)

	fp2, err := types.Fingerprint(snapWithMem)
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "presence of linear memory must change the fingerprint")
}

func TestFingerprint_EmptySnapshot(t *testing.T) {
	snap := snapshot.FromMap(nil)

	fp, err := types.Fingerprint(snap)
	require.NoError(t, err)
	assert.NotEqual(t, uint64(0), fp, "empty but non-nil snapshot should produce a non-zero fingerprint")
}
