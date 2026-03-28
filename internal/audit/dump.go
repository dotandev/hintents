// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
)

// AuditDump is the raw {input, state, events} JSON payload produced by AuditLogger.
type AuditDump struct {
	Input     map[string]interface{} `json:"input"`
	State     map[string]interface{} `json:"state"`
	Events    []interface{}          `json:"events"`
	Timestamp string                 `json:"timestamp"`
}

// SignedAuditDump extends AuditDump with signing metadata (matches SignedAuditLog from TS).
type SignedAuditDump struct {
	Trace     AuditDump `json:"trace"`
	Hash      string    `json:"hash"`
	Signature string    `json:"signature"`
	Algorithm string    `json:"algorithm"`
	PublicKey string    `json:"publicKey"`
	Signer    struct {
		Provider string `json:"provider"`
	} `json:"signer"`
}

// ParseAuditDump deserialises raw JSON into an AuditDump.
func ParseAuditDump(data []byte) (*AuditDump, error) {
	var d AuditDump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse audit dump: %w", err)
	}
	return &d, nil
}

// ParseSignedAuditDump deserialises raw JSON into a SignedAuditDump.
func ParseSignedAuditDump(data []byte) (*SignedAuditDump, error) {
	var d SignedAuditDump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("failed to parse signed audit dump: %w", err)
	}
	return &d, nil
}
