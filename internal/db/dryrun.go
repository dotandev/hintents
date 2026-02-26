// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// DestructiveOp represents a type of destructive SQL operation.
type DestructiveOp string

const (
	OpDelete   DestructiveOp = "DELETE"
	OpDrop     DestructiveOp = "DROP"
	OpAlter    DestructiveOp = "ALTER"
	OpTruncate DestructiveOp = "TRUNCATE"
	OpUpdate   DestructiveOp = "UPDATE"
	OpSafe     DestructiveOp = ""
)

// DryRunResult holds the outcome of a dry-run analysis.
type DryRunResult struct {
	Query       string
	Args        []interface{}
	Operation   DestructiveOp
	Destructive bool
}

// ClassifySQL returns the destructive operation type for a SQL statement.
// Returns OpSafe if the statement is not destructive.
func ClassifySQL(query string) DestructiveOp {
	normalized := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(normalized, "DELETE"):
		return OpDelete
	case strings.HasPrefix(normalized, "DROP"):
		return OpDrop
	case strings.HasPrefix(normalized, "ALTER"):
		return OpAlter
	case strings.HasPrefix(normalized, "TRUNCATE"):
		return OpTruncate
	case strings.HasPrefix(normalized, "UPDATE"):
		return OpUpdate
	default:
		return OpSafe
	}
}

// DryRunExec analyzes a SQL statement and logs a warning if it is destructive,
// without executing it. Returns a DryRunResult describing what would happen.
func DryRunExec(logger *slog.Logger, _ *sql.DB, query string, args ...interface{}) DryRunResult {
	op := ClassifySQL(query)
	result := DryRunResult{
		Query:       query,
		Args:        args,
		Operation:   op,
		Destructive: op != OpSafe,
	}

	if result.Destructive {
		logger.Warn("[DRY-RUN] destructive SQL detected",
			"operation", string(op),
			"query", query,
			"args", fmt.Sprintf("%v", args),
		)
	}

	return result
}
