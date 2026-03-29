// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package debug_test

import (
	"testing"

	"github.com/dotandev/hintents/internal/debug"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_VerifyIntegrity_Clean(t *testing.T) {
	reg := debug.New("v1", "txhash", "testnet", "env", "meta")
	reg.Add(1000, snapshot.FromMap(map[string]string{"k": "v"}))
	reg.Add(2000, snapshot.FromMap(map[string]string{"k2": "v2"}))

	warnings := reg.VerifyIntegrity()
	assert.Empty(t, warnings, "freshly written registry should have no integrity warnings")
}

func TestRegistry_VerifyIntegrity_DetectsMismatch(t *testing.T) {
	reg := debug.New("v1", "txhash", "testnet", "env", "meta")
	snap := snapshot.FromMap(map[string]string{"key": "original"})
	reg.Add(1000, snap)

	require.Len(t, reg.Entries, 1)

	// Tamper: replace the snapshot with different data but keep the old fingerprint.
	reg.Entries[0].Snapshot = snapshot.FromMap(map[string]string{"key": "tampered"})

	warnings := reg.VerifyIntegrity()
	require.Len(t, warnings, 1)
	assert.Equal(t, int64(1000), warnings[0].Timestamp)
	assert.NotEqual(t, warnings[0].Stored, warnings[0].Computed)
}

func TestRegistry_VerifyIntegrity_SkipsZeroFingerprint(t *testing.T) {
	reg := debug.New("v1", "txhash", "testnet", "env", "meta")
	reg.Add(1000, snapshot.FromMap(map[string]string{"k": "v"}))

	// Simulate a legacy entry with no stored fingerprint.
	reg.Entries[0].Fingerprint = 0

	warnings := reg.VerifyIntegrity()
	assert.Empty(t, warnings, "entries without a fingerprint should be skipped")
}

func TestRegistry_Add_StoresNonZeroFingerprint(t *testing.T) {
	reg := debug.New("v1", "txhash", "testnet", "env", "meta")
	reg.Add(1000, snapshot.FromMap(map[string]string{"k": "v"}))

	require.Len(t, reg.Entries, 1)
	assert.NotEqual(t, uint64(0), reg.Entries[0].Fingerprint,
		"Add should store a non-zero fingerprint for a non-empty snapshot")
}
