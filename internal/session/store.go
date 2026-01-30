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

package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/simulator"
	_ "modernc.org/sqlite"
)

const (
	// SchemaVersion tracks the database schema version for migrations
	SchemaVersion = 1

	// DefaultTTL is the default time-to-live for sessions (30 days)
	DefaultTTL = 30 * 24 * time.Hour

	// DefaultMaxSessions is the maximum number of sessions to keep
	DefaultMaxSessions = 1000
)

// SessionData represents the complete state of a debug session
type SessionData struct {
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	LastAccessAt  time.Time `json:"last_access_at"`
	Status        string    `json:"status"` // active, saved, resumed, expired
	Network       string    `json:"network"`
	HorizonURL    string    `json:"horizon_url"`
	TxHash        string    `json:"tx_hash"`
	EnvelopeXdr   string    `json:"envelope_xdr"`
	ResultXdr     string    `json:"result_xdr"`
	ResultMetaXdr string    `json:"result_meta_xdr"`

	// Simulator I/O
	SimRequestJSON  string `json:"sim_request_json"`  // JSON sent to erst-sim
	SimResponseJSON string `json:"sim_response_json"` // JSON received from erst-sim

	// Metadata
	ErstVersion   string `json:"erst_version"`
	SchemaVersion int    `json:"schema_version"`
}

// Store manages session persistence in SQLite
type Store struct {
	db *sql.DB
}

// NewStore creates or opens the session database
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	erstDir := filepath.Join(homeDir, ".erst")
	if err := os.MkdirAll(erstDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .erst directory: %w", err)
	}

	dbPath := filepath.Join(erstDir, "sessions.db")

	// Open SQLite database
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Set file permissions to 600 (read/write for owner only)
	if err := os.Chmod(dbPath, 0600); err != nil {
		logger.Logger.Warn("Failed to set database permissions", "error", err)
	}

	return store, nil
}

// initSchema creates the sessions table if it doesn't exist
func (s *Store) initSchema() error {
	query := `
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
	`

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Save persists a session to the database
func (s *Store) Save(ctx context.Context, data *SessionData) error {
	if data.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	now := time.Now()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}
	data.LastAccessAt = now
	data.SchemaVersion = SchemaVersion

	query := `
	INSERT INTO sessions (
		id, created_at, last_access_at, status, network, horizon_url, tx_hash,
		envelope_xdr, result_xdr, result_meta_xdr,
		sim_request_json, sim_response_json, erst_version, schema_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		last_access_at = excluded.last_access_at,
		status = excluded.status,
		network = excluded.network,
		horizon_url = excluded.horizon_url,
		tx_hash = excluded.tx_hash,
		envelope_xdr = excluded.envelope_xdr,
		result_xdr = excluded.result_xdr,
		result_meta_xdr = excluded.result_meta_xdr,
		sim_request_json = excluded.sim_request_json,
		sim_response_json = excluded.sim_response_json,
		erst_version = excluded.erst_version,
		schema_version = excluded.schema_version
	`

	_, err := s.db.ExecContext(ctx, query,
		data.ID, data.CreatedAt, data.LastAccessAt, data.Status,
		data.Network, data.HorizonURL, data.TxHash,
		data.EnvelopeXdr, data.ResultXdr, data.ResultMetaXdr,
		data.SimRequestJSON, data.SimResponseJSON,
		data.ErstVersion, data.SchemaVersion,
	)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	logger.Logger.Debug("Session saved", "id", data.ID, "tx_hash", data.TxHash)
	return nil
}

// Load retrieves a session by ID
func (s *Store) Load(ctx context.Context, sessionID string) (*SessionData, error) {
	query := `
	SELECT id, created_at, last_access_at, status, network, horizon_url, tx_hash,
	       envelope_xdr, result_xdr, result_meta_xdr,
	       sim_request_json, sim_response_json, erst_version, schema_version
	FROM sessions
	WHERE id = ?
	`

	var data SessionData
	var createdAt, lastAccessAt string

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&data.ID, &createdAt, &lastAccessAt, &data.Status,
		&data.Network, &data.HorizonURL, &data.TxHash,
		&data.EnvelopeXdr, &data.ResultXdr, &data.ResultMetaXdr,
		&data.SimRequestJSON, &data.SimResponseJSON,
		&data.ErstVersion, &data.SchemaVersion,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Parse timestamps
	if data.CreatedAt, err = time.Parse(time.RFC3339, createdAt); err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}
	if data.LastAccessAt, err = time.Parse(time.RFC3339, lastAccessAt); err != nil {
		return nil, fmt.Errorf("failed to parse last_access_at: %w", err)
	}

	// Update last_access_at on load
	data.LastAccessAt = time.Now()
	updateQuery := `UPDATE sessions SET last_access_at = ? WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, updateQuery, data.LastAccessAt, sessionID); err != nil {
		logger.Logger.Warn("Failed to update last_access_at", "error", err)
	}

	return &data, nil
}

// List returns recent sessions, ordered by last_access_at descending
func (s *Store) List(ctx context.Context, limit int) ([]*SessionData, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
	SELECT id, created_at, last_access_at, status, network, horizon_url, tx_hash,
	       envelope_xdr, result_xdr, result_meta_xdr,
	       sim_request_json, sim_response_json, erst_version, schema_version
	FROM sessions
	ORDER BY last_access_at DESC
	LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionData
	for rows.Next() {
		var data SessionData
		var createdAt, lastAccessAt string

		err := rows.Scan(
			&data.ID, &createdAt, &lastAccessAt, &data.Status,
			&data.Network, &data.HorizonURL, &data.TxHash,
			&data.EnvelopeXdr, &data.ResultXdr, &data.ResultMetaXdr,
			&data.SimRequestJSON, &data.SimResponseJSON,
			&data.ErstVersion, &data.SchemaVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// Parse timestamps
		if data.CreatedAt, err = time.Parse(time.RFC3339, createdAt); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		if data.LastAccessAt, err = time.Parse(time.RFC3339, lastAccessAt); err != nil {
			return nil, fmt.Errorf("failed to parse last_access_at: %w", err)
		}

		sessions = append(sessions, &data)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// Delete removes a session by ID
func (s *Store) Delete(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	logger.Logger.Debug("Session deleted", "id", sessionID)
	return nil
}

// Cleanup removes expired sessions and enforces max session limit
func (s *Store) Cleanup(ctx context.Context, ttl time.Duration, maxSessions int) error {
	now := time.Now()
	cutoff := now.Add(-ttl)

	// Delete expired sessions
	deleteExpired := `DELETE FROM sessions WHERE last_access_at < ?`
	result, err := s.db.ExecContext(ctx, deleteExpired, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	expiredCount, _ := result.RowsAffected()
	if expiredCount > 0 {
		logger.Logger.Debug("Cleaned up expired sessions", "count", expiredCount)
	}

	// Enforce max sessions limit
	if maxSessions > 0 {
		countQuery := `SELECT COUNT(*) FROM sessions`
		var count int
		if err := s.db.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
			return fmt.Errorf("failed to count sessions: %w", err)
		}

		if count > maxSessions {
			excess := count - maxSessions
			deleteOldest := `
				DELETE FROM sessions
				WHERE id IN (
					SELECT id FROM sessions
					ORDER BY last_access_at ASC
					LIMIT ?
				)
			`
			result, err := s.db.ExecContext(ctx, deleteOldest, excess)
			if err != nil {
				return fmt.Errorf("failed to delete oldest sessions: %w", err)
			}

			deletedCount, _ := result.RowsAffected()
			if deletedCount > 0 {
				logger.Logger.Debug("Cleaned up excess sessions", "count", deletedCount)
			}
		}
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// GenerateID creates a new session ID from transaction hash and timestamp
func GenerateID(txHash string) string {
	if txHash != "" {
		// Use first 8 chars of hash + timestamp for readability
		shortHash := txHash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}
		return fmt.Sprintf("%s-%d", shortHash, time.Now().Unix())
	}
	// Fallback to timestamp-based ID
	return fmt.Sprintf("session-%d", time.Now().Unix())
}

// ToSimulationRequest converts stored JSON back to SimulationRequest
func (s *SessionData) ToSimulationRequest() (*simulator.SimulationRequest, error) {
	if s.SimRequestJSON == "" {
		return nil, fmt.Errorf("no simulation request data stored")
	}

	var req simulator.SimulationRequest
	if err := json.Unmarshal([]byte(s.SimRequestJSON), &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal simulation request: %w", err)
	}

	return &req, nil
}

// ToSimulationResponse converts stored JSON back to SimulationResponse
func (s *SessionData) ToSimulationResponse() (*simulator.SimulationResponse, error) {
	if s.SimResponseJSON == "" {
		return nil, fmt.Errorf("no simulation response data stored")
	}

	var resp simulator.SimulationResponse
	if err := json.Unmarshal([]byte(s.SimResponseJSON), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal simulation response: %w", err)
	}

	return &resp, nil
}
