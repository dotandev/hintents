// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

// Input collects all available signals about a transaction execution.
type Input struct {
	TxHash           string
	Network          string
	Status           string
	Error            string
	Events           []string
	Logs             []string
	DiagnosticEvents []simulator.DiagnosticEvent
	BudgetUsage      *simulator.BudgetUsage
}

// Summarize returns a single-paragraph plain-English explanation of why the
// transaction executed as it did.  For failed transactions, heuristic rules are
// applied in priority order to identify the most probable root cause.
func Summarize(in Input) string {
	if in.Status == "success" {
		return fmt.Sprintf(
			"Transaction %s executed successfully on %s with no detected errors.",
			shortHash(in.TxHash), in.Network,
		)
	}

	combined := strings.Join(append(in.Events, in.Logs...), " ") + " " + in.Error

	if reason := checkAuthFailure(in, combined); reason != "" {
		return reason
	}
	if reason := checkBudgetExceeded(in, combined); reason != "" {
		return reason
	}
	if reason := checkInsufficientBalance(in, combined); reason != "" {
		return reason
	}
	if reason := checkMissingEntry(in, combined); reason != "" {
		return reason
	}
	if reason := checkWasmTrap(in, combined); reason != "" {
		return reason
	}

	if in.Error != "" {
		return fmt.Sprintf(
			"Transaction %s failed on %s. The simulator reported: %s.",
			shortHash(in.TxHash), in.Network, sanitize(in.Error),
		)
	}
	return fmt.Sprintf(
		"Transaction %s failed on %s. No diagnostic information was produced; inspect the raw XDR for details.",
		shortHash(in.TxHash), in.Network,
	)
}

// checkAuthFailure detects authorization-related failures, including cross-contract
// scenarios where one contract invoked another that lacked the required authorization.
func checkAuthFailure(in Input, combined string) string {
	lc := strings.ToLower(combined)
	if !strings.Contains(lc, "error(auth,") &&
		!strings.Contains(lc, "not authorized") &&
		!strings.Contains(lc, "require_auth") &&
		!strings.Contains(lc, "auth failed") &&
		!strings.Contains(lc, "missing authorization") &&
		!strings.Contains(lc, "invalidaction") &&
		!strings.Contains(lc, "notauthorized") {
		return ""
	}

	callerID, calleeID := extractCallerCallee(in.DiagnosticEvents)
	switch {
	case callerID != "" && calleeID != "":
		return fmt.Sprintf(
			"Transaction %s failed on %s because contract %s invoked contract %s which lacked the required authorization.",
			shortHash(in.TxHash), in.Network, callerID, calleeID,
		)
	case calleeID != "":
		return fmt.Sprintf(
			"Transaction %s failed on %s because contract %s could not satisfy an authorization check.",
			shortHash(in.TxHash), in.Network, calleeID,
		)
	default:
		return fmt.Sprintf(
			"Transaction %s failed on %s due to an authorization failure; a required signature or auth entry was absent or invalid.",
			shortHash(in.TxHash), in.Network,
		)
	}
}

// checkBudgetExceeded detects CPU or memory budget overruns.
func checkBudgetExceeded(in Input, combined string) string {
	lc := strings.ToLower(combined)
	cpuOver := strings.Contains(lc, "cpulimitexceeded") ||
		strings.Contains(lc, "cpu limit exceeded") ||
		strings.Contains(lc, "error(budget, cpu")
	memOver := strings.Contains(lc, "memlimitexceeded") ||
		strings.Contains(lc, "memory limit exceeded") ||
		strings.Contains(lc, "error(budget, mem")

	if in.BudgetUsage != nil {
		if in.BudgetUsage.CPUUsagePercent >= 100 {
			cpuOver = true
		}
		if in.BudgetUsage.MemoryUsagePercent >= 100 {
			memOver = true
		}
	}

	switch {
	case cpuOver && memOver:
		return fmt.Sprintf(
			"Transaction %s failed on %s because it exhausted both the CPU instruction budget and the memory allocation budget during contract execution.",
			shortHash(in.TxHash), in.Network,
		)
	case cpuOver:
		return fmt.Sprintf(
			"Transaction %s failed on %s because the contract execution exceeded the Soroban CPU instruction budget.",
			shortHash(in.TxHash), in.Network,
		)
	case memOver:
		return fmt.Sprintf(
			"Transaction %s failed on %s because the contract execution exceeded the Soroban memory allocation budget.",
			shortHash(in.TxHash), in.Network,
		)
	}
	return ""
}

// checkInsufficientBalance detects balance or token-transfer failures.
func checkInsufficientBalance(in Input, combined string) string {
	lc := strings.ToLower(combined)
	if strings.Contains(lc, "insufficient_balance") ||
		strings.Contains(lc, "insufficient balance") ||
		strings.Contains(lc, "balance is not sufficient") {
		return fmt.Sprintf(
			"Transaction %s failed on %s because an account or contract held insufficient balance to cover the requested transfer.",
			shortHash(in.TxHash), in.Network,
		)
	}
	return ""
}

// checkMissingEntry detects storage look-up failures for absent ledger entries.
func checkMissingEntry(in Input, combined string) string {
	lc := strings.ToLower(combined)
	if strings.Contains(lc, "missingvalue") ||
		strings.Contains(lc, "missing value") ||
		strings.Contains(lc, "error(storage,") ||
		strings.Contains(lc, "not found") {
		return fmt.Sprintf(
			"Transaction %s failed on %s because a required ledger entry or contract storage key was not present at execution time.",
			shortHash(in.TxHash), in.Network,
		)
	}
	return ""
}

// checkWasmTrap detects low-level WASM trap or unhandled panic conditions.
func checkWasmTrap(in Input, combined string) string {
	lc := strings.ToLower(combined)
	if strings.Contains(lc, "wasm trap") ||
		strings.Contains(lc, "unreachable") ||
		strings.Contains(lc, "contract_invocation_failed") {
		return fmt.Sprintf(
			"Transaction %s failed on %s due to a fatal WASM trap inside the contract, typically caused by an unhandled panic or an explicit unreachable instruction.",
			shortHash(in.TxHash), in.Network,
		)
	}
	return ""
}

// extractCallerCallee returns the last two distinct contract IDs encountered in
// diagnostic events, treating the earlier one as the caller and the later one as
// the callee that triggered the failure.
func extractCallerCallee(events []simulator.DiagnosticEvent) (caller, callee string) {
	seen := make([]string, 0, 4)
	dedup := make(map[string]struct{})
	for _, e := range events {
		if e.ContractID == nil {
			continue
		}
		id := *e.ContractID
		if _, ok := dedup[id]; !ok {
			dedup[id] = struct{}{}
			seen = append(seen, id)
		}
	}
	switch len(seen) {
	case 0:
		return "", ""
	case 1:
		return "", seen[0]
	default:
		return seen[len(seen)-2], seen[len(seen)-1]
	}
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:6] + "..." + hash[len(hash)-6:]
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
