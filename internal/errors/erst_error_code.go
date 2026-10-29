// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	stdErrors "errors"
	"strings"
)

// ErstErrorCode is a unified error code type for RPC and Simulator boundaries.
type ErstErrorCode string

const (
	// General
	ErstUnknown          ErstErrorCode = "UNKNOWN"
	ErstValidationFailed ErstErrorCode = "VALIDATION_FAILED"
	ErstConfigFailed     ErstErrorCode = "CONFIG_ERROR"
	// RPC
	ErstRPCConnectionFailed ErstErrorCode = "RPC_CONNECTION_FAILED"
	ErstRPCTimeout          ErstErrorCode = "RPC_TIMEOUT"
	ErstAllRPCFailed        ErstErrorCode = "ALL_RPC_FAILED"
	ErstRPCError            ErstErrorCode = "RPC_ERROR"
	// Simulator
	ErstSimulatorNotFound    ErstErrorCode = "SIMULATOR_NOT_FOUND"
	ErstSimulationFailed     ErstErrorCode = "SIMULATION_FAILED"
	ErstSimCrash             ErstErrorCode = "SIMULATOR_CRASH"
	ErstSimulationLogicError ErstErrorCode = "SIMULATION_LOGIC_ERROR"
	// Ledger/Network
	ErstLedgerNotFound  ErstErrorCode = "LEDGER_NOT_FOUND"
	ErstLedgerArchived  ErstErrorCode = "LEDGER_ARCHIVED"
	ErstInvalidNetwork  ErstErrorCode = "INVALID_NETWORK"
	ErstNetworkNotFound ErstErrorCode = "NETWORK_NOT_FOUND"
	// Rate limiting
	ErstRateLimitExceeded ErstErrorCode = "RATE_LIMIT_EXCEEDED"
	// Auth
	ErstTransactionNotFound ErstErrorCode = "TRANSACTION_NOT_FOUND"
	ErstUnauthorized        ErstErrorCode = "UNAUTHORIZED"
	// Simulator (typed error payloads)
	ErstSimTrap          ErstErrorCode = "SIMULATOR_TRAP"
	ErstSimTimeout       ErstErrorCode = "SIMULATOR_TIMEOUT"
	ErstSimOutOfMemory   ErstErrorCode = "SIMULATOR_OUT_OF_MEMORY"
	ErstSimDecodeFailed  ErstErrorCode = "SIMULATOR_DECODE_FAILED"
)

// ErstError wraps an error with a standardized code and preserves the original error string.
type ErstError struct {
	Code    ErstErrorCode
	Message string // human-readable message
	OrigErr error  // original error
}

func (e *ErstError) Error() string {
	if e.OrigErr != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.OrigErr.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *ErstError) Unwrap() error {
	return e.OrigErr
}

// Is allows errors.Is to match an ErstError against its corresponding sentinel
// errors. It checks both the codeToSentinel map (for Code* constants) and the
// errorCodeRegistry reverse mapping (for Erst* constants).
func (e *ErstError) Is(target error) bool {
	// Check the Code* -> sentinel mapping
	if sentinel, ok := codeToSentinel[e.Code]; ok {
		if target == sentinel {
			return true
		}
	}
	// Check the reverse of errorCodeRegistry: if the target sentinel
	// maps to the same ErstErrorCode as this error, it's a match.
	if code, ok := errorCodeRegistry[target]; ok {
		return code == e.Code
	}
	return false
}

// Registry mapping Go errors to ErstErrorCode
var errorCodeRegistry = map[error]ErstErrorCode{
	ErrTransactionNotFound:  ErstTransactionNotFound,
	ErrRPCConnectionFailed:  ErstRPCConnectionFailed,
	ErrRPCTimeout:           ErstRPCTimeout,
	ErrAllRPCFailed:         ErstAllRPCFailed,
	ErrSimulatorNotFound:    ErstSimulatorNotFound,
	ErrSimulationFailed:     ErstSimulationFailed,
	ErrSimCrash:             ErstSimCrash,
	ErrInvalidNetwork:       ErstInvalidNetwork,
	ErrMarshalFailed:        ErstValidationFailed,
	ErrUnmarshalFailed:      ErstValidationFailed,
	ErrSimulationLogicError: ErstSimulationLogicError,
	ErrRPCError:             ErstRPCError,
	ErrValidationFailed:     ErstValidationFailed,
	ErrProtocolUnsupported:  ErstValidationFailed,
	ErrArgumentRequired:     ErstValidationFailed,
	ErrAuditLogInvalid:      ErstValidationFailed,
	ErrSessionNotFound:      ErstValidationFailed,
	ErrUnauthorized:         ErstUnauthorized,
	ErrLedgerNotFound:       ErstLedgerNotFound,
	ErrLedgerArchived:       ErstLedgerArchived,
	ErrRateLimitExceeded:    ErstRateLimitExceeded,
	ErrConfigFailed:         ErstConfigFailed,
	ErrNetworkNotFound:      ErstNetworkNotFound,
}

// ClassifyError maps an error to an ErstError with a code and preserves the original error string.
func ClassifyError(err error) *ErstError {
	if err == nil {
		return nil
	}
	for sentinel, code := range errorCodeRegistry {
		if stdErrors.Is(err, sentinel) {
			return &ErstError{
				Code:    code,
				Message: err.Error(),
				OrigErr: err,
			}
		}
	}
	return &ErstError{
		Code:    ErstUnknown,
		Message: err.Error(),
		OrigErr: err,
	}
}

// SimulatorErrorPayload is the structured error information returned by the
// Rust simulator over IPC. The fields are optional because not every error
// kind carries all of them.
type SimulatorErrorPayload struct {
	// Code is the raw code string emitted by the simulator (e.g. "trap",
	// "timeout", "out_of_memory", "decode_failure").
	Code string
	// Message is the human-readable error text.
	Message string
	// Detail is optional extra context (e.g. trap origin or decode location).
	Detail string
}

// MapSimulatorError maps a simulator error payload to a typed ErstError. It
// recognizes trap, timeout, OOM and decode failures (by known code or by
// substring in the message) and falls back to ErstSimulationFailed for any
// unrecognized payload. Returns nil for an empty payload.
func MapSimulatorError(payload SimulatorErrorPayload) *ErstError {
	code := strings.ToLower(strings.TrimSpace(payload.Code))
	msg := strings.TrimSpace(payload.Message)
	if code == "" && msg == "" {
		return nil
	}

	mapped := mapSimulatorCode(code)
	if mapped == ErstUnknown {
		mapped = mapSimulatorMessage(msg)
	}
	if mapped == ErstUnknown {
		mapped = ErstSimulationFailed
	}

	message := msg
	if message == "" {
		message = code
	}
	if payload.Detail != "" {
		message = message + ": " + payload.Detail
	}

	return &ErstError{
		Code:    mapped,
		Message: message,
	}
}

// mapSimulatorCode maps a raw, lowercased simulator code to a typed code.
// It returns ErstUnknown when the code is not recognized.
func mapSimulatorCode(code string) ErstErrorCode {
	switch code {
	case "trap":
		return ErstSimTrap
	case "timeout":
		return ErstSimTimeout
	case "out_of_memory", "oom":
		return ErstSimOutOfMemory
	case "decode_failure", "decode_failed", "decode":
		return ErstSimDecodeFailed
	}
	return ErstUnknown
}

// mapSimulatorMessage derives a typed code from a free-form error message.
// It returns ErstUnknown when no known signature is present.
func mapSimulatorMessage(msg string) ErstErrorCode {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "out of memory"), strings.Contains(lower, "oom"),
		strings.Contains(lower, "memory limit exceeded"), strings.Contains(lower, "exceeded memory"):
		return ErstSimOutOfMemory
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "timed out"):
		return ErstSimTimeout
	case strings.Contains(lower, "trap"), strings.Contains(lower, "panic"):
		return ErstSimTrap
	case strings.Contains(lower, "decode"), strings.Contains(lower, "decoding failed"),
		strings.Contains(lower, "invalid response"):
		return ErstSimDecodeFailed
	}
	return ErstUnknown
}
