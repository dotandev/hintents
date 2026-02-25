// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"fmt"
)

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
	ErrConfigFailed         = errors.New("configuration error")
	ErrNetworkNotFound      = errors.New("network not found")
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

// Wrap functions for consistent error wrapping
func WrapTransactionNotFound(err error) error {
	return fmt.Errorf("%w: %w", ErrTransactionNotFound, err)
}

func WrapRPCConnectionFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrRPCConnectionFailed, err)
}

func WrapSimulatorNotFound(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulatorNotFound, msg)
}

func WrapSimulationFailed(err error, stderr string) error {
	return fmt.Errorf("%w: %w, stderr: %s", ErrSimulationFailed, err, stderr)
}

func WrapInvalidNetwork(network string) error {
	return fmt.Errorf("%w: %s. Must be one of: testnet, mainnet, futurenet", ErrInvalidNetwork, network)
}

func WrapMarshalFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrMarshalFailed, err)
}

func WrapUnmarshalFailed(err error, output string) error {
	return fmt.Errorf("%w: %w, output: %s", ErrUnmarshalFailed, err, output)
}

func WrapSimulationLogicError(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulationLogicError, msg)
}

func WrapRPCTimeout(err error) error {
	return fmt.Errorf("%w: %w", ErrRPCTimeout, err)
}

func WrapAllRPCFailed() error {
	return ErrAllRPCFailed
}

func WrapRPCError(url string, msg string, code int) error {
	return fmt.Errorf("%w from %s: %s (code %d)", ErrRPCError, url, msg, code)
}

func WrapSimCrash(err error, stderr string) error {
	if stderr != "" {
		return fmt.Errorf("%w: %w, stderr: %s", ErrSimCrash, err, stderr)
	}
	return fmt.Errorf("%w: %w", ErrSimCrash, err)
}

func WrapValidationError(msg string) error {
	return fmt.Errorf("%w: %s", ErrValidationFailed, msg)
}

func WrapProtocolUnsupported(version uint32) error {
	return fmt.Errorf("%w: %d", ErrProtocolUnsupported, version)
}

func WrapCliArgumentRequired(arg string) error {
	return fmt.Errorf("%w: --%s", ErrArgumentRequired, arg)
}

func WrapAuditLogInvalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrAuditLogInvalid, msg)
}

func WrapSessionNotFound(sessionID string) error {
	return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
}

func WrapUnauthorized(msg string) error {
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrUnauthorized, msg)
	}
	return ErrUnauthorized
}

func WrapLedgerNotFound(sequence uint32) error {
	return &LedgerNotFoundError{
		Sequence: sequence,
		Message:  fmt.Sprintf("%v: ledger %d not found (may be archived or not yet created)", ErrLedgerNotFound, sequence),
	}
}

func WrapLedgerArchived(sequence uint32) error {
	return &LedgerArchivedError{
		Sequence: sequence,
		Message:  fmt.Sprintf("%v: ledger %d has been archived and is no longer available", ErrLedgerArchived, sequence),
	}
}

func WrapRateLimitExceeded() error {
	return &RateLimitError{
		Message: fmt.Sprintf("%v, please try again later", ErrRateLimitExceeded),
	}
}

func WrapConfigError(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrConfigFailed, msg, err)
	}
	return fmt.Errorf("%w: %s", ErrConfigFailed, msg)
}

func WrapNetworkNotFound(network string) error {
	return fmt.Errorf("%w: %s", ErrNetworkNotFound, network)
}
