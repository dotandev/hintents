// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"os"
)

type OverrideData struct {
	LedgerEntries map[string]string `json:"ledger_entries,omitempty"`
}

func loadOverrideState(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var override OverrideData
	if err := json.Unmarshal(data, &override); err != nil {
		return nil, err
	}

	return override.LedgerEntries, nil
}
