// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors are defined
	assert.NotNil(t, ErrTransactionNotFound)
	assert.NotNil(t, ErrRPCConnectionFailed)
	assert.NotNil(t, ErrSimulatorNotFound)
	assert.NotNil(t, ErrSimulationFailed)
	assert.NotNil(t, ErrInvalidNetwork)
	assert.NotNil(t, ErrMarshalFailed)
	assert.NotNil(t, ErrUnmarshalFailed)
	assert.NotNil(t, ErrSimulationLogicError)
	assert.NotNil(t, ErrRPCResponseTooLarge)
}

func TestErrorWrapping(t *testing.T) {
	baseErr := fmt.Errorf("base error")

	// Test WrapTransactionNotFound
	wrappedErr := WrapTransactionNotFound(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrTransactionNotFound))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapRPCConnectionFailed
	wrappedErr = WrapRPCConnectionFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrRPCConnectionFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapSimulatorNotFound
	wrappedErr = WrapSimulatorNotFound("test message")
	assert.True(t, errors.Is(wrappedErr, ErrSimulatorNotFound))
	assert.Contains(t, wrappedErr.Error(), "test message")

	// Test WrapSimulationFailed
	wrappedErr = WrapSimulationFailed(baseErr, "stderr output")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "stderr output")

	// Test WrapInvalidNetwork
	wrappedErr = WrapInvalidNetwork("invalid")
	assert.True(t, errors.Is(wrappedErr, ErrInvalidNetwork))
	assert.Contains(t, wrappedErr.Error(), "invalid")
	assert.Contains(t, wrappedErr.Error(), "testnet, mainnet, futurenet")

	// Test WrapMarshalFailed
	wrappedErr = WrapMarshalFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrMarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapUnmarshalFailed
	wrappedErr = WrapUnmarshalFailed(baseErr, "output")
	assert.True(t, errors.Is(wrappedErr, ErrUnmarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "output")

	// Test WrapSimulationLogicError
	wrappedErr = WrapSimulationLogicError("logic error")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationLogicError))
	assert.Contains(t, wrappedErr.Error(), "logic error")
}

func TestErrorComparison(t *testing.T) {
	// Test that different error types are distinguishable
	err1 := WrapTransactionNotFound(fmt.Errorf("test"))
	err2 := WrapRPCConnectionFailed(fmt.Errorf("test"))

	assert.True(t, errors.Is(err1, ErrTransactionNotFound))
	assert.False(t, errors.Is(err1, ErrRPCConnectionFailed))

	assert.True(t, errors.Is(err2, ErrRPCConnectionFailed))
	assert.False(t, errors.Is(err2, ErrTransactionNotFound))
}

func TestWrapRPCResponseTooLarge(t *testing.T) {
	url := "https://soroban-testnet.stellar.org"
	err := WrapRPCResponseTooLarge(url)

	assert.True(t, errors.Is(err, ErrRPCResponseTooLarge))
	assert.False(t, errors.Is(err, ErrRPCConnectionFailed))
	assert.Contains(t, err.Error(), url)
	assert.Contains(t, err.Error(), "exceeded the server's maximum allowed size")
	assert.Contains(t, err.Error(), "Soroban RPC response limit")

	var rte *ResponseTooLargeError
	assert.True(t, errors.As(err, &rte))
	assert.Equal(t, url, rte.URL)
}

func TestErstError_Is_MatchesSentinel(t *testing.T) {
	tests := []struct {
		name     string
		code     ErstErrorCode
		sentinel error
	}{
		{"RPC connection failed", CodeRPCConnectionFailed, ErrRPCConnectionFailed},
		{"RPC timeout", CodeRPCTimeout, ErrRPCTimeout},
		{"All RPC failed", CodeRPCAllFailed, ErrAllRPCFailed},
		{"RPC error", CodeRPCError, ErrRPCError},
		{"RPC response too large", CodeRPCResponseTooLarge, ErrRPCResponseTooLarge},
		{"RPC request too large", CodeRPCRequestTooLarge, ErrRPCRequestTooLarge},
		{"Rate limit exceeded", CodeRPCRateLimitExceeded, ErrRateLimitExceeded},
		{"Marshal failed", CodeRPCMarshalFailed, ErrMarshalFailed},
		{"Unmarshal failed", CodeRPCUnmarshalFailed, ErrUnmarshalFailed},
		{"Transaction not found", CodeTransactionNotFound, ErrTransactionNotFound},
		{"Ledger not found", CodeLedgerNotFound, ErrLedgerNotFound},
		{"Ledger archived", CodeLedgerArchived, ErrLedgerArchived},
		{"Sim not found", CodeSimNotFound, ErrSimulatorNotFound},
		{"Sim crash", CodeSimCrash, ErrSimCrash},
		{"Sim exec failed", CodeSimExecFailed, ErrSimulationFailed},
		{"Sim memory limit", CodeSimMemoryLimitExceeded, ErrSimulationFailed},
		{"Sim logic error", CodeSimLogicError, ErrSimulationLogicError},
		{"Sim proto unsupported", CodeSimProtoUnsup, ErrProtocolUnsupported},
		{"Validation failed", CodeValidationFailed, ErrValidationFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			erstErr := NewSimError(tt.code, fmt.Errorf("original error"))
			assert.True(t, errors.Is(erstErr, tt.sentinel),
				"errors.Is(%v, %v) should be true", tt.code, tt.sentinel)
		})
	}
}

func TestErstError_Is_DoesNotMatchWrongSentinel(t *testing.T) {
	erstErr := NewSimError(CodeSimCrash, fmt.Errorf("crash"))

	assert.False(t, errors.Is(erstErr, ErrRPCConnectionFailed))
	assert.False(t, errors.Is(erstErr, ErrRateLimitExceeded))
	assert.False(t, errors.Is(erstErr, ErrTransactionNotFound))
}

func TestErstError_Is_UnknownCodeMatchesNothing(t *testing.T) {
	erstErr := NewSimError(CodeUnknown, fmt.Errorf("unknown"))

	assert.False(t, errors.Is(erstErr, ErrSimCrash))
	assert.False(t, errors.Is(erstErr, ErrRPCConnectionFailed))
	assert.False(t, errors.Is(erstErr, ErrSimulationFailed))
}

func TestErstError_Unwrap_ReturnsOrigErr(t *testing.T) {
	origErr := fmt.Errorf("crash")
	erstErr := NewSimError(CodeSimCrash, origErr)
	assert.Equal(t, origErr, erstErr.Unwrap(), "Unwrap should return the original error")
}

func TestShellExitSentinel(t *testing.T) {
	assert.True(t, errors.Is(ErrShellExit, ErrShellExit))
	assert.False(t, errors.Is(ErrShellExit, ErrRPCConnectionFailed))
}
