// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WasmDebugInfo holds the path and extracted contract ID of a local WASM file.
type WasmDebugInfo struct {
	Path       string
	ContractID string
}

// DiscoverLocalWasmFiles scans the standard Rust WASM release directory.
func DiscoverLocalWasmFiles() ([]WasmDebugInfo, error) {
	targetDir := filepath.Join("target", "wasm32-unknown-unknown", "release")
	var discoveredFiles []WasmDebugInfo

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory not found is a valid state if not compiled yet
		}
		return nil, fmt.Errorf("failed to read target directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".wasm") {
			fullPath := filepath.Join(targetDir, entry.Name())

			// Extract or generate the Contract ID for matching
			contractID, err := extractContractID(fullPath)
			if err != nil {
				continue // Skip unreadable files
			}

			discoveredFiles = append(discoveredFiles, WasmDebugInfo{
				Path:       fullPath,
				ContractID: contractID,
			})
		}
	}

	return discoveredFiles, nil
}

// extractContractID reads the WASM file to determine its associated contract ID.
// This is typically a hash of the WASM bytecode or extracted from a custom section.
func extractContractID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Fallback implementation: Hash the WASM byte code to match against the trace's hash
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// MergeDebugSymbols attempts to match a given contract ID with local WASM files
// and merge the DWARF symbols into the trace viewer context.
func MergeDebugSymbols(expectedContractID string) error {
	localFiles, err := DiscoverLocalWasmFiles()
	if err != nil {
		return err
	}

	for _, file := range localFiles {
		if file.ContractID == expectedContractID {
			// Logic to parse the DWARF custom sections from the WASM file.
			// This typically integrates with your existing viewer state to map addresses to lines.
			return parseAndMergeDwarf(file.Path)
		}
	}

	return nil
}

func parseAndMergeDwarf(wasmPath string) error {
	// Stub for the actual DWARF extraction logic using debug/dwarf or passing to the Rust IPC.
	return nil
}
