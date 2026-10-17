// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
)

// ValidateTransactionHash checks if the provided string is a valid Stellar transaction hash.
// A valid hash must be exactly 64 characters long and contain only hexadecimal characters.
// The check is case-insensitive.
func ValidateTransactionHash(hash string) error {
	if len(hash) != 64 {
		return errors.WrapValidationError(fmt.Sprintf("transaction hash must be exactly 64 characters long, got %d", len(hash)))
	}
	_, err := hex.DecodeString(hash)
	if err != nil {
		return errors.WrapValidationError("transaction hash must contain only valid hexadecimal characters")
	}
	return nil
}

// RPCEnvelopeValidationError represents a typed validation failure for a JSON-RPC 2.0 response envelope.
type RPCEnvelopeValidationError struct {
	Field   string
	Message string
}

func (e *RPCEnvelopeValidationError) Error() string {
	return fmt.Sprintf("invalid json-rpc 2.0 envelope [%s]: %s", e.Field, e.Message)
}

// RPCResponseEnvelope represents a raw JSON-RPC 2.0 response envelope.
type RPCResponseEnvelope struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// matchEnvelopeIDs compares two JSON-RPC IDs, accounting for numeric and string types.
func matchEnvelopeIDs(expected, actual any) bool {
	if expected == actual {
		return true
	}
	return fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
}

// ValidateEnvelope validates a JSON-RPC 2.0 response envelope according to the specification:
// 1. jsonrpc member MUST be present and exactly "2.0".
// 2. id member MUST match expectedID (if expectedID is provided, i.e. not nil).
// 3. Exactly one of result or error MUST be included; both or neither is an error.
func ValidateEnvelope(resp *RPCResponseEnvelope, expectedID any) error {
	if resp == nil {
		return &RPCEnvelopeValidationError{
			Field:   "envelope",
			Message: "response envelope is nil",
		}
	}

	if resp.Jsonrpc != "2.0" {
		return &RPCEnvelopeValidationError{
			Field:   "jsonrpc",
			Message: fmt.Sprintf("expected jsonrpc version \"2.0\", got %q", resp.Jsonrpc),
		}
	}

	if expectedID != nil {
		if !matchEnvelopeIDs(expectedID, resp.ID) {
			return &RPCEnvelopeValidationError{
				Field:   "id",
				Message: fmt.Sprintf("response id %v does not match expected id %v", resp.ID, expectedID),
			}
		}
	}

	hasResult := len(resp.Result) > 0 && string(resp.Result) != "null"
	hasError := len(resp.Error) > 0 && string(resp.Error) != "null"

	if hasResult && hasError {
		return &RPCEnvelopeValidationError{
			Field:   "result/error",
			Message: "both result and error members are present; must be mutually exclusive",
		}
	}

	if !hasResult && !hasError {
		return &RPCEnvelopeValidationError{
			Field:   "result/error",
			Message: "neither result nor error member is present",
		}
	}

	return nil
}

// ValidateEnvelopeBytes decodes and validates a raw JSON-RPC 2.0 response payload.
func ValidateEnvelopeBytes(data []byte, expectedID any) (*RPCResponseEnvelope, error) {
	if len(data) == 0 {
		return nil, &RPCEnvelopeValidationError{
			Field:   "payload",
			Message: "empty response payload",
		}
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, &RPCEnvelopeValidationError{
			Field:   "payload",
			Message: fmt.Sprintf("invalid JSON payload: %v", err),
		}
	}

	// 1. Verify jsonrpc field existence and value
	rawVersion, hasVersion := rawMap["jsonrpc"]
	if !hasVersion {
		return nil, &RPCEnvelopeValidationError{
			Field:   "jsonrpc",
			Message: "missing jsonrpc version",
		}
	}

	var version string
	if err := json.Unmarshal(rawVersion, &version); err != nil || version != "2.0" {
		return nil, &RPCEnvelopeValidationError{
			Field:   "jsonrpc",
			Message: fmt.Sprintf("expected jsonrpc version \"2.0\", got %s", string(rawVersion)),
		}
	}

	// 2. Verify mutual exclusivity of result and error keys
	_, hasResultKey := rawMap["result"]
	_, hasErrorKey := rawMap["error"]

	if hasResultKey && hasErrorKey {
		return nil, &RPCEnvelopeValidationError{
			Field:   "result/error",
			Message: "both result and error members are present; must be mutually exclusive",
		}
	}

	if !hasResultKey && !hasErrorKey {
		return nil, &RPCEnvelopeValidationError{
			Field:   "result/error",
			Message: "neither result nor error member is present",
		}
	}

	var env RPCResponseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, &RPCEnvelopeValidationError{
			Field:   "envelope",
			Message: fmt.Sprintf("failed to parse envelope: %v", err),
		}
	}

	// 3. Verify request/response ID matching
	if expectedID != nil {
		if !matchEnvelopeIDs(expectedID, env.ID) {
			return nil, &RPCEnvelopeValidationError{
				Field:   "id",
				Message: fmt.Sprintf("response id %v does not match expected id %v", env.ID, expectedID),
			}
		}
	}

	return &env, nil
}
