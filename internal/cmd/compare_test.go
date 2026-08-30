// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseContractWasmOverrideSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "bridge.wasm")
	if err := os.WriteFile(wasmPath, []byte{0x00, 0x61, 0x73, 0x6d}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	overrides, err := parseContractWasmOverrideSpecs([]string{
		"cafebabe=" + wasmPath,
	})
	if err != nil {
		t.Fatalf("parseContractWasmOverrideSpecs: %v", err)
	}
	if got := overrides["cafebabe"]; got != wasmPath {
		t.Fatalf("expected override path %q, got %q", wasmPath, got)
	}
}

func TestParseContractWasmOverrideSpecsRejectsInvalidSpec(t *testing.T) {
	if _, err := parseContractWasmOverrideSpecs([]string{"missing-separator"}); err == nil {
		t.Fatal("expected parseContractWasmOverrideSpecs to reject malformed override")
	}
}

func TestCloneStringMap(t *testing.T) {
	original := map[string]string{"a": "1"}
	cloned := cloneStringMap(original)
	cloned["a"] = "2"

	if original["a"] != "1" {
		t.Fatalf("expected original map to stay unchanged, got %q", original["a"])
	}
}
