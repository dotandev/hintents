// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"fmt"
)

// formatBytes converts bytes to a human-readable string (e.g., "1.5 MB")
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// New is a proxy to the standard errors.New
func New(text string) error {
	return errors.New(text)
}

// Is is a proxy to the standard errors.Is
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As is a proxy to the standard errors.As
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Sentinel errors for comparison with errors.Is
var (
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrRPCConnectionFailed  = errors.New("RPC connection failed")
	ErrRPCTimeout           = errors.New("RPC request timed out")
	ErrAllRPCFailed         = errors.New("all RPC endpoints failed")
	ErrSimulatorNotFound    = errors.New("simulator binary not found")
	ErrSimulationFailed     = errors.New("simulation execution failed")
	ErrSimCrash             = errors.New("simulator process crashed")
	ErrInvalidNetwork       = errors.New("invalid network")
	ErrMarshalFailed        = errors.New("failed to marshal request")
	ErrUnmarshalFailed      = errors.New("failed to unmarshal response")
	ErrSimulationLogicError = errors.New("simulation logic error")
	ErrRPCError             = errors.New("RPC server returned an error")
	ErrValidationFailed     = errors.New("validation failed")
	ErrProtocolUnsupported  = errors.New("unsupported protocol version")
	ErrArgumentRequired     = errors.New("required argument missing")
	ErrAuditLogInvalid      = errors.New("audit log verification failed")
	ErrSessionNotFound      = errors.New("session not found")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrLedgerNotFound       = errors.New("ledger not found")
	ErrLedgerArchived       = errors.New("ledger has been archived")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
	ErrRPCResponseTooLarge  = errors.New("RPC response too large")
	ErrRPCRequestTooLarge   = errors.New("RPC request payload too large")
	ErrConfigFailed         = errors.New("configuration error")
	ErrNetworkNotFound      = errors.New("network not found")
	ErrMissingLedgerKey     = errors.New("missing ledger key in footprint")
	ErrWasmInvalid          = errors.New("invalid WASM file")
	ErrSpecNotFound         = errors.New("contract spec not found")
	ErrShellExit            = errors.New("exit")
)

type LedgerNotFoundError struct {
	Sequence uint32
	Message  string
}

func (e *LedgerNotFoundError) Error() string {
	return e.Message
}

func (e *LedgerNotFoundError) Is(target error) bool {
	return target == ErrLedgerNotFound
}

type LedgerArchivedError struct {
	Sequence uint32
	Message  string
}

func (e *LedgerArchivedError) Error() string {
	return e.Message
}

func (e *LedgerArchivedError) Is(target error) bool {
	return target == ErrLedgerArchived
}

type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimitExceeded
}

// ResponseTooLargeError indicates the Soroban RPC response exceeded server limits.
type ResponseTooLargeError struct {
	URL     string
	Message string
}

func (e *ResponseTooLargeError) Error() string {
	return e.Message
}

func (e *ResponseTooLargeError) Is(target error) bool {
	return target == ErrRPCResponseTooLarge
}

// MissingLedgerKeyError is returned when partial simulation halts because
// a required ledger key is absent from the provided state snapshot.
type MissingLedgerKeyError struct {
	Key string
}

func (e *MissingLedgerKeyError) Error() string {
	return fmt.Sprintf("%v: %s", ErrMissingLedgerKey, e.Key)
}

func (e *MissingLedgerKeyError) Is(target error) bool {
	return target == ErrMissingLedgerKey
}

// Wrap functions for consistent error wrapping
func WrapTransactionNotFound(err error) error {
	return &ErstError{
		Code:    ErstLedgerNotFound,
		Message: "transaction not found",
		OrigErr: err,
	}
}

func WrapRPCConnectionFailed(err error) error {
	return &ErstError{
		Code:    ErstRPCConnectionFailed,
		Message: "RPC connection failed",
		OrigErr: err,
	}
}

func WrapSimulatorNotFound(msg string) error {
	return &ErstError{
		Code:    ErstSimulatorNotFound,
		Message: msg,
	}
}

func WrapSimulationFailed(err error, stderr string) error {
	return &ErstError{
		Code:    ErstSimulationFailed,
		Message: stderr,
		OrigErr: err,
	}
}

func WrapInvalidNetwork(network string) error {
	return &ErstError{
		Code:    ErstInvalidNetwork,
		Message: network + ". Must be one of: testnet, mainnet, futurenet",
	}
}

func WrapMarshalFailed(err error) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: "failed to marshal request",
		OrigErr: err,
	}
}

func WrapUnmarshalFailed(err error, output string) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: output,
		OrigErr: err,
	}
}

func WrapSimulationLogicError(msg string) error {
	return &ErstError{
		Code:    ErstSimulationLogicError,
		Message: msg,
	}
}

func WrapRPCTimeout(err error) error {
	return &ErstError{
		Code:    ErstRPCTimeout,
		Message: "RPC request timed out",
		OrigErr: err,
	}
}

func WrapAllRPCFailed() error {
	return &ErstError{
		Code:    ErstAllRPCFailed,
		Message: "all RPC endpoints failed",
	}
}

func WrapRPCError(url string, msg string, code int) error {
	return &ErstError{
		Code:    ErstRPCError,
		Message: fmt.Sprintf("from %s: %s (code %d)", url, msg, code),
	}
}

func WrapSimCrash(err error, stderr string) error {
	msg := stderr
	if msg == "" && err != nil {
		msg = err.Error()
	}
	return &ErstError{
		Code:    ErstSimCrash,
		Message: msg,
		OrigErr: err,
	}
}

func WrapValidationError(msg string) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: msg,
	}
}

func WrapProtocolUnsupported(version uint32) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: fmt.Sprintf("unsupported protocol version: %d", version),
	}
}

func WrapCliArgumentRequired(arg string) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: "--" + arg,
	}
}

func WrapAuditLogInvalid(msg string) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: msg,
	}
}

func WrapSessionNotFound(sessionID string) error {
	return &ErstError{
		Code:    ErstValidationFailed,
		Message: sessionID,
	}
}

func WrapUnauthorized(msg string) error {
	if msg != "" {
		return &ErstError{
			Code:    ErstUnauthorized,
			Message: msg,
		}
	}
	return &ErstError{
		Code:    ErstUnauthorized,
		Message: "unauthorized",
	}
}

func WrapLedgerNotFound(sequence uint32) error {
	return &ErstError{
		Code:    ErstLedgerNotFound,
		Message: fmt.Sprintf("ledger %d not found (may be archived or not yet created)", sequence),
	}
}

func WrapLedgerArchived(sequence uint32) error {
	return &ErstError{
		Code:    ErstLedgerArchived,
		Message: fmt.Sprintf("ledger %d has been archived and is no longer available", sequence),
	}
}

func WrapRateLimitExceeded() error {
	return &ErstError{
		Code:    ErstRateLimitExceeded,
		Message: "rate limit exceeded, please try again later",
	}
}

func WrapConfigError(msg string, err error) error {
	return &ErstError{
		Code:    ErstConfigFailed,
		Message: msg,
	}
}

func WrapNetworkNotFound(network string) error {
	return &ErstError{
		Code:    ErstNetworkNotFound,
		Message: network,
	}
}

func WrapWasmInvalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrWasmInvalid, msg)
}

func WrapSpecNotFound() error {
	return fmt.Errorf("%w: no contractspecv0 section found; is this a compiled Soroban contract?", ErrSpecNotFound)
}

// WrapRPCResponseTooLarge wraps an HTTP 413 response into a readable message
// explaining that the Soroban RPC response exceeded the server's size limit.
func WrapRPCResponseTooLarge(url string) error {
	return &ResponseTooLargeError{
		URL: url,
		Message: fmt.Sprintf(
			"%v: the response from %s exceeded the server's maximum allowed size; "+
				"reduce the request scope (e.g. fewer ledger keys) or contact the RPC provider"+
				" to increase the Soroban RPC response limit",
			ErrRPCResponseTooLarge, url),
	}
}

// WrapRPCRequestTooLarge returns an error when the JSON payload exceeds
// the maximum allowed size (10MB) to prevent network submission.
func WrapRPCRequestTooLarge(sizeBytes int64, maxSizeBytes int64) error {
	return fmt.Errorf(
		"%v: request payload size (%s) exceeds maximum allowed size (%s). "+
			"This payload is too large to submit to the network. "+
			"Consider reducing the amount of data being sent (e.g., fewer ledger entries, "+
			"smaller transaction envelopes, or breaking the request into smaller chunks)",
		ErrRPCRequestTooLarge,
		formatBytes(sizeBytes),
		formatBytes(maxSizeBytes),
	)
}

func WrapMissingLedgerKey(key string) error {
	return &MissingLedgerKeyError{Key: key}
}

const (
	// RPC origin
	CodeRPCConnectionFailed  ErstErrorCode = "RPC_CONNECTION_FAILED"
	CodeRPCTimeout           ErstErrorCode = "RPC_TIMEOUT"
	CodeRPCAllFailed         ErstErrorCode = "RPC_ALL_ENDPOINTS_FAILED"
	CodeRPCError             ErstErrorCode = "RPC_SERVER_ERROR"
	CodeRPCResponseTooLarge  ErstErrorCode = "RPC_RESPONSE_TOO_LARGE"
	CodeRPCRequestTooLarge   ErstErrorCode = "RPC_REQUEST_TOO_LARGE"
	CodeRPCRateLimitExceeded ErstErrorCode = "RPC_RATE_LIMIT_EXCEEDED"
	CodeRPCMarshalFailed     ErstErrorCode = "RPC_MARSHAL_FAILED"
	CodeRPCUnmarshalFailed   ErstErrorCode = "RPC_UNMARSHAL_FAILED"
	CodeTransactionNotFound  ErstErrorCode = "RPC_TRANSACTION_NOT_FOUND"
	CodeLedgerNotFound       ErstErrorCode = "RPC_LEDGER_NOT_FOUND"
	CodeLedgerArchived       ErstErrorCode = "RPC_LEDGER_ARCHIVED"

	// Simulator origin
	CodeSimNotFound            ErstErrorCode = "SIM_BINARY_NOT_FOUND"
	CodeSimCrash               ErstErrorCode = "SIM_PROCESS_CRASHED"
	CodeSimExecFailed          ErstErrorCode = "SIM_EXECUTION_FAILED"
	CodeSimMemoryLimitExceeded ErstErrorCode = "ERR_MEMORY_LIMIT_EXCEEDED"
	CodeSimLogicError          ErstErrorCode = "SIM_LOGIC_ERROR"
	CodeSimProtoUnsup          ErstErrorCode = "SIM_PROTOCOL_UNSUPPORTED"

	// Shared / general
	CodeValidationFailed ErstErrorCode = "VALIDATION_FAILED"
	CodeConfigFailed     ErstErrorCode = "CONFIG_ERROR"
	CodeUnknown          ErstErrorCode = "UNKNOWN"
)

// codeToSentinel maps each ErstErrorCode to its corresponding sentinel error
// so that errors.Is(erstErr, sentinel) works reliably.
var codeToSentinel = map[ErstErrorCode]error{
	CodeRPCConnectionFailed:    ErrRPCConnectionFailed,
	CodeRPCTimeout:             ErrRPCTimeout,
	CodeRPCAllFailed:           ErrAllRPCFailed,
	CodeRPCError:               ErrRPCError,
	CodeRPCResponseTooLarge:    ErrRPCResponseTooLarge,
	CodeRPCRequestTooLarge:     ErrRPCRequestTooLarge,
	CodeRPCRateLimitExceeded:   ErrRateLimitExceeded,
	CodeRPCMarshalFailed:       ErrMarshalFailed,
	CodeRPCUnmarshalFailed:     ErrUnmarshalFailed,
	CodeTransactionNotFound:    ErrTransactionNotFound,
	CodeLedgerNotFound:         ErrLedgerNotFound,
	CodeLedgerArchived:         ErrLedgerArchived,
	CodeSimNotFound:            ErrSimulatorNotFound,
	CodeSimCrash:               ErrSimCrash,
	CodeSimExecFailed:          ErrSimulationFailed,
	CodeSimMemoryLimitExceeded: ErrSimulationFailed,
	CodeSimLogicError:          ErrSimulationLogicError,
	CodeSimProtoUnsup:          ErrProtocolUnsupported,
	CodeValidationFailed:       ErrValidationFailed,
	CodeConfigFailed:           ErrConfigFailed,
}

// newErstError is the internal constructor.
func newErstError(code ErstErrorCode, message string, original error) *ErstError {
	if message == "" && original != nil {
		message = original.Error()
	}
	return &ErstError{Code: code, Message: message, OrigErr: original}
}

// --- Typed constructors for RPC boundary ---

// NewRPCError wraps any RPC error into the unified type.
func NewRPCError(code ErstErrorCode, original error) *ErstError {
	return newErstError(code, "", original)
}

// --- Typed constructors for Simulator boundary ---

// NewSimError wraps any Simulator error into the unified type.
func NewSimError(code ErstErrorCode, original error) *ErstError {
	return newErstError(code, "", original)
}

// NewSimErrorMsg wraps a simulator error with an explicit message (for string-only errors).
func NewSimErrorMsg(code ErstErrorCode, message string) *ErstError {
	return newErstError(code, message, nil)
}

// IsErstCode checks if an error carries a specific ErstErrorCode.
func IsErstCode(err error, code ErstErrorCode) bool {
	var e *ErstError
	if As(err, &e) {
		return e.Code == code
	}
	return false
}
