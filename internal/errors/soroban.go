// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"fmt"
	"strings"
)

// Soroban RPC error codes (JSON-RPC standard and Soroban-specific)
const (
	// JSON-RPC standard errors
	SorobanCodeParseError     = -32700
	SorobanCodeInvalidRequest = -32600
	SorobanCodeMethodNotFound = -32601
	SorobanCodeInvalidParams  = -32602
	SorobanCodeInternalError  = -32603

	// Soroban-specific errors (server-defined range: -32000 to -32099)
	SorobanCodeTransactionFailed = -32001
	SorobanCodeSimulationFailed  = -32002
	SorobanCodeResourceExceeded  = -32003
)

// Sentinel errors for Soroban-specific error types
var (
	ErrContractPanic      = New("contract panic")
	ErrBudgetExceeded     = New("budget exceeded")
	ErrContractNotFound   = New("contract not found")
	ErrHostFunctionFailed = New("host function failed")
	ErrAuthFailed         = New("authorization failed")
	ErrStorageFull        = New("storage limit exceeded")
	ErrInvalidInvocation  = New("invalid contract invocation")
)

// SorobanErrorCode represents categorized Soroban error codes
type SorobanErrorCode string

const (
	CodeContractPanic      SorobanErrorCode = "SOROBAN_CONTRACT_PANIC"
	CodeBudgetExceeded     SorobanErrorCode = "SOROBAN_BUDGET_EXCEEDED"
	CodeContractNotFound   SorobanErrorCode = "SOROBAN_CONTRACT_NOT_FOUND"
	CodeHostFunctionFailed SorobanErrorCode = "SOROBAN_HOST_FUNCTION_FAILED"
	CodeAuthFailed         SorobanErrorCode = "SOROBAN_AUTH_FAILED"
	CodeStorageFull        SorobanErrorCode = "SOROBAN_STORAGE_FULL"
	CodeInvalidInvocation  SorobanErrorCode = "SOROBAN_INVALID_INVOCATION"
	CodeSorobanUnknown     SorobanErrorCode = "SOROBAN_UNKNOWN"
)

// SorobanError represents a structured Soroban RPC error with preserved details
type SorobanError struct {
	Code       SorobanErrorCode
	RPCCode    int
	Message    string
	Details    string // Preserved details like panic messages
	URL        string
	Diagnostic string // Additional diagnostic info from the RPC response
}

func (e *SorobanError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s [%s]", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *SorobanError) Unwrap() error {
	return sorobanCodeToSentinel[e.Code]
}

func (e *SorobanError) Is(target error) bool {
	// All Soroban errors are RPC errors
	if target == ErrRPCError {
		return true
	}
	if sentinel, ok := sorobanCodeToSentinel[e.Code]; ok {
		return target == sentinel
	}
	return false
}

// sorobanCodeToSentinel maps SorobanErrorCode to sentinel errors
var sorobanCodeToSentinel = map[SorobanErrorCode]error{
	CodeContractPanic:      ErrContractPanic,
	CodeBudgetExceeded:     ErrBudgetExceeded,
	CodeContractNotFound:   ErrContractNotFound,
	CodeHostFunctionFailed: ErrHostFunctionFailed,
	CodeAuthFailed:         ErrAuthFailed,
	CodeStorageFull:        ErrStorageFull,
	CodeInvalidInvocation:  ErrInvalidInvocation,
}

// ClassifySorobanError maps a Soroban RPC error to a typed SorobanError
func ClassifySorobanError(url string, message string, rpcCode int) *SorobanError {
	code, details := classifyByMessage(message)

	return &SorobanError{
		Code:    code,
		RPCCode: rpcCode,
		Message: message,
		Details: details,
		URL:     url,
	}
}

// classifyByMessage analyzes the error message to determine the error type
func classifyByMessage(message string) (SorobanErrorCode, string) {
	lowerMsg := strings.ToLower(message)

	// Contract panic detection
	if strings.Contains(lowerMsg, "panic") ||
		strings.Contains(lowerMsg, "contract trapped") ||
		strings.Contains(lowerMsg, "unreachable") {
		details := extractPanicDetails(message)
		return CodeContractPanic, details
	}

	// Budget exceeded detection
	if strings.Contains(lowerMsg, "budget") ||
		strings.Contains(lowerMsg, "cpu limit") ||
		strings.Contains(lowerMsg, "mem limit") ||
		strings.Contains(lowerMsg, "exceeded the limit") ||
		strings.Contains(lowerMsg, "resource limit") {
		return CodeBudgetExceeded, extractBudgetDetails(message)
	}

	// Contract not found
	if strings.Contains(lowerMsg, "contract not found") ||
		strings.Contains(lowerMsg, "no such contract") ||
		strings.Contains(lowerMsg, "missing contract") {
		return CodeContractNotFound, ""
	}

	// Host function failures
	if strings.Contains(lowerMsg, "host function") ||
		strings.Contains(lowerMsg, "hostfunction") ||
		strings.Contains(lowerMsg, "invoke_contract") {
		return CodeHostFunctionFailed, ""
	}

	// Authorization failures
	if strings.Contains(lowerMsg, "authorization") ||
		strings.Contains(lowerMsg, "auth") && strings.Contains(lowerMsg, "fail") ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "signature") && strings.Contains(lowerMsg, "invalid") {
		return CodeAuthFailed, ""
	}

	// Storage limit
	if strings.Contains(lowerMsg, "storage") &&
		(strings.Contains(lowerMsg, "limit") || strings.Contains(lowerMsg, "full")) {
		return CodeStorageFull, ""
	}

	// Invalid invocation
	if strings.Contains(lowerMsg, "invalid") &&
		(strings.Contains(lowerMsg, "argument") ||
			strings.Contains(lowerMsg, "parameter") ||
			strings.Contains(lowerMsg, "invocation")) {
		return CodeInvalidInvocation, ""
	}

	return CodeSorobanUnknown, ""
}

// extractPanicDetails attempts to extract the panic message from the error
func extractPanicDetails(message string) string {
	// Look for common panic message patterns
	patterns := []string{
		"panic: ",
		"contract trapped: ",
		"error: ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(strings.ToLower(message), strings.ToLower(pattern)); idx != -1 {
			start := idx + len(pattern)
			if start < len(message) {
				return strings.TrimSpace(message[start:])
			}
		}
	}

	return ""
}

// extractBudgetDetails extracts budget-related details from the error message
func extractBudgetDetails(message string) string {
	// Look for numeric values that might indicate budget usage
	if strings.Contains(message, "cpu") || strings.Contains(message, "mem") {
		return message
	}
	return ""
}

// WrapSorobanError creates a SorobanError from RPC response data
func WrapSorobanError(url string, message string, rpcCode int) error {
	return ClassifySorobanError(url, message, rpcCode)
}

// IsSorobanError checks if an error is a SorobanError
func IsSorobanError(err error) bool {
	var se *SorobanError
	return As(err, &se)
}

// IsContractPanic checks if the error is a contract panic
func IsContractPanic(err error) bool {
	return Is(err, ErrContractPanic)
}

// IsBudgetExceeded checks if the error is a budget exceeded error
func IsBudgetExceeded(err error) bool {
	return Is(err, ErrBudgetExceeded)
}

// IsContractNotFound checks if the error is a contract not found error
func IsContractNotFound(err error) bool {
	return Is(err, ErrContractNotFound)
}

// IsAuthFailed checks if the error is an authorization failure
func IsAuthFailed(err error) bool {
	return Is(err, ErrAuthFailed)
}

// GetSorobanErrorDetails extracts details from a SorobanError if present
func GetSorobanErrorDetails(err error) (details string, ok bool) {
	var se *SorobanError
	if As(err, &se) {
		return se.Details, se.Details != ""
	}
	return "", false
}

// GetSorobanErrorCode returns the SorobanErrorCode if the error is a SorobanError
func GetSorobanErrorCode(err error) (SorobanErrorCode, bool) {
	var se *SorobanError
	if As(err, &se) {
		return se.Code, true
	}
	return "", false
}
