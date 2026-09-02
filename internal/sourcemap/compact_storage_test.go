// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"bytes"
	"encoding/json"
	"testing"
)

// BenchmarkCompactStorage benchmarks the compact storage format against JSON.
func BenchmarkCompactStorage(b *testing.B) {
	// Create test data mimicking a complex contract with thousands of source mappings
	mappings := generateTestMappings(10000)
	files := generateTestFiles(100)

	b.Run("JSON_Serialization", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, _ := json.Marshal(struct {
				Mappings []SourceMapping `json:"mappings"`
				Files    []string        `json:"files"`
			}{
				Mappings: mappings,
				Files:    files,
			})
			// Use the data to prevent optimization
			_ = len(data)
		}
	})

	b.Run("Compact_Uncompressed", func(b *testing.B) {
		csm := NewCompactSourceMap(mappings, files)
		buf := new(bytes.Buffer)
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = csm.serialize(buf)
		}
	})

	b.Run("Compact_Compressed", func(b *testing.B) {
		csm := NewCompactSourceMap(mappings, files)
		buf := new(bytes.Buffer)
		for i := 0; i < b.N; i++ {
			buf.Reset()
			_ = csm.serializeCompressed(buf)
		}
	})
}

// TestCompactStorageSizeReduction verifies the target 30% size reduction.
func TestCompactStorageSizeReduction(t *testing.T) {
	// Test with various sizes to ensure consistent reduction
	testCases := []struct {
		name       string
		mappings   int
		files      int
		minPercent float64 // Minimum reduction percentage
	}{
		{"Small_Contract", 1000, 50, 25},
		{"Medium_Contract", 10000, 100, 30},
		{"Large_Contract", 50000, 200, 30},
		{"Complex_Contract", 100000, 500, 35},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mappings := generateTestMappings(tc.mappings)
			files := generateTestFiles(tc.files)

			// Measure JSON size
			jsonData, err := json.Marshal(struct {
				Mappings []SourceMapping `json:"mappings"`
				Files    []string        `json:"files"`
			}{
				Mappings: mappings,
				Files:    files,
			})
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}
			jsonSize := len(jsonData)

			// Measure compact uncompressed size
			csm := NewCompactSourceMap(mappings, files)
			var compactBuf bytes.Buffer
			if err := csm.serialize(&compactBuf); err != nil {
				t.Fatalf("Failed to serialize compact: %v", err)
			}
			compactSize := compactBuf.Len()

			// Measure compact compressed size
			var compressedBuf bytes.Buffer
			if err := csm.serializeCompressed(&compressedBuf); err != nil {
				t.Fatalf("Failed to serialize compressed: %v", err)
			}
			compressedSize := compressedBuf.Len()

			// Calculate reduction ratios
			compactReduction := 1.0 - (float64(compactSize) / float64(jsonSize))
			compressedReduction := 1.0 - (float64(compressedSize) / float64(jsonSize))

			t.Logf("Mappings: %d, Files: %d", tc.mappings, tc.files)
			t.Logf("JSON size: %d bytes", jsonSize)
			t.Logf("Compact (uncompressed) size: %d bytes (%.1f%% reduction)", compactSize, compactReduction*100)
			t.Logf("Compact (compressed) size: %d bytes (%.1f%% reduction)", compressedSize, compressedReduction*100)

			// Verify we meet the minimum reduction target
			if compactReduction < tc.minPercent/100 {
				t.Errorf("Compact storage reduction %.1f%% is below target %.0f%%",
					compactReduction*100, tc.minPercent)
			}
		})
	}
}

// TestCompactStorageRoundTrip verifies serialization and deserialization work correctly.
func TestCompactStorageRoundTrip(t *testing.T) {
	mappings := generateTestMappings(5000)
	files := generateTestFiles(50)

	csm := NewCompactSourceMap(mappings, files)

	t.Run("Uncompressed", func(t *testing.T) {
		var buf bytes.Buffer
		if err := csm.serialize(&buf); err != nil {
			t.Fatalf("Failed to serialize: %v", err)
		}

		deserialized, err := Deserialize(&buf)
		if err != nil {
			t.Fatalf("Failed to deserialize: %v", err)
		}

		verifyRoundTrip(t, csm, deserialized)
	})

	t.Run("Compressed", func(t *testing.T) {
		var buf bytes.Buffer
		if err := csm.serializeCompressed(&buf); err != nil {
			t.Fatalf("Failed to serialize compressed: %v", err)
		}

		deserialized, err := Deserialize(&buf)
		if err != nil {
			t.Fatalf("Failed to deserialize: %v", err)
		}

		verifyRoundTrip(t, csm, deserialized)
	})
}

// TestGetSourceLocation tests the binary search lookup.
func TestGetSourceLocation(t *testing.T) {
	mappings := []SourceMapping{
		{0, 1, 5, 0},
		{100, 2, 10, 0},
		{200, 3, 15, 1},
		{300, 4, 20, 1},
		{400, 5, 25, 2},
	}
	files := []string{"file1.rs", "file2.rs", "file3.rs"}

	csm := NewCompactSourceMap(mappings, files)

	tests := []struct {
		wasmOffset uint64
		wantFile   string
		wantLine   int
		wantFound  bool
	}{
		{0, "file1.rs", 1, true},
		{50, "file1.rs", 1, true}, // Between 0 and 100, should return first
		{100, "file2.rs", 2, true},
		{150, "file2.rs", 2, true}, // Between 100 and 200
		{200, "file3.rs", 3, true},
		{300, "file4.rs", 4, false}, // Unknown file index
		{500, "", 0, false},         // Beyond all mappings
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			file, line, _, found := csm.GetSourceLocation(tt.wasmOffset)
			if found != tt.wantFound {
				t.Errorf("GetSourceLocation(%d) found=%v, want found=%v", tt.wasmOffset, found, tt.wantFound)
			}
			if found && tt.wantFound {
				if file != tt.wantFile {
					t.Errorf("GetSourceLocation(%d) file=%s, want file=%s", tt.wasmOffset, file, tt.wantFile)
				}
				if line != tt.wantLine {
					t.Errorf("GetSourceLocation(%d) line=%d, want line=%d", tt.wasmOffset, line, tt.wantLine)
				}
			}
		})
	}
}

// TestInternFilePaths tests the file interning functionality.
func TestInternFilePaths(t *testing.T) {
	paths := []string{
		"src/lib.rs",
		"src/contract.rs",
		"src/lib.rs",      // Duplicate
		"src\\lib.rs",     // Windows-style separator (should be normalized)
		"src/contract.rs", // Duplicate
	}

	interned, mapping := InternFilePaths(paths)

	// Should have 2 unique paths
	if len(interned) != 2 {
		t.Errorf("Expected 2 interned paths, got %d", len(interned))
	}

	// Check that duplicates map to the same index
	if mapping["src/lib.rs"] != mapping["src\\lib.rs"] {
		t.Error("Windows-style path should map to same index as Unix-style")
	}
}

// TestBuildMappingFromDWARF tests the DWARF to compact mapping conversion.
func TestBuildMappingFromDWARF(t *testing.T) {
	entries := []DWARFLineEntry{
		{0, "main.rs", 1, 0},
		{10, "main.rs", 2, 5},
		{20, "lib.rs", 10, 3},
		{30, "lib.rs", 11, 7},
	}

	files := []string{"main.rs", "lib.rs"}

	mappings := BuildMappingFromDWARF(entries, files)

	if len(mappings) != 4 {
		t.Errorf("Expected 4 mappings, got %d", len(mappings))
	}

	// Verify first mapping
	if mappings[0].WasmOffset != 0 || mappings[0].Line != 1 || mappings[0].FileIndex != 0 {
		t.Errorf("First mapping incorrect: %+v", mappings[0])
	}

	// Verify mappings are sorted by address
	for i := 1; i < len(mappings); i++ {
		if mappings[i].WasmOffset <= mappings[i-1].WasmOffset {
			t.Errorf("Mappings not sorted: %d vs %d", mappings[i-1].WasmOffset, mappings[i].WasmOffset)
		}
	}
}

// verifyRoundTrip checks that deserialized data matches original.
func verifyRoundTrip(t *testing.T, original, deserialized *CompactSourceMap) {
	if len(original.Files) != len(deserialized.Files) {
		t.Errorf("Files count mismatch: %d vs %d", len(original.Files), len(deserialized.Files))
	}

	for i, f := range original.Files {
		if deserialized.Files[i] != f {
			t.Errorf("File %d mismatch: %s vs %s", i, f, deserialized.Files[i])
		}
	}

	if len(original.Mappings) != len(deserialized.Mappings) {
		t.Errorf("Mappings count mismatch: %d vs %d", len(original.Mappings), len(deserialized.Mappings))
	}

	for i, m := range original.Mappings {
		dm := deserialized.Mappings[i]
		if m.WasmOffset != dm.WasmOffset || m.Line != dm.Line || m.Column != dm.Column || m.FileIndex != dm.FileIndex {
			t.Errorf("Mapping %d mismatch: %+v vs %+v", i, m, dm)
		}
	}
}

// generateTestMappings creates test mappings with realistic distribution.
func generateTestMappings(count int) []SourceMapping {
	mappings := make([]SourceMapping, count)
	offset := uint64(0)
	line := uint32(1)

	// Simulate typical source mapping distribution
	// Addresses increment by varying amounts
	// Lines increment by 1-5 typically
	// Files cycle through a subset
	for i := 0; i < count; i++ {
		offset += uint64(1 + (i % 50)) // Varying instruction spacing
		line += uint32(1 + (i % 3))    // Mostly line increments of 1-3

		mappings[i] = SourceMapping{
			WasmOffset: offset,
			Line:       line,
			Column:     uint32(i % 80),
			FileIndex:  uint32(i % 20), // Cycle through 20 files
		}
	}

	return mappings
}

// generateTestFiles creates test file paths.
func generateTestFiles(count int) []string {
	files := make([]string, count)
	dirs := []string{"src", "lib", "contracts", "modules", "utils"}

	for i := 0; i < count; i++ {
		dir := dirs[i%len(dirs)]
		files[i] = dir + "/module_" + string(rune('a'+i%26)) + ".rs"
	}

	return files
}

// BenchmarkGetSourceLocation benchmarks the binary search lookup.
func BenchmarkGetSourceLocation(b *testing.B) {
	mappings := generateTestMappings(100000)
	files := generateTestFiles(100)
	csm := NewCompactSourceMap(mappings, files)

	// Generate random offsets to search for
	offsets := make([]uint64, 1000)
	for i := range offsets {
		offsets[i] = uint64(i * 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, off := range offsets {
			_, _, _, _ = csm.GetSourceLocation(off)
		}
	}
}
