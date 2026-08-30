// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Issue #1273: Ledger Entry Mocking CLI & API
package simulator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type LedgerOverrideManifest struct {
	LedgerEntries map[string]string `json:"ledger_entries,omitempty"`
}

func LoadLedgerOverrideManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest LedgerOverrideManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return manifest.LedgerEntries, nil
}

func ParseLedgerOverrideFlags(entries []string) (map[string]string, error) {
	overrides := make(map[string]string)
	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid ledger override format: %q, expected key:value", entry)
		}
		overrides[parts[0]] = parts[1]
	}

	return overrides, nil
}

func MergeLedgerOverrides(base map[string]string, overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return base
	}

	if base == nil {
		base = make(map[string]string)
	}

	for key, value := range overrides {
		base[key] = value
	}

	return base
}
