// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"bytes"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with both session and cache
// schemas, seeds data, and returns the db and a cleanup function.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	// Session schema (mirrors internal/session/store.go)
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP NOT NULL,
		last_access_at TIMESTAMP NOT NULL,
		status TEXT NOT NULL,
		network TEXT NOT NULL,
		horizon_url TEXT NOT NULL,
		tx_hash TEXT NOT NULL,
		envelope_xdr TEXT,
		result_xdr TEXT,
		result_meta_xdr TEXT,
		sim_request_json TEXT,
		sim_response_json TEXT,
		erst_version TEXT,
		schema_version INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_last_access ON sessions(last_access_at);
	CREATE INDEX IF NOT EXISTS idx_tx_hash ON sessions(tx_hash);
	`)
	require.NoError(t, err)

	// Cache schema (mirrors internal/rpc/cache.go)
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS rpc_cache (
		key_hash   TEXT PRIMARY KEY,
		cache_key  TEXT NOT NULL,
		value      TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_rpc_cache_expires ON rpc_cache(expires_at);
	`)
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })
	return db
}

func seedSession(t *testing.T, db *sql.DB, id, txHash string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO sessions (id, created_at, last_access_at, status, network, horizon_url, tx_hash, schema_version)
		VALUES (?, ?, ?, 'active', 'testnet', 'https://horizon-testnet.stellar.org', ?, 1)
	`, id, now, now, txHash)
	require.NoError(t, err)
}

func seedCacheEntry(t *testing.T, db *sql.DB, keyHash, cacheKey, value string) {
	t.Helper()
	now := time.Now().UnixNano()
	expires := time.Now().Add(24 * time.Hour).UnixNano()
	_, err := db.Exec(`
		INSERT INTO rpc_cache (key_hash, cache_key, value, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, keyHash, cacheKey, value, now, expires)
	require.NoError(t, err)
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	require.NoError(t, err)
	return count
}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// --- Classification tests ---

func TestClassifySQL_DestructiveStatements(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected DestructiveOp
	}{
		{"delete single row", "DELETE FROM sessions WHERE id = ?", OpDelete},
		{"delete with subquery", "DELETE FROM sessions WHERE id IN (SELECT id FROM sessions ORDER BY last_access_at ASC LIMIT ?)", OpDelete},
		{"delete expired", "DELETE FROM sessions WHERE last_access_at < ?", OpDelete},
		{"delete cache entry", "DELETE FROM rpc_cache WHERE key_hash = ?", OpDelete},
		{"delete expired cache", "DELETE FROM rpc_cache WHERE expires_at < ?", OpDelete},
		{"drop table", "DROP TABLE sessions", OpDrop},
		{"alter table", "ALTER TABLE sessions ADD COLUMN foo TEXT", OpAlter},
		{"truncate", "TRUNCATE TABLE sessions", OpTruncate},
		{"update row", "UPDATE sessions SET status = ? WHERE id = ?", OpUpdate},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := ClassifySQL(tc.query)
			assert.Equal(t, tc.expected, op)
		})
	}
}

func TestClassifySQL_SafeStatements(t *testing.T) {
	safeQueries := []struct {
		name  string
		query string
	}{
		{"select", "SELECT * FROM sessions"},
		{"insert", "INSERT INTO sessions (id) VALUES (?)"},
		{"create table", "CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY)"},
		{"create index", "CREATE INDEX IF NOT EXISTS idx_tx ON sessions(tx_hash)"},
		{"pragma", "PRAGMA journal_mode=WAL"},
	}

	for _, tc := range safeQueries {
		t.Run(tc.name, func(t *testing.T) {
			op := ClassifySQL(tc.query)
			assert.Equal(t, OpSafe, op)
		})
	}
}

func TestClassifySQL_CaseInsensitive(t *testing.T) {
	assert.Equal(t, OpDelete, ClassifySQL("delete from sessions"))
	assert.Equal(t, OpDrop, ClassifySQL("drop table foo"))
	assert.Equal(t, OpUpdate, ClassifySQL("update sessions set x = 1"))
}

func TestClassifySQL_LeadingWhitespace(t *testing.T) {
	assert.Equal(t, OpDelete, ClassifySQL("  DELETE FROM sessions WHERE id = ?"))
	assert.Equal(t, OpUpdate, ClassifySQL("\tUPDATE sessions SET x = 1"))
}

// --- DryRunExec tests ---

func TestDryRunExec_WarnsOnDestructiveSQL(t *testing.T) {
	db := setupTestDB(t)
	seedSession(t, db, "sess-1", "abc123")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	result := DryRunExec(log, db, "DELETE FROM sessions WHERE id = ?", "sess-1")

	assert.True(t, result.Destructive)
	assert.Equal(t, OpDelete, result.Operation)
	assert.Contains(t, buf.String(), "[DRY-RUN]")
	assert.Contains(t, buf.String(), "DELETE FROM sessions WHERE id = ?")
	assert.Equal(t, 1, countRows(t, db, "sessions"), "dry-run must not delete data")
}

func TestDryRunExec_SafeQueryNoWarning(t *testing.T) {
	db := setupTestDB(t)

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	result := DryRunExec(log, db, "SELECT * FROM sessions")

	assert.False(t, result.Destructive)
	assert.Equal(t, OpSafe, result.Operation)
	assert.Empty(t, buf.String())
}

// --- Data-integrity tests for every destructive query in the codebase ---

func TestDryRun_SessionDelete_NoDataLoss(t *testing.T) {
	db := setupTestDB(t)
	seedSession(t, db, "sess-1", "hash1")
	seedSession(t, db, "sess-2", "hash2")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	result := DryRunExec(log, db, `DELETE FROM sessions WHERE id = ?`, "sess-1")

	assert.True(t, result.Destructive)
	assert.Equal(t, 2, countRows(t, db, "sessions"))
	assert.Contains(t, buf.String(), "destructive SQL detected")
}

func TestDryRun_SessionCleanupExpired_NoDataLoss(t *testing.T) {
	db := setupTestDB(t)
	seedSession(t, db, "sess-old", "hash-old")
	seedSession(t, db, "sess-new", "hash-new")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	cutoff := time.Now().Add(time.Hour).Format(time.RFC3339)
	result := DryRunExec(log, db, `DELETE FROM sessions WHERE last_access_at < ?`, cutoff)

	assert.True(t, result.Destructive)
	assert.Equal(t, 2, countRows(t, db, "sessions"))
	assert.Contains(t, buf.String(), "DELETE")
}

func TestDryRun_SessionCleanupExcess_NoDataLoss(t *testing.T) {
	db := setupTestDB(t)
	seedSession(t, db, "s1", "h1")
	seedSession(t, db, "s2", "h2")
	seedSession(t, db, "s3", "h3")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	query := `
		DELETE FROM sessions
		WHERE id IN (
			SELECT id FROM sessions
			ORDER BY last_access_at ASC
			LIMIT ?
		)
	`
	result := DryRunExec(log, db, query, 2)

	assert.True(t, result.Destructive)
	assert.Equal(t, 3, countRows(t, db, "sessions"))
	assert.Contains(t, buf.String(), "DELETE FROM sessions")
}

func TestDryRun_CacheInvalidate_NoDataLoss(t *testing.T) {
	db := setupTestDB(t)
	seedCacheEntry(t, db, "keyhash1", "mykey", "myval")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	result := DryRunExec(log, db, "DELETE FROM rpc_cache WHERE key_hash = ?", "keyhash1")

	assert.True(t, result.Destructive)
	assert.Equal(t, 1, countRows(t, db, "rpc_cache"))
	assert.Contains(t, buf.String(), "rpc_cache")
}

func TestDryRun_CacheCleanup_NoDataLoss(t *testing.T) {
	db := setupTestDB(t)
	seedCacheEntry(t, db, "kh1", "k1", "v1")
	seedCacheEntry(t, db, "kh2", "k2", "v2")

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	cutoff := time.Now().Add(48 * time.Hour).UnixNano()
	result := DryRunExec(log, db, "DELETE FROM rpc_cache WHERE expires_at < ?", cutoff)

	assert.True(t, result.Destructive)
	assert.Equal(t, 2, countRows(t, db, "rpc_cache"))
	assert.Contains(t, buf.String(), "destructive SQL detected")
}

// --- Warning output format ---

func TestDryRunExec_WarningContainsArgs(t *testing.T) {
	db := setupTestDB(t)

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	DryRunExec(log, db, "DELETE FROM sessions WHERE id = ?", "target-id")

	output := buf.String()
	assert.Contains(t, output, "target-id")
	assert.Contains(t, output, "operation=DELETE")
}

func TestDryRunExec_AllDestructiveOpsWarn(t *testing.T) {
	db := setupTestDB(t)

	ops := []struct {
		query    string
		expected DestructiveOp
	}{
		{"DELETE FROM sessions WHERE id = ?", OpDelete},
		{"DROP TABLE sessions", OpDrop},
		{"ALTER TABLE sessions ADD COLUMN foo TEXT", OpAlter},
		{"TRUNCATE TABLE sessions", OpTruncate},
		{"UPDATE sessions SET status = ? WHERE id = ?", OpUpdate},
	}

	for _, tc := range ops {
		t.Run(string(tc.expected), func(t *testing.T) {
			var buf bytes.Buffer
			log := newTestLogger(&buf)

			result := DryRunExec(log, db, tc.query, "arg1")

			assert.True(t, result.Destructive)
			assert.Equal(t, tc.expected, result.Operation)
			assert.Contains(t, buf.String(), "[DRY-RUN]")
		})
	}
}
