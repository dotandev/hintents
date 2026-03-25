// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import stdErrors "errors"

// ErstErrorCode is a unified error code type for RPC and Simulator boundaries.
type ErstErrorCode string

const (
	// General
	ErstUnknown          ErstErrorCode = "UNKNOWN"
	ErstValidationFailed ErstErrorCode = "VALIDATION_FAILED"
	ErstConfigFailed     ErstErrorCode = "CONFIG_ERROR"
	// RPC
	ErstRPCConnectionFailed   ErstErrorCode = "RPC_CONNECTION_FAILED"
	ErstRPCTimeout            ErstErrorCode = "RPC_TIMEOUT"
	ErstAllRPCFailed          ErstErrorCode = "RPC_ALL_FAILED"
	ErstRPCError              ErstErrorCode = "RPC_ERROR"
	ErstRPCResponseTooLarge   ErstErrorCode = "RPC_RESPONSE_TOO_LARGE"
	ErstRPCRequestTooLarge    ErstErrorCode = "RPC_REQUEST_TOO_LARGE"
	ErstRPCRateLimitExceeded  ErstErrorCode = "RPC_RATE_LIMIT_EXCEEDED"
	ErstRPCMarshalFailed      ErstErrorCode = "RPC_MARSHAL_FAILED"
	ErstRPCUnmarshalFailed    ErstErrorCode = "RPC_UNMARSHAL_FAILED"
	ErstTransactionNotFound   ErstErrorCode = "RPC_TRANSACTION_NOT_FOUND"
	// Simulator
	ErstSimulatorNotFound     ErstErrorCode = "SIM_BINARY_NOT_FOUND"
	ErstSimulationFailed      ErstErrorCode = "SIM_EXECUTION_FAILED"
	ErstSimCrash              ErstErrorCode = "SIM_PROCESS_CRASHED"
	ErstSimulationLogicError  ErstErrorCode = "SIM_LOGIC_ERROR"
	ErstSimMemoryLimitExceeded ErstErrorCode = "SIM_MEMORY_LIMIT_EXCEEDED"
	ErstSimProtoUnsup         ErstErrorCode = "SIM_PROTOCOL_UNSUPPORTED"
	// Ledger/Network
	ErstLedgerNotFound  ErstErrorCode = "LEDGER_NOT_FOUND"
	ErstLedgerArchived  ErstErrorCode = "LEDGER_ARCHIVED"
	ErstInvalidNetwork  ErstErrorCode = "INVALID_NETWORK"
	ErstNetworkNotFound ErstErrorCode = "NETWORK_NOT_FOUND"
	// Rate limiting
	ErstRateLimitExceeded ErstErrorCode = "RATE_LIMIT_EXCEEDED"
	// Auth
	ErstUnauthorized ErstErrorCode = "UNAUTHORIZED"
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

// Is allows errors.Is to match an ErstError against its corresponding sentinel
// errors by checking the errorCodeRegistry reverse mapping.
func (e *ErstError) Is(target error) bool {
	if code, ok := errorCodeRegistry[target]; ok {
		return code == e.Code
	}
	return false
}

// Registry mapping Go errors to ErstErrorCode
// Registry mapping Go errors (sentinels) to ErstErrorCode.
// This is the source of truth for both ClassifyError and ErstError.Is.
var errorCodeRegistry = map[error]ErstErrorCode{
	ErrTransactionNotFound:  ErstTransactionNotFound,
	ErrRPCConnectionFailed:  ErstRPCConnectionFailed,
	ErrRPCTimeout:           ErstRPCTimeout,
	ErrAllRPCFailed:         ErstAllRPCFailed,
	ErrSimulatorNotFound:    ErstSimulatorNotFound,
	ErrSimulationFailed:     ErstSimulationFailed,
	ErrSimCrash:             ErstSimCrash,
	ErrInvalidNetwork:       ErstInvalidNetwork,
	ErrMarshalFailed:        ErstRPCMarshalFailed,
	ErrUnmarshalFailed:      ErstRPCUnmarshalFailed,
	ErrSimulationLogicError: ErstSimulationLogicError,
	ErrRPCError:             ErstRPCError,
	ErrValidationFailed:     ErstValidationFailed,
	ErrProtocolUnsupported:  ErstSimProtoUnsup,
	ErrArgumentRequired:     ErstValidationFailed,
	ErrAuditLogInvalid:      ErstValidationFailed,
	ErrSessionNotFound:      ErstValidationFailed,
	ErrUnauthorized:         ErstUnauthorized,
	ErrLedgerNotFound:       ErstLedgerNotFound,
	ErrLedgerArchived:       ErstLedgerArchived,
	ErrRateLimitExceeded:    ErstRateLimitExceeded,
	ErrConfigFailed:         ErstConfigFailed,
	ErrNetworkNotFound:      ErstNetworkNotFound,
	ErrRPCResponseTooLarge:  ErstRPCResponseTooLarge,
	ErrRPCRequestTooLarge:   ErstRPCRequestTooLarge,
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
