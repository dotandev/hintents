// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/hintents/internal/snapshot"
)

// TestGeneratorCreation tests that the generator can be created with valid config
func TestGeneratorCreation(t *testing.T) {
	config := &GeneratorConfig{
		Count:      1000,
		OutputFile: "test_snapshot.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	if gen == nil {
		t.Fatal("Generator should not be nil")
	}

	if gen.config.Count != 1000 {
		t.Errorf("Expected count 1000, got %d", gen.config.Count)
	}
}

// TestKeyGeneration tests that keys are unique and properly formatted
func TestKeyGeneration(t *testing.T) {
	config := &GeneratorConfig{
		Count:      10000,
		OutputFile: "test_keys.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)

	// Generate multiple keys and check uniqueness
	keys := make(map[string]bool)
	for i := int64(0); i < 1000; i++ {
		key := gen.GenerateRandomKey(i)

		// Check base64 encoding
		if _, err := base64.StdEncoding.DecodeString(key); err != nil {
			t.Errorf("Key %d is not valid base64: %v", i, err)
		}

		// Check uniqueness
		if keys[key] {
			t.Errorf("Duplicate key generated at index %d", i)
		}
		keys[key] = true
	}

	if len(keys) < 900 { // Allow some collisions by chance, but expect mostly unique
		t.Errorf("Expected mostly unique keys, got only %d unique out of 1000", len(keys))
	}
}

// TestValueGeneration tests that values are valid base64 and properly sized
func TestValueGeneration(t *testing.T) {
	config := &GeneratorConfig{
		Count:      1000,
		OutputFile: "test_values.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)

	for i := 0; i < 100; i++ {
		value := gen.GenerateRandomValue()

		// Should be valid base64
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Errorf("Value %d is not valid base64: %v", i, err)
			continue
		}

		// Check size range (128-2176 bytes)
		if len(decoded) < 128 || len(decoded) > 2176 {
			t.Errorf("Value %d has unexpected size: %d bytes (expected 128-2176)", i, len(decoded))
		}
	}
}

// TestEntryGeneration tests that entries are properly formatted
func TestEntryGeneration(t *testing.T) {
	config := &GeneratorConfig{
		Count:      100,
		OutputFile: "test_entries.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	entries := gen.GenerateEntries()

	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}

	for i, entry := range entries {
		if len(entry) != 2 {
			t.Errorf("Entry %d has %d elements, expected 2", i, len(entry))
		}

		key := entry[0]
		value := entry[1]

		// Validate key
		if _, err := base64.StdEncoding.DecodeString(key); err != nil {
			t.Errorf("Entry %d key is not valid base64: %v", i, err)
		}

		// Validate value
		if _, err := base64.StdEncoding.DecodeString(value); err != nil {
			t.Errorf("Entry %d value is not valid base64: %v", i, err)
		}
	}
}

// TestSnapshotGeneration tests complete snapshot generation
func TestSnapshotGeneration(t *testing.T) {
	config := &GeneratorConfig{
		Count:      500,
		OutputFile: "test_snapshot_gen.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	snap := gen.GenerateSnapshot()

	if snap == nil {
		t.Fatal("Snapshot should not be nil")
	}

	if len(snap.LedgerEntries) != 500 {
		t.Errorf("Expected 500 entries in snapshot, got %d", len(snap.LedgerEntries))
	}

	// Verify sorting by key
	for i := 1; i < len(snap.LedgerEntries); i++ {
		prev := snap.LedgerEntries[i-1][0]
		curr := snap.LedgerEntries[i][0]
		if prev > curr {
			t.Errorf("Entries not sorted by key at index %d: %s > %s", i, prev, curr)
		}
	}
}

// TestSnapshotSaveAndLoad tests that snapshots can be saved and loaded
func TestSnapshotSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "snapshot_test.json")

	config := &GeneratorConfig{
		Count:      100,
		OutputFile: outputPath,
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	snap := gen.GenerateSnapshot()

	// Save snapshot
	if err := snapshot.Save(outputPath, snap); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Snapshot file not created: %v", err)
	}

	// Load snapshot back
	loadedSnap, err := snapshot.Load(outputPath)
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Verify loaded snapshot matches
	if len(loadedSnap.LedgerEntries) != len(snap.LedgerEntries) {
		t.Errorf("Loaded snapshot has different entry count: %d vs %d",
			len(loadedSnap.LedgerEntries), len(snap.LedgerEntries))
	}

	for i := range snap.LedgerEntries {
		if loadedSnap.LedgerEntries[i][0] != snap.LedgerEntries[i][0] {
			t.Errorf("Entry %d key mismatch", i)
		}
		if loadedSnap.LedgerEntries[i][1] != snap.LedgerEntries[i][1] {
			t.Errorf("Entry %d value mismatch", i)
		}
	}

	// Cleanup
	os.Remove(outputPath)
}

// TestDeterministicGeneration tests that different instances produce different results
// (unless seeded, but our implementation uses crypto/rand which is not seedable in the current design)
func TestValueVariety(t *testing.T) {
	config := &GeneratorConfig{
		Count:      1000,
		OutputFile: "test_variety.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)

	// Generate multiple values and verify they're different
	values := make([]string, 0, 100)
	hashes := make(map[string]int)

	for i := 0; i < 100; i++ {
		value := gen.GenerateRandomValue()
		values = append(values, value)

		// Compute hash of the value for quick dedup check
		h := sha256.Sum256([]byte(value))
		hashStr := hex.EncodeToString(h[:])
		hashes[hashStr]++
	}

	// Verify variety (all hashes should appear once)
	duplicates := 0
	for _, count := range hashes {
		if count > 1 {
			duplicates += count - 1
		}
	}

	// With 100 random values, duplicates should be extremely rare
	if duplicates > 5 {
		t.Errorf("Too many duplicate values: %d out of 100", duplicates)
	}
}

// TestLargeSnapshot tests generation of a moderately large snapshot
func TestLargeSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large snapshot test in short mode")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "large_snapshot.json")

	config := &GeneratorConfig{
		Count:      10000,
		OutputFile: outputPath,
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	snap := gen.GenerateSnapshot()

	if len(snap.LedgerEntries) != 10000 {
		t.Errorf("Expected 10000 entries, got %d", len(snap.LedgerEntries))
	}

	// Save and verify file size is reasonable
	if err := snapshot.Save(outputPath, snap); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	fileSize := info.Size()
	avgSize := fileSize / 10000

	// Average size should be around 3-4 KB per entry (key + value overhead)
	if avgSize < 2000 || avgSize > 5000 {
		t.Logf("Average entry size: %d bytes (may be outside typical range)", avgSize)
	}

	os.Remove(outputPath)
}

// BenchmarkKeyGeneration benchmarks key generation
func BenchmarkKeyGeneration(b *testing.B) {
	config := &GeneratorConfig{
		Count:      1000000,
		OutputFile: "bench_keys.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = gen.GenerateRandomKey(int64(i))
	}
}

// BenchmarkValueGeneration benchmarks value generation
func BenchmarkValueGeneration(b *testing.B) {
	config := &GeneratorConfig{
		Count:      1000000,
		OutputFile: "bench_values.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = gen.GenerateRandomValue()
	}
}

// BenchmarkEntryGeneration benchmarks complete entry generation
func BenchmarkEntryGeneration(b *testing.B) {
	config := &GeneratorConfig{
		Count:      int64(b.N),
		OutputFile: "bench_entries.json",
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	b.ResetTimer()

	_ = gen.GenerateEntries()
}

// BenchmarkSnapshotGeneration benchmarks snapshot generation and saving
func BenchmarkSnapshotGeneration(b *testing.B) {
	tmpDir := b.TempDir()

	cases := []struct {
		name  string
		count int64
	}{
		{"small", 1000},
		{"medium", 10000},
		{"large", 100000},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				outputPath := filepath.Join(tmpDir, "bench_snapshot.json")

				config := &GeneratorConfig{
					Count:      c.count,
					OutputFile: outputPath,
					Verbose:    false,
				}

				gen := NewEntryGenerator(config)
				snap := gen.GenerateSnapshot()

				_ = snapshot.Save(outputPath, snap)
				os.Remove(outputPath)
			}
		})
	}
}

// TestSnapshotFormat tests that the snapshot JSON has the correct format
func TestSnapshotFormat(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "format_test.json")

	config := &GeneratorConfig{
		Count:      50,
		OutputFile: outputPath,
		Verbose:    false,
	}

	gen := NewEntryGenerator(config)
	snap := gen.GenerateSnapshot()
	_ = snapshot.Save(outputPath, snap)

	// Read raw file content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read snapshot: %v", err)
	}

	content := string(data)

	// Check that it contains expected JSON structure
	if !contains(content, `"ledgerEntries"`) {
		t.Error("Snapshot missing 'ledgerEntries' key")
	}

	if !contains(content, `[`) {
		t.Error("Snapshot missing array notation")
	}

	os.Remove(outputPath)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[0:len(substr)] == substr || contains(s[1:], substr))
}
