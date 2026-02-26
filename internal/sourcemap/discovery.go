// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const wasmTargetPath = "target/wasm32-unknown-unknown/release"

// HashMismatchError is returned when the local WASM hash does not match
// the expected on-chain hash.
type HashMismatchError struct {
	Path     string
	Local    string
	OnChain  string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("opt-level mismatch: local WASM hash %q does not match on-chain hash %q (path: %s)", e.Local, e.OnChain, e.Path)
}

// CheckHashMismatch computes the SHA256 hash of the WASM at path and
// compares it to onChainHash. It returns a HashMismatchError when they
// differ, so callers can surface a warning to the user.
func CheckHashMismatch(path, onChainHash string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read WASM file %q: %w", path, err)
	}
	sum := sha256.Sum256(content)
	local := hex.EncodeToString(sum[:])
	if local != onChainHash {
		return &HashMismatchError{Path: path, Local: local, OnChain: onChainHash}
	}
	return nil
}

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
