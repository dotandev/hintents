// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

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
	EnvelopeXdr      string            `json:"envelope_xdr"`
	ResultMetaXdr    string            `json:"result_meta_xdr"`
	LedgerEntries    map[string]string `json:"ledger_entries,omitempty"`
	Timestamp        int64             `json:"timestamp,omitempty"`
	LedgerSequence   uint32            `json:"ledger_sequence,omitempty"`
	WasmPath         *string           `json:"wasm_path,omitempty"`
	MockArgs         *[]string         `json:"mock_args,omitempty"`
	Profile          bool              `json:"profile,omitempty"`
	ProtocolVersion  *uint32           `json:"protocol_version,omitempty"`
	MockBaseFee      *uint32           `json:"mock_base_fee,omitempty"`
	MockGasPrice     *uint64           `json:"mock_gas_price,omitempty"`
	MemoryLimit      *uint64           `json:"memory_limit,omitempty"`
	EnableCoverage   bool              `json:"enable_coverage,omitempty"`
	CoverageLCOVPath *string           `json:"coverage_lcov_path,omitempty"`

	//New: restorePreamble for state restoration operations
	RestorePreamble map[string]interface{} `json:"restore_preamble,omitempty"`

	AuthTraceOpts       *AuthTraceOptions      `json:"auth_trace_opts,omitempty"`
	CustomAuthCfg       map[string]interface{} `json:"custom_auth_config,omitempty"`
	ResourceCalibration *ResourceCalibration   `json:"resource_calibration,omitempty"`

	// SandboxNativeTokenCapStroops, when set, enforces a hard cap on the sum of native
	// (XLM) payment amounts in the envelope. Used in local/sandbox mode to simulate
	// realistic economic constraints during integration tests. Exceeding the cap
	// causes Run() to return an error before invoking the simulator binary.
	SandboxNativeTokenCapStroops *uint64 `json:"sandbox_native_token_cap_stroops,omitempty"`
}

type ResourceCalibration struct {
	SHA256Fixed      uint64 `json:"sha256_fixed"`
	SHA256PerByte    uint64 `json:"sha256_per_byte"`
	Keccak256Fixed   uint64 `json:"keccak256_fixed"`
	Keccak256PerByte uint64 `json:"keccak256_per_byte"`
	Ed25519Fixed     uint64 `json:"ed25519_fixed"`
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
	WasmInstruction          *string  `json:"wasm_instruction,omitempty"`
}

// BudgetUsage represents resource consumption during simulation
type BudgetUsage struct {
	CPUInstructions    uint64  `json:"cpu_instructions"`
	MemoryBytes        uint64  `json:"memory_bytes"`
	OperationsCount    int     `json:"operations_count"`
	CPULimit           uint64  `json:"cpu_limit"`
	MemoryLimit        uint64  `json:"memory_limit"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
}

type SimulationResponse struct {
	Status            string               `json:"status"` // "success" or "error"
	Error             string               `json:"error,omitempty"`
	ErrorCode         string               `json:"error_code,omitempty"`
	LCOVReport        string               `json:"lcov_report,omitempty"`
	LCOVReportPath    string               `json:"lcov_report_path,omitempty"`
	Events            []string             `json:"events,omitempty"`            // Raw event strings (backward compatibility)
	DiagnosticEvents  []DiagnosticEvent    `json:"diagnostic_events,omitempty"` // Structured diagnostic events
	Logs              []string             `json:"logs,omitempty"`              // Host debug logs
	Flamegraph        string               `json:"flamegraph,omitempty"`        // SVG flamegraph
	AuthTrace         *authtrace.AuthTrace `json:"auth_trace,omitempty"`
	BudgetUsage       *BudgetUsage         `json:"budget_usage,omitempty"` // Resource consumption metrics
	CategorizedEvents []CategorizedEvent   `json:"categorized_events,omitempty"`
	ProtocolVersion   *uint32              `json:"protocol_version,omitempty"` // Protocol version used
	StackTrace        *WasmStackTrace      `json:"stack_trace,omitempty"`      // Enhanced WASM stack trace on traps
	SourceLocation    string               `json:"source_location,omitempty"`
	WasmOffset        *uint64              `json:"wasm_offset,omitempty"`
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

// SourceLocation represents a precise position in Rust/WASM source code.
type SourceLocation struct {
	File      string `json:"file"`
	Line      uint   `json:"line"`
	Column    uint   `json:"column"`
	ColumnEnd *uint  `json:"column_end,omitempty"`
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

// WasmStackTrace holds a structured WASM call stack captured on a trap.
// This bypasses Soroban Host abstractions to expose the raw Wasmi call stack.
type WasmStackTrace struct {
	TrapKind       interface{}  `json:"trap_kind"`       // Categorised trap reason
	RawMessage     string       `json:"raw_message"`     // Original error string
	Frames         []StackFrame `json:"frames"`          // Ordered call stack frames
	SorobanWrapped bool         `json:"soroban_wrapped"` // Whether the error passed through Soroban Host
}

// StackFrame represents a single frame in a WASM call stack.
type StackFrame struct {
	Index      int     `json:"index"`                 // Position in the call stack (0 = trap site)
	FuncIndex  *uint32 `json:"func_index,omitempty"`  // WASM function index
	FuncName   *string `json:"func_name,omitempty"`   // Demangled function name
	WasmOffset *uint64 `json:"wasm_offset,omitempty"` // Byte offset in the WASM module
	Module     *string `json:"module,omitempty"`      // Module name from name section
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
