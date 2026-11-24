// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Dump is the raw {input, state, events} JSON payload produced by AuditLogger.
type Dump struct {
	Input     map[string]interface{} `json:"input"`
	State     map[string]interface{} `json:"state"`
	Events    []interface{}          `json:"events"`
	Timestamp string                 `json:"timestamp"`
}

// SignedDump extends Dump with signing metadata (matches SignedAuditLog from TS).
type SignedDump struct {
	Trace     Dump   `json:"trace"`
	Hash      string `json:"hash"`
	Signature string `json:"signature"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
	Signer    struct {
		Provider string `json:"provider"`
	} `json:"signer"`
}

var (
	dumpRequiredKeys       = []string{"input", "state", "events", "timestamp"}
	signedDumpRequiredKeys = []string{"trace", "hash", "signature", "algorithm", "publicKey", "signer"}
	signerRequiredKeys     = []string{"provider"}
)

// ParseDump deserialises raw JSON into an Dump.
//
// It returns an error when the payload is not a JSON object, when any of the
// required fields is missing or null, or when a field has the wrong JSON type.
func ParseDump(data []byte) (*Dump, error) {
	obj, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse audit dump: %w", err)
	}
	if err := requireKeys(obj, dumpRequiredKeys...); err != nil {
		return nil, fmt.Errorf("failed to parse audit dump: %w", err)
	}

	var d Dump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse audit dump: %w", err)
	}
	return &d, nil
}

// ParseSignedDump deserialises raw JSON into a SignedDump.
//
// It applies the same rules as ParseDump to the nested "trace" object, and
// additionally requires the signing metadata fields and "signer.provider".
func ParseSignedDump(data []byte) (*SignedDump, error) {
	obj, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: %w", err)
	}
	if err := requireKeys(obj, signedDumpRequiredKeys...); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: %w", err)
	}

	trace, err := decodeObject(obj["trace"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: trace: %w", err)
	}
	if err := requireKeys(trace, dumpRequiredKeys...); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: trace: %w", err)
	}

	signer, err := decodeObject(obj["signer"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: signer: %w", err)
	}
	if err := requireKeys(signer, signerRequiredKeys...); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: signer: %w", err)
	}

	var d SignedDump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: %w", err)
	}
	return &d, nil
}

// decodeObject decodes data into a map of raw JSON values. It fails if data is
// not a JSON object (including the literal null).
func decodeObject(data []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("expected a JSON object, got null")
	}
	return obj, nil
}

// requireKeys reports the first key that is absent from obj or set to null.
func requireKeys(obj map[string]json.RawMessage, keys ...string) error {
	for _, k := range keys {
		v, ok := obj[k]
		if !ok {
			return fmt.Errorf("missing required field %q", k)
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("field %q must not be null", k)
		}
	}
	return nil
}
