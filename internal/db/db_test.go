// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestInitSchema_FreshDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Ensure initial version is 0
	initialVersion, err := getSchemaVersion(db)
	require.NoError(t, err)
	assert.Equal(t, 0, initialVersion)

	// Run migrations
	err = initSchema(db)
	require.NoError(t, err)

	// Verify version upgraded to CurrentSchemaVersion
	version, err := getSchemaVersion(db)
	require.NoError(t, err)
	assert.Equal(t, CurrentSchemaVersion, version)

	// Verify table exists
	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sessions';").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify indices exist
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_tx_hash';").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_error';").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInitSchema_Idempotency(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// First initialization
	err = initSchema(db)
	require.NoError(t, err)

	// Second initialization should be an idempotent no-op
	err = initSchema(db)
	require.NoError(t, err)

	version, err := getSchemaVersion(db)
	require.NoError(t, err)
	assert.Equal(t, CurrentSchemaVersion, version)
}

func TestInitSchema_ExistingV0Database(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Simulate an existing database created before version tracking (user_version = 0)
	_, err = db.Exec(`
	CREATE TABLE sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tx_hash TEXT NOT NULL,
		network TEXT NOT NULL,
		status TEXT,
		error_msg TEXT,
		events TEXT,
		logs TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO sessions (tx_hash, network, status) VALUES ('0xabc', 'testnet', 'success');
	`)
	require.NoError(t, err)

	// Run migration
	err = initSchema(db)
	require.NoError(t, err)

	// Version should be upgraded to CurrentSchemaVersion
	version, err := getSchemaVersion(db)
	require.NoError(t, err)
	assert.Equal(t, CurrentSchemaVersion, version)

	// Existing row must be preserved
	var txHash string
	err = db.QueryRow("SELECT tx_hash FROM sessions WHERE id = 1;").Scan(&txHash)
	require.NoError(t, err)
	assert.Equal(t, "0xabc", txHash)
}
