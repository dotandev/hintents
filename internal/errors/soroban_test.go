// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifySorobanError_ContractPanic(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedCode    SorobanErrorCode
		expectedDetails string
	}{
		{
			name:            "panic keyword",
			message:         "contract trapped: panic: assertion failed",
			expectedCode:    CodeContractPanic,
			expectedDetails: "assertion failed",
		},
		{
			name:         "unreachable",
			message:      "wasm unreachable instruction executed",
			expectedCode: CodeContractPanic,
		},
		{
			name:         "contract trapped",
			message:      "contract trapped: memory access out of bounds",
			expectedCode: CodeContractPanic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifySorobanError("http://localhost", tt.message, -32001)
			assert.Equal(t, tt.expectedCode, err.Code)
			assert.True(t, errors.Is(err, ErrContractPanic))
			if tt.expectedDetails != "" {
				assert.Contains(t, err.Details, tt.expectedDetails)
			}
		})
	}
}

func TestClassifySorobanError_BudgetExceeded(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"cpu limit", "transaction simulation failed: cpu limit exceeded"},
		{"mem limit", "transaction simulation failed: mem limit exceeded"},
		{"budget", "budget exceeded while executing contract"},
		{"resource limit", "resource limit exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifySorobanError("http://localhost", tt.message, -32003)
			assert.Equal(t, CodeBudgetExceeded, err.Code)
			assert.True(t, errors.Is(err, ErrBudgetExceeded))
		})
	}
}

func TestClassifySorobanError_ContractNotFound(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"not found", "contract not found"},
		{"no such contract", "no such contract at address"},
		{"missing contract", "missing contract code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifySorobanError("http://localhost", tt.message, -32001)
			assert.Equal(t, CodeContractNotFound, err.Code)
			assert.True(t, errors.Is(err, ErrContractNotFound))
		})
	}
}

func TestClassifySorobanError_AuthFailed(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"authorization", "authorization failed for address"},
		{"auth fail", "auth failed: missing signature"},
		{"unauthorized", "unauthorized access to contract"},
		{"invalid signature", "signature invalid for transaction"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifySorobanError("http://localhost", tt.message, -32001)
			assert.Equal(t, CodeAuthFailed, err.Code)
			assert.True(t, errors.Is(err, ErrAuthFailed))
		})
	}
}

func TestClassifySorobanError_Unknown(t *testing.T) {
	err := ClassifySorobanError("http://localhost", "some random error", -32000)
	assert.Equal(t, CodeSorobanUnknown, err.Code)
	assert.True(t, errors.Is(err, ErrRPCError))
}

func TestSorobanError_Is(t *testing.T) {
	err := ClassifySorobanError("http://localhost", "panic: test", -32001)

	assert.True(t, errors.Is(err, ErrContractPanic))
	assert.True(t, errors.Is(err, ErrRPCError))
	assert.False(t, errors.Is(err, ErrBudgetExceeded))
	assert.False(t, errors.Is(err, ErrContractNotFound))
}

func TestSorobanError_Unwrap(t *testing.T) {
	err := ClassifySorobanError("http://localhost", "budget exceeded", -32003)
	unwrapped := err.Unwrap()
	assert.Equal(t, ErrBudgetExceeded, unwrapped)
}

func TestSorobanError_Error(t *testing.T) {
	err := &SorobanError{
		Code:    CodeContractPanic,
		Message: "contract failed",
		Details: "assertion failed at line 42",
		URL:     "http://localhost",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "SOROBAN_CONTRACT_PANIC")
	assert.Contains(t, errStr, "contract failed")
	assert.Contains(t, errStr, "assertion failed at line 42")
}

func TestSorobanError_ErrorNoDetails(t *testing.T) {
	err := &SorobanError{
		Code:    CodeBudgetExceeded,
		Message: "budget exceeded",
		URL:     "http://localhost",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "SOROBAN_BUDGET_EXCEEDED")
	assert.Contains(t, errStr, "budget exceeded")
	assert.NotContains(t, errStr, "[") // No details bracket
}

func TestHelperFunctions(t *testing.T) {
	panicErr := ClassifySorobanError("http://localhost", "panic: test", -32001)
	budgetErr := ClassifySorobanError("http://localhost", "budget exceeded", -32003)
	contractErr := ClassifySorobanError("http://localhost", "contract not found", -32001)
	authErr := ClassifySorobanError("http://localhost", "authorization failed", -32001)

	assert.True(t, IsSorobanError(panicErr))
	assert.True(t, IsContractPanic(panicErr))
	assert.False(t, IsContractPanic(budgetErr))

	assert.True(t, IsBudgetExceeded(budgetErr))
	assert.False(t, IsBudgetExceeded(panicErr))

	assert.True(t, IsContractNotFound(contractErr))
	assert.True(t, IsAuthFailed(authErr))
}

func TestGetSorobanErrorDetails(t *testing.T) {
	err := ClassifySorobanError("http://localhost", "panic: assertion failed", -32001)
	details, ok := GetSorobanErrorDetails(err)
	assert.True(t, ok)
	assert.Equal(t, "assertion failed", details)

	// Error without details
	err2 := ClassifySorobanError("http://localhost", "contract not found", -32001)
	_, ok = GetSorobanErrorDetails(err2)
	assert.False(t, ok)
}

func TestGetSorobanErrorCode(t *testing.T) {
	err := ClassifySorobanError("http://localhost", "budget exceeded", -32003)
	code, ok := GetSorobanErrorCode(err)
	assert.True(t, ok)
	assert.Equal(t, CodeBudgetExceeded, code)

	// Non-Soroban error
	regularErr := New("regular error")
	_, ok = GetSorobanErrorCode(regularErr)
	assert.False(t, ok)
}

func TestWrapSorobanError(t *testing.T) {
	err := WrapSorobanError("http://localhost:8000", "panic: division by zero", -32001)
	assert.True(t, errors.Is(err, ErrContractPanic))

	var se *SorobanError
	assert.True(t, errors.As(err, &se))
	assert.Equal(t, "http://localhost:8000", se.URL)
	assert.Equal(t, -32001, se.RPCCode)
}
