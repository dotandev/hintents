// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
)

const maxRecentSearches = 20

// UIState records which output sections were visible and which search strings
// were used when a transaction was last viewed.
type UIState struct {
	ExpandedSections []string `json:"expanded_sections,omitempty"`
	RecentSearches   []string `json:"recent_searches,omitempty"`
}

// UIStateStore persists viewer preferences in ~/.erst/ui_state.db.
type UIStateStore struct {
	db *sql.DB
}

// NewUIStateStore opens or creates the viewer state database.
func NewUIStateStore() (*UIStateStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	return newUIStateStoreAt(filepath.Join(home, ".erst", "ui_state.db"))
}

func newUIStateStoreAt(dbPath string) (*UIStateStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open UI state database: %w", err)
	}
	s := &UIStateStore{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = os.Chmod(dbPath, 0600)
	return s, nil
}

func (s *UIStateStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS tx_ui_state (
			tx_hash    TEXT PRIMARY KEY,
			sections   TEXT NOT NULL DEFAULT '[]',
			updated_at TIMESTAMP NOT NULL
		);
		CREATE TABLE IF NOT EXISTS recent_searches (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			query       TEXT NOT NULL,
			searched_at TIMESTAMP NOT NULL
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_recent_searches_query
			ON recent_searches(query);
	`)
	if err != nil {
		return fmt.Errorf("failed to initialize UI state schema: %w", err)
	}
	return nil
}

// SaveSectionState records which output sections were visible for a transaction.
func (s *UIStateStore) SaveSectionState(ctx context.Context, txHash string, sections []string) error {
	b, err := json.Marshal(sections)
	if err != nil {
		return fmt.Errorf("failed to marshal sections: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tx_ui_state (tx_hash, sections, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(tx_hash) DO UPDATE SET
			sections   = excluded.sections,
			updated_at = excluded.updated_at
	`, txHash, string(b), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to save section state: %w", err)
	}
	return nil
}

// LoadSectionState returns the sections visible the last time this transaction
// was viewed, or nil if no state has been stored yet.
func (s *UIStateStore) LoadSectionState(ctx context.Context, txHash string) ([]string, error) {
	var raw string
	err := s.db.QueryRowContext(ctx,
		`SELECT sections FROM tx_ui_state WHERE tx_hash = ?`, txHash,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query section state: %w", err)
	}
	var sections []string
	if err := json.Unmarshal([]byte(raw), &sections); err != nil {
		return nil, fmt.Errorf("failed to unmarshal section state: %w", err)
	}
	return sections, nil
}

// AppendRecentSearch persists a search query, deduplicating by refreshing its
// timestamp, and trims entries beyond maxRecentSearches.
func (s *UIStateStore) AppendRecentSearch(ctx context.Context, query string) error {
	if query == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO recent_searches (query, searched_at)
		VALUES (?, ?)
		ON CONFLICT(query) DO UPDATE SET searched_at = excluded.searched_at
	`, query, now); err != nil {
		return fmt.Errorf("failed to append recent search: %w", err)
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM recent_searches
		WHERE id NOT IN (
			SELECT id FROM recent_searches
			ORDER BY searched_at DESC
			LIMIT ?
		)
	`, maxRecentSearches)
	return err
}

// RecentSearches returns up to limit search queries ordered newest first.
func (s *UIStateStore) RecentSearches(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > maxRecentSearches {
		limit = maxRecentSearches
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT query FROM recent_searches ORDER BY searched_at DESC, id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent searches: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Close releases the database connection.
func (s *UIStateStore) Close() error {
	return s.db.Close()
}
