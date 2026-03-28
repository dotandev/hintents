// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"os"
	"path/filepath"
	"runtime"
)

// CleanupTempArtifacts removes temporary snapshot/session files from the temp directory.
// Returns the number of files removed and any error encountered.
func CleanupTempArtifacts() (int, error) {
	tmpDir := os.TempDir()
	pattern := "erst-snapshot-*"
	matches, err := filepath.Glob(filepath.Join(tmpDir, pattern))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, file := range matches {
		err := os.RemoveAll(file)
		if err == nil {
			count++
		}
	}
	return count, nil
}
