// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

type SecurityViolation struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Contract    string                 `json:"contract"`
	Details     map[string]interface{} `json:"details,omitempty"`
}
