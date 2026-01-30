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

package simulator

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/dotandev/hintents/internal/authtrace"
	_ "modernc.org/sqlite"
)

// SimulationRequest is the JSON object passed to the Rust binary via Stdin
type SimulationRequest struct {
	EnvelopeXdr     string            `json:"envelope_xdr"`
	ResultMetaXdr   string            `json:"result_meta_xdr"`
	LedgerEntries   map[string]string `json:"ledger_entries,omitempty"`
	Timestamp       int64             `json:"timestamp,omitempty"`
	LedgerSequence  uint32            `json:"ledger_sequence,omitempty"`
	WasmPath        *string           `json:"wasm_path,omitempty"`
	MockArgs        *[]string         `json:"mock_args,omitempty"`
	Profile         bool              `json:"profile,omitempty"`
	ProtocolVersion *uint32           `json:"protocol_version,omitempty"`

	AuthTraceOpts *AuthTraceOptions      `json:"auth_trace_opts,omitempty"`
	CustomAuthCfg map[string]interface{} `json:"custom_auth_config,omitempty"`
}

type AuthTraceOptions struct {
	Enabled              bool `json:"enabled"`
	TraceCustomContracts bool `json:"trace_custom_contracts"`
	CaptureSigDetails    bool `json:"capture_sig_details"`
	MaxEventDepth        int  `json:"max_event_depth,omitempty"`
}

// DiagnosticEvent represents a structured diagnostic event from the simulator
type DiagnosticEvent struct {
	EventType                string   `json:"event_type"` // "contract", "system", "diagnostic"
	ContractID               *string  `json:"contract_id,omitempty"`
	Topics                   []string `json:"topics"`
	Data                     string   `json:"data"`
	InSuccessfulContractCall bool     `json:"in_successful_contract_call"`
}

// BudgetUsage represents resource consumption during simulation
type BudgetUsage struct {
	CPUInstructions uint64 `json:"cpu_instructions"`
	MemoryBytes     uint64 `json:"memory_bytes"`
	OperationsCount int    `json:"operations_count"`
}

type SimulationResponse struct {
	Status            string               `json:"status"` // "success" or "error"
	Error             string               `json:"error,omitempty"`
	Events            []string             `json:"events,omitempty"`            // Raw event strings (backward compatibility)
	DiagnosticEvents  []DiagnosticEvent    `json:"diagnostic_events,omitempty"` // Structured diagnostic events
	Logs              []string             `json:"logs,omitempty"`              // Host debug logs
	Flamegraph        string               `json:"flamegraph,omitempty"`        // SVG flamegraph
	AuthTrace         *authtrace.AuthTrace `json:"auth_trace,omitempty"`
	BudgetUsage       *BudgetUsage         `json:"budget_usage,omitempty"` // Resource consumption metrics
	CategorizedEvents []CategorizedEvent   `json:"categorized_events,omitempty"`
	ProtocolVersion   *uint32              `json:"protocol_version,omitempty"` // Protocol version used
}

type CategorizedEvent struct {
	EventType  string   `json:"event_type"`
	ContractID *string  `json:"contract_id,omitempty"`
	Topics     []string `json:"topics"`
	Data       string   `json:"data"`
}

type SecurityViolation struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Contract    string                 `json:"contract"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// Session represents a stored simulation result
type Session struct {
	ID        int64     `json:"id"`
	TxHash    string    `json:"tx_hash"`
	Network   string    `json:"network"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
	Events    string    `json:"events,omitempty"` // JSON string
	Logs      string    `json:"logs,omitempty"`   // JSON string
}

type DB struct {
	conn *sql.DB
}

func OpenDB() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".erst", "sessions.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.init(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tx_hash TEXT NOT NULL,
		network TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		error TEXT,
		events TEXT,
		logs TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_tx_hash ON sessions(tx_hash);
	CREATE INDEX IF NOT EXISTS idx_error ON sessions(error);
	`
	_, err := db.conn.Exec(query)
	return err
}

func (db *DB) SaveSession(s *Session) error {
	query := "INSERT INTO sessions (tx_hash, network, timestamp, error, events, logs) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := db.conn.Exec(query, s.TxHash, s.Network, s.Timestamp, s.Error, s.Events, s.Logs)
	return err
}

type SearchFilters struct {
	Error    string
	Event    string
	Contract string
	UseRegex bool
}

func (db *DB) SearchSessions(filters SearchFilters) ([]Session, error) {
	query := "SELECT id, tx_hash, network, timestamp, error, events, logs FROM sessions WHERE 1=1"
	var args []interface{}

	if filters.Error != "" {
		if filters.UseRegex {
			query += " AND error REGEXP ?"
		} else {
			query += " AND error LIKE ?"
			filters.Error = "%" + filters.Error + "%"
		}
		args = append(args, filters.Error)
	}

	if filters.Event != "" {
		if filters.UseRegex {
			query += " AND events REGEXP ?"
		} else {
			query += " AND events LIKE ?"
			filters.Event = "%" + filters.Event + "%"
		}
		args = append(args, filters.Event)
	}

	if filters.Contract != "" {
		if filters.UseRegex {
			query += " AND (events REGEXP ? OR logs REGEXP ?)"
			args = append(args, filters.Contract, filters.Contract)
		} else {
			query += " AND (events LIKE ? OR logs LIKE ?)"
			match := "%" + filters.Contract + "%"
			args = append(args, match, match)
		}
	}

	query += " ORDER BY timestamp DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.TxHash, &s.Network, &s.Timestamp, &s.Error, &s.Events, &s.Logs)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}
