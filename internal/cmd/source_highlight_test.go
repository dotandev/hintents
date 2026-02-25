// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
)

func TestDisplaySourceLocation(t *testing.T) {
	// 1. Create a dummy source file
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "token.rs")
	sourceContent := `fn main() {
    let x = 10;
    let y = x / 0; // Error here
    println!("y: {}", y);
}
`
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// 2. Mock SourceLocation
	colEnd := uint(17)
	loc := &simulator.SourceLocation{
		File:      sourcePath,
		Line:      3,
		Column:    13, // At 'x / 0'
		ColumnEnd: &colEnd,
	}

	// 3. Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 4. Run function
	displaySourceLocation(loc)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = oldStdout

	output := buf.String()

	// 5. Assert output contains highlighted token
	assert.Contains(t, output, "token.rs:3:13")
	assert.Contains(t, output, "3 |     let y = x / 0; // Error here")
	assert.Contains(t, output, "^^^^")
}
