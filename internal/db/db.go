// Copyright 2026 Erst Software
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "modernc.org/sqlite"
)

// Session represents a debugging session result
type Session struct {
	ID        int64     `json:"id"`
	TxHash    string    `json:"tx_hash"`
	Network   string    `json:"network"`
	Status    string    `json:"status"`
	ErrorMsg  string    `json:"error_msg"`
	Events    []string  `json:"events"`
	Logs      []string  `json:"logs"`
	Timestamp time.Time `json:"timestamp"`
}

// Store handles database operations
type Store struct {
	db *sql.DB
}

// migration represents a single ordered schema migration step.
// Each step must be idempotent and is applied inside a transaction.
type migration struct {
	version int
	query   string
}

// migrations is ordered by version ascending. The version of a
// migration is the schema version the database reaches after applying it.
var migrations = []migration{
	{
		version: 1,
		query: `
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tx_hash TEXT NOT NULL,
			network TEXT NOT NULL,
			status TEXT,
			error_msg TEXT,
			events TEXT,
			logs TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_tx_hash ON sessions(tx_hash);
		CREATE INDEX IF NOT EXISTS idx_sessions_error ON sessions(error_msg);
		`,
	},
}

// InitDB initializes the SQLite database
func InitDB() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}
	dir := filepath.Join(home, ".erst")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	dbPath := filepath.Join(dir, "sessions.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err = migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// migrate applies any pending schema migrations in order, tracking the
// current schema version via PRAGMA user_version. Each step runs inside a
// transaction so a failure leaves the database at its previous version.
func migrate(db *sql.DB) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	latest := migrations[len(migrations)-1].version
	if current > latest {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, latest)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("failed to apply schema migration %d: %w", m.version, err)
		}
	}

	return nil
}

// applyMigration executes a single migration step and bumps the schema
// version atomically within the same transaction.
func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.Exec(m.query); err != nil {
		return fmt.Errorf("failed to execute migration query: %w", err)
	}

	// PRAGMA user_version does not support bind parameters, so the value is
	// formatted directly. m.version is an int from our own constant table, so this is
	// safe from injection.
	if _, err = tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}
	return nil
}

// SaveSession persists a debugging session
func (s *Store) SaveSession(session *Session) error {
	eventsJSON, _ := json.Marshal(session.Events)
	logsJSON, _ := json.Marshal(session.Logs)

	query := `
	INSERT INTO sessions (tx_hash, network, status, error_msg, events, logs, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, session.TxHash, session.Network, session.Status, session.ErrorMsg, string(eventsJSON), string(logsJSON), time.Now())
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

// SearchParams defines the criteria for searching sessions
type SearchParams struct {
	TxHash     string
	ErrorRegex string
	EventRegex string
	Limit      int
}

// SearchSessions searches for sessions matching the params
func (s *Store) SearchSessions(params SearchParams) ([]Session, error) {
	query := "SELECT id, tx_hash, network, status, error_msg, events, logs, timestamp FROM sessions WHERE 1=1"
	args := []interface{}{}

	if params.TxHash != "" {
		query += " AND tx_hash = ?"
		args = append(args, params.TxHash)
	}

	query += " ORDER BY timestamp DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []Session
	var errorRe *regexp.Regexp
	var eventRe *regexp.Regexp

	if params.ErrorRegex != "" {
		errorRe, err = regexp.Compile(params.ErrorRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid error regex: %w", err)
		}
	}
	if params.EventRegex != "" {
		eventRe, err = regexp.Compile(params.EventRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid event regex: %w", err)
		}
	}

	count := 0
	for rows.Next() {
		// Optimization: if we have enough results, stop
		if params.Limit > 0 && count >= params.Limit {
			break
		}

		var sess Session
		var eventsRaw, logsRaw string
		var ts time.Time

		if err := rows.Scan(&sess.ID, &sess.TxHash, &sess.Network, &sess.Status, &sess.ErrorMsg, &eventsRaw, &logsRaw, &ts); err != nil {
			continue
		}
		sess.Timestamp = ts

		// Deserialize JSON
		_ = json.Unmarshal([]byte(eventsRaw), &sess.Events)
		_ = json.Unmarshal([]byte(logsRaw), &sess.Logs)

		// Filter
		if errorRe != nil {
			if !errorRe.MatchString(sess.ErrorMsg) {
				continue
			}
		}

		if eventRe != nil {
			found := false
			for _, e := range sess.Events {
				if eventRe.MatchString(e) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, sess)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}

	return results, nil
}
