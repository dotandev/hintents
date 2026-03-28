// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupTempArtifacts(t *testing.T) {
	tmpDir := os.TempDir()
	// Create fake temp files
	files := []string{
		filepath.Join(tmpDir, "erst-snapshot-foo"),
		filepath.Join(tmpDir, "erst-snapshot-bar"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
	}

	count, err := CleanupTempArtifacts()
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if count < len(files) {
		t.Errorf("expected at least %d files cleaned, got %d", len(files), count)
	}
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("file %s was not removed", f)
		}
	}
}
