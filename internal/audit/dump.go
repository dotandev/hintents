// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
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

// ParseDump deserialises raw JSON into an Dump.
func ParseDump(data []byte) (*Dump, error) {
	var d Dump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse audit dump: %w", err)
	}
	return &d, nil
}

// ParseSignedDump deserialises raw JSON into a SignedDump.
func ParseSignedDump(data []byte) (*SignedDump, error) {
	var d SignedDump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: %w", err)
	}
	return &d, nil
}
