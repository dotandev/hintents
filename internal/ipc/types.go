// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ipc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

// ToErstError converts an IPC Error from the Rust simulator into the unified ErstError type.
// The original Code and Message strings are preserved in OriginalError.
// Note: the Rust simulator currently emits plain message strings without structured codes,
// so classification falls back to message-based heuristics via classifyByMessage.
func (e *Error) ToErstError() *errors.ErstError {
	code := mapIPCCode(e.Code)
	if code == errors.CodeUnknown {
		code = classifyByMessage(e.Message)
	}
	return errors.NewSimError(code, fmt.Errorf("%s: %s", e.Code, e.Message))
}

// mapIPCCode translates structured IPC error code strings from the Rust simulator
// into the unified ErstErrorCode classification.
// Currently the Rust simulator does not emit structured codes, so this will
// return CodeUnknown in most cases and ToErstError will fall back to classifyByMessage.
func mapIPCCode(raw string) errors.ErstErrorCode {
	switch strings.ToUpper(raw) {
	case "SIMULATION_FAILED", "EXECUTION_FAILED":
		return errors.CodeSimExecFailed
	case "WASM_TRAP", "CONTRACT_TRAP":
		return errors.CodeSimCrash
	case "INVALID_INPUT", "VALIDATION_ERROR":
		return errors.CodeValidationFailed
	case "PROTOCOL_UNSUPPORTED":
		return errors.CodeSimProtoUnsup
	case "ERR_MEMORY_LIMIT_EXCEEDED", "MEMORY_LIMIT_EXCEEDED":
		return errors.CodeSimMemoryLimitExceeded
	default:
		return errors.CodeUnknown
	}
}

// classifyByMessage inspects the raw error message from the Rust simulator
// and maps it to the best-matching ErstErrorCode.
// This is a fallback for when the simulator does not emit a structured code field.
func classifyByMessage(msg string) errors.ErstErrorCode {
	switch {
	case strings.Contains(msg, "decode Envelope"),
		strings.Contains(msg, "decode LedgerKey"),
		strings.Contains(msg, "decode LedgerEntry"),
		strings.Contains(msg, "decode WASM"):
		return errors.CodeRPCUnmarshalFailed
	case strings.Contains(msg, "Wasm Trap"),
		strings.Contains(msg, "wasm trap"),
		strings.Contains(msg, "unreachable"),
		strings.Contains(msg, "stack overflow"),
		strings.Contains(msg, "out of bounds"):
		return errors.CodeSimCrash
	case strings.Contains(strings.ToLower(msg), "err_memory_limit_exceeded"),
		strings.Contains(strings.ToLower(msg), "memory limit exceeded"):
		return errors.CodeSimMemoryLimitExceeded
	case strings.Contains(msg, "InvalidInput"):
		return errors.CodeValidationFailed
	default:
		return errors.CodeSimExecFailed
	}
}

func UnmarshalSimulationRequestSchema(data []byte) (SimulationRequestSchema, error) {
	var r SimulationRequestSchema
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SimulationRequestSchema) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalSimulationResponseSchema(data []byte) (SimulationResponseSchema, error) {
	var r SimulationResponseSchema
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *SimulationResponseSchema) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type SimulationRequestSchema struct {
	Network Network `json:"network"`
	// Client-generated unique request identifier
	RequestID string `json:"request_id"`
	Version   string `json:"version"`
	Xdr       string `json:"xdr"`
}

type SimulationResponseSchema struct {
	Error     *Error  `json:"error,omitempty"`
	RequestID string  `json:"request_id"`
	Result    *Result `json:"result,omitempty"`
	Success   bool    `json:"success"`
	Version   string  `json:"version"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	// Fee charged in stroops
	FeeCharged string `json:"fee_charged"`
}

type Network string

const (
	Futurenet Network = "futurenet"
	Public    Network = "public"
	Testnet   Network = "testnet"
)
