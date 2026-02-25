// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const wasmTargetPath = "target/wasm32-unknown-unknown/release"

// DiscoverLocalSymbols scans for WASM files in the local target directory.
// It returns a map of WASM hashes to their absolute file paths.
func DiscoverLocalSymbols(projectRoot string) (map[string]string, error) {
	searchDir := filepath.Join(projectRoot, wasmTargetPath)
	found := make(map[string]string)

	if _, err := os.Stat(searchDir); os.IsNotExist(err) {
		return found, nil
	}

	files, err := os.ReadDir(searchDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".wasm") {
			fullPath := filepath.Join(searchDir, file.Name())
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			// Calculate SHA256 hash to match against contract bytecode
			hash := sha256.Sum256(content)
			hashStr := hex.EncodeToString(hash[:])
			found[hashStr] = fullPath
		}
	}

	return found, nil
}
