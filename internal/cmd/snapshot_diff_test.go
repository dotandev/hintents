// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/snapshot"
)

func TestSnapshotDiffIdentical(t *testing.T) {
	tempDir := t.TempDir()

	memory := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	encodedMem := base64.StdEncoding.EncodeToString(memory)

	snapA := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{{"key", "val"}},
		LinearMemory:  encodedMem,
	}
	snapB := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{{"key", "val"}},
		LinearMemory:  encodedMem,
	}

	pathA := filepath.Join(tempDir, "a.json")
	pathB := filepath.Join(tempDir, "b.json")

	writeSnapshot(t, pathA, snapA)
	writeSnapshot(t, pathB, snapB)

	snapshotDiffAFlag = pathA
	snapshotDiffBFlag = pathB
	snapshotDiffOffsetFlag = -1
	snapshotDiffLengthFlag = 0
	snapshotDiffContextFlag = 16

	output := captureOutput(t, func() {
		err := runSnapshotDiff(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "identical") {
		t.Errorf("expected 'identical' in output, got: %s", output)
	}
}

func TestSnapshotDiffWithChanges(t *testing.T) {
	tempDir := t.TempDir()

	memoryA := []byte{0x00, 0x01, 0x02, 0x03}
	memoryB := []byte{0x00, 0xFF, 0x02, 0xAA}

	snapA := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{},
		LinearMemory:  base64.StdEncoding.EncodeToString(memoryA),
	}
	snapB := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{},
		LinearMemory:  base64.StdEncoding.EncodeToString(memoryB),
	}

	pathA := filepath.Join(tempDir, "a.json")
	pathB := filepath.Join(tempDir, "b.json")

	writeSnapshot(t, pathA, snapA)
	writeSnapshot(t, pathB, snapB)

	snapshotDiffAFlag = pathA
	snapshotDiffBFlag = pathB
	snapshotDiffOffsetFlag = -1
	snapshotDiffLengthFlag = 0
	snapshotDiffContextFlag = 16

	output := captureOutput(t, func() {
		err := runSnapshotDiff(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "changed") {
		t.Errorf("expected 'changed' in output, got: %s", output)
	}
	if !strings.Contains(output, "2") {
		t.Errorf("expected change count in output, got: %s", output)
	}
}

func TestSnapshotDiffMissingFileA(t *testing.T) {
	snapshotDiffAFlag = ""
	snapshotDiffBFlag = "b.json"

	err := runSnapshotDiff(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing snapshot-a flag")
	}
}

func TestSnapshotDiffMissingFileB(t *testing.T) {
	snapshotDiffAFlag = "a.json"
	snapshotDiffBFlag = ""

	err := runSnapshotDiff(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing snapshot-b flag")
	}
}

func TestSnapshotDiffNoMemory(t *testing.T) {
	tempDir := t.TempDir()

	snapA := &snapshot.Snapshot{LedgerEntries: []snapshot.LedgerEntryTuple{}}
	snapB := &snapshot.Snapshot{LedgerEntries: []snapshot.LedgerEntryTuple{}}

	pathA := filepath.Join(tempDir, "a.json")
	pathB := filepath.Join(tempDir, "b.json")

	writeSnapshot(t, pathA, snapA)
	writeSnapshot(t, pathB, snapB)

	snapshotDiffAFlag = pathA
	snapshotDiffBFlag = pathB
	snapshotDiffOffsetFlag = -1
	snapshotDiffLengthFlag = 0
	snapshotDiffContextFlag = 16

	output := captureOutput(t, func() {
		err := runSnapshotDiff(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "Neither snapshot contains linear memory") {
		t.Errorf("expected no memory message, got: %s", output)
	}
}

func TestSnapshotDiffWithOffset(t *testing.T) {
	tempDir := t.TempDir()

	memoryA := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	memoryB := []byte{0x00, 0x01, 0xFF, 0xFF, 0x04, 0x05, 0x06, 0x07}

	snapA := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{},
		LinearMemory:  base64.StdEncoding.EncodeToString(memoryA),
	}
	snapB := &snapshot.Snapshot{
		LedgerEntries: []snapshot.LedgerEntryTuple{},
		LinearMemory:  base64.StdEncoding.EncodeToString(memoryB),
	}

	pathA := filepath.Join(tempDir, "a.json")
	pathB := filepath.Join(tempDir, "b.json")

	writeSnapshot(t, pathA, snapA)
	writeSnapshot(t, pathB, snapB)

	snapshotDiffAFlag = pathA
	snapshotDiffBFlag = pathB
	snapshotDiffOffsetFlag = 2
	snapshotDiffLengthFlag = 4
	snapshotDiffContextFlag = 16

	output := captureOutput(t, func() {
		err := runSnapshotDiff(nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "0x00000002") {
		t.Errorf("expected offset 0x00000002 in output, got: %s", output)
	}
}

func TestFormatHexAscii(t *testing.T) {
	data := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00, 0x7f}
	hex, ascii := formatHexAscii(data)

	if !strings.HasPrefix(hex, "48 65 6c 6c 6f 00 7f") {
		t.Errorf("unexpected hex: %s", hex)
	}
	if !strings.HasPrefix(ascii, "Hello..") {
		t.Errorf("unexpected ascii: %s", ascii)
	}
}

func writeSnapshot(t *testing.T, path string, snap *snapshot.Snapshot) {
	t.Helper()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy error: %v", err)
	}
	return buf.String()
}
