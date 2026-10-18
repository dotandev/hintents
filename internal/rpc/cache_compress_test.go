// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storedCacheValue(t *testing.T, key string) []byte {
	t.Helper()
	db, err := ensureDB()
	require.NoError(t, err)

	var stored []byte
	require.NoError(t, db.QueryRow(
		"SELECT value FROM rpc_cache WHERE key_hash = ?", getCacheKey(key),
	).Scan(&stored))
	return stored
}

func TestCache_CompressesLedgerEntries(t *testing.T) {
	setupTestCacheDB(t)

	samples := state.SEP41Samples()
	for i, s := range samples[:8] {
		key := "entry-" + string(rune('a'+i))
		value := base64.StdEncoding.EncodeToString(s)
		require.NoError(t, Set(key, value))

		stored := storedCacheValue(t, key)
		assert.True(t, state.IsEncoded(stored), "ledger entry should be compressed on disk")
		assert.Less(t, len(stored), len(value))

		got, found, err := Get(key)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, value, got)
	}
}

func TestCache_ReadsLegacyUncompressedRows(t *testing.T) {
	setupTestCacheDB(t)
	db, err := ensureDB()
	require.NoError(t, err)

	legacy := base64.StdEncoding.EncodeToString(state.SEP41Samples()[0])
	now := time.Now()
	_, err = db.Exec(
		`INSERT INTO rpc_cache (key_hash, cache_key, value, network, created_at, expires_at)
		 VALUES (?, ?, ?, '', ?, ?)`,
		getCacheKey("legacy"), "legacy", legacy, now.UnixNano(), now.Add(time.Hour).UnixNano(),
	)
	require.NoError(t, err)

	got, found, err := Get("legacy")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, legacy, got)
}

func TestCache_UndecodableRowIsMiss(t *testing.T) {
	setupTestCacheDB(t)
	db, err := ensureDB()
	require.NoError(t, err)

	now := time.Now()
	_, err = db.Exec(
		`INSERT INTO rpc_cache (key_hash, cache_key, value, network, created_at, expires_at)
		 VALUES (?, ?, ?, '', ?, ?)`,
		getCacheKey("corrupt"), "corrupt", []byte{0x00, 0x01, 0xde, 0xad, 0xbe, 0xef},
		now.UnixNano(), now.Add(time.Hour).UnixNano(),
	)
	require.NoError(t, err)

	_, found, err := Get("corrupt")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestCache_EmptyValueRoundTrip(t *testing.T) {
	setupTestCacheDB(t)

	require.NoError(t, Set("empty", ""))
	got, found, err := Get("empty")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "", got)
}
