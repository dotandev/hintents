// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverrideState(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantEntries int
		wantErr     bool
	}{
		{
			name: "valid override with entries",
			content: `{
				"ledger_entries": {
					"key1": "value1",
					"key2": "value2"
				}
			}`,
			wantEntries: 2,
			wantErr:     false,
		},
		{
			name: "empty ledger entries",
			content: `{
				"ledger_entries": {}
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
		{
			name: "null ledger entries",
			content: `{
				"ledger_entries": null
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
		{
			name:        "invalid JSON",
			content:     `{invalid json}`,
			wantEntries: 0,
			wantErr:     true,
		},
		{
			name: "missing ledger_entries field",
			content: `{
				"other_field": "value"
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "override.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			entries, err := loadOverrideState(tmpFile)

			if (err != nil) != tt.wantErr {
				t.Errorf("loadOverrideState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(entries) != tt.wantEntries {
				t.Errorf("loadOverrideState() got %d entries, want %d", len(entries), tt.wantEntries)
			}
		})
	}
}

func TestLoadOverrideState_FileNotFound(t *testing.T) {
	_, err := loadOverrideState("/nonexistent/file.json")
	if err == nil {
		t.Error("loadOverrideState() expected error for nonexistent file, got nil")
	}
}

func TestOverrideDataStructure(t *testing.T) {
	original := OverrideData{
		LedgerEntries: map[string]string{
			"balance_key": "base64_balance_data",
			"contract_key": "base64_contract_data",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal OverrideData: %v", err)
	}

	var decoded OverrideData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal OverrideData: %v", err)
	}

	if len(decoded.LedgerEntries) != len(original.LedgerEntries) {
		t.Errorf("Entry count mismatch: got %d, want %d", len(decoded.LedgerEntries), len(original.LedgerEntries))
	}

	for key, value := range original.LedgerEntries {
		if decoded.LedgerEntries[key] != value {
			t.Errorf("Entry mismatch for key %s: got %s, want %s", key, decoded.LedgerEntries[key], value)
		}
	}
}

func TestOverrideData_SandboxScenario(t *testing.T) {
	tmpDir := t.TempDir()
	overrideFile := filepath.Join(tmpDir, "sandbox_1000xlm.json")

	override := OverrideData{
		LedgerEntries: map[string]string{
			"user_balance": "base64_encoded_1000xlm",
			"contract_state": "base64_encoded_state",
		},
	}

	data, err := json.MarshalIndent(override, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal override: %v", err)
	}

	if err := os.WriteFile(overrideFile, data, 0644); err != nil {
		t.Fatalf("Failed to write override file: %v", err)
	}

	loaded, err := loadOverrideState(overrideFile)
	if err != nil {
		t.Fatalf("Failed to load override state: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("Expected 2 ledger entries, got %d", len(loaded))
	}

	if loaded["user_balance"] != "base64_encoded_1000xlm" {
		t.Errorf("Balance override not preserved correctly")
	}
}
