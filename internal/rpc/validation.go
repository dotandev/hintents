// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

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

// EnvelopeValidationError describes a malformed JSON-RPC response envelope.
type EnvelopeValidationError struct {
	Field  string
	Reason string
}

func (e *EnvelopeValidationError) Error() string {
	if e.Field == "" {
		return "invalid JSON-RPC response envelope: " + e.Reason
	}
	return fmt.Sprintf("invalid JSON-RPC response envelope: %s: %s", e.Field, e.Reason)
}

// ValidateEnvelope verifies a JSON-RPC 2.0 response and matches its ID to the
// ID used by the corresponding request.
func ValidateEnvelope(response []byte, expectedID any) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(response, &envelope); err != nil {
		return &EnvelopeValidationError{Reason: fmt.Sprintf("invalid JSON object: %v", err)}
	}
	if envelope == nil {
		return &EnvelopeValidationError{Reason: "response must be a JSON object"}
	}

	var version string
	if rawVersion, ok := envelope["jsonrpc"]; !ok {
		return &EnvelopeValidationError{Field: "jsonrpc", Reason: "field is required"}
	} else if err := json.Unmarshal(rawVersion, &version); err != nil || version != "2.0" {
		return &EnvelopeValidationError{Field: "jsonrpc", Reason: "must be \"2.0\""}
	}

	actualID, ok := envelope["id"]
	if !ok {
		return &EnvelopeValidationError{Field: "id", Reason: "field is required"}
	}
	expectedIDBytes, err := json.Marshal(expectedID)
	if err != nil {
		return &EnvelopeValidationError{Field: "id", Reason: fmt.Sprintf("cannot encode expected ID: %v", err)}
	}
	if !equalJSONRPCID(actualID, expectedIDBytes) {
		return &EnvelopeValidationError{Field: "id", Reason: "does not match request ID"}
	}

	_, hasResult := envelope["result"]
	rawError, hasError := envelope["error"]
	if hasResult == hasError {
		return &EnvelopeValidationError{Reason: "exactly one of result or error is required"}
	}
	if hasError {
		var rpcError map[string]json.RawMessage
		if err := json.Unmarshal(rawError, &rpcError); err != nil || rpcError == nil {
			return &EnvelopeValidationError{Field: "error", Reason: "must be an object"}
		}
		if rawCode, ok := rpcError["code"]; !ok || !isJSONInteger(rawCode) {
			return &EnvelopeValidationError{Field: "error.code", Reason: "must be an integer"}
		}
		rawMessage, ok := rpcError["message"]
		if !ok || len(rawMessage) == 0 || rawMessage[0] != '"' {
			return &EnvelopeValidationError{Field: "error.message", Reason: "must be a string"}
		}
	}

	return nil
}

func equalJSONRPCID(actual, expected json.RawMessage) bool {
	actualID, actualOK := decodeJSONRPCID(actual)
	expectedID, expectedOK := decodeJSONRPCID(expected)
	if !actualOK || !expectedOK {
		return false
	}

	actualNumber, actualIsNumber := actualID.(json.Number)
	expectedNumber, expectedIsNumber := expectedID.(json.Number)
	if actualIsNumber || expectedIsNumber {
		if !actualIsNumber || !expectedIsNumber {
			return false
		}
		actualRational, actualOK := new(big.Rat).SetString(actualNumber.String())
		expectedRational, expectedOK := new(big.Rat).SetString(expectedNumber.String())
		return actualOK && expectedOK && actualRational.Cmp(expectedRational) == 0
	}

	actualString, actualIsString := actualID.(string)
	expectedString, expectedIsString := expectedID.(string)
	return actualIsString && expectedIsString && actualString == expectedString
}

func decodeJSONRPCID(raw json.RawMessage) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var id any
	if err := decoder.Decode(&id); err != nil {
		return nil, false
	}
	switch id.(type) {
	case json.Number, string:
		return id, true
	default:
		return nil, false
	}
}

func isJSONInteger(raw json.RawMessage) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return false
	}
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	_, ok = new(big.Int).SetString(strings.TrimSpace(number.String()), 10)
	return ok
}
