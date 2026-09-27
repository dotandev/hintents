// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// safeWasmPath resolves rawPath to an absolute path and verifies it stays
// within baseDir, rejecting directory-traversal sequences such as "../../".
//
// The check mirrors the pattern in internal/cli/batch.go: use filepath.Abs to
// canonicalise, then filepath.Rel to confirm the result is still inside the
// intended root (a traversal produces a relative path that starts with "..").
//
// baseDir is normally the process working directory (os.Getwd()), which is the
// workspace root the user runs erst from.
func safeWasmPath(rawPath, baseDir string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("path must not be empty")
	}

	// Resolve both sides to absolute, cleaned paths so that symlinks and
	// redundant separators cannot be used to obscure traversal.
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", rawPath, err)
	}

	// filepath.Rel returns a relative path from absBase to absPath.
	// If that relative path starts with ".." the resolved path escapes baseDir.
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return "", fmt.Errorf("path %q is not reachable from workspace root: %w", rawPath, err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf(
			"path %q resolves to %q which is outside the workspace root %q",
			rawPath, absPath, absBase,
		)
	}

	return absPath, nil
}
