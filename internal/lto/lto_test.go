// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package lto

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// ParseLTOValue Tests
// =============================================================================

func TestParseLTOValue(t *testing.T) {
	tests := []struct {
		value string
		want  LTOKind
	}{
		{"false", LTONone},
		{"\"false\"", LTONone},
		{"off", LTONone},
		{"\"off\"", LTONone},
		{"", LTONone},
		{"true", LTOFat},
		{"\"true\"", LTOFat},
		{"fat", LTOFat},
		{"\"fat\"", LTOFat},
		{"thin", LTOThin},
		{"\"thin\"", LTOThin},
		{"  true  ", LTOFat},
		{"unknown", LTONone},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := ParseLTOValue(tt.value)
			if got != tt.want {
				t.Errorf("ParseLTOValue(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

// =============================================================================
// ParseCargoTomlContent Tests
// =============================================================================

func TestParseCargoTomlContent_FatLTO(t *testing.T) {
	content := `
[package]
name = "my-contract"
version = "0.1.0"

[profile.release]
lto = true
opt-level = "z"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Detected {
		t.Error("expected LTO to be detected")
	}
	if results[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", results[0].Severity)
	}
	if results[0].Profile.Name != "release" {
		t.Errorf("expected profile 'release', got %q", results[0].Profile.Name)
	}
}

func TestParseCargoTomlContent_ThinLTO(t *testing.T) {
	content := `
[profile.release]
lto = "thin"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", results[0].Severity)
	}
}

func TestParseCargoTomlContent_FatLTOString(t *testing.T) {
	content := `
[profile.release]
lto = "fat"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Severity != "error" {
		t.Errorf("expected severity 'error', got %q", results[0].Severity)
	}
}

func TestParseCargoTomlContent_NoLTO(t *testing.T) {
	content := `
[package]
name = "my-contract"

[profile.release]
opt-level = "z"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 0 {
		t.Errorf("expected 0 results for no LTO, got %d", len(results))
	}
}

func TestParseCargoTomlContent_LTOFalse(t *testing.T) {
	content := `
[profile.release]
lto = false
`
	results := ParseCargoTomlContent(content)
	if len(results) != 0 {
		t.Errorf("expected 0 results for lto = false, got %d", len(results))
	}
}

func TestParseCargoTomlContent_LTOOff(t *testing.T) {
	content := `
[profile.release]
lto = "off"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 0 {
		t.Errorf("expected 0 results for lto = off, got %d", len(results))
	}
}

func TestParseCargoTomlContent_MultipleProfiles(t *testing.T) {
	content := `
[profile.release]
lto = true

[profile.dev]
lto = false

[profile.bench]
lto = "thin"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First: release with fat LTO
	if results[0].Profile.Name != "release" {
		t.Errorf("first result profile = %q, want 'release'", results[0].Profile.Name)
	}
	if results[0].Severity != "error" {
		t.Errorf("first result severity = %q, want 'error'", results[0].Severity)
	}

	// Second: bench with thin LTO
	if results[1].Profile.Name != "bench" {
		t.Errorf("second result profile = %q, want 'bench'", results[1].Profile.Name)
	}
	if results[1].Severity != "warning" {
		t.Errorf("second result severity = %q, want 'warning'", results[1].Severity)
	}
}

func TestParseCargoTomlContent_CommentsIgnored(t *testing.T) {
	content := `
[profile.release]
# lto = true
opt-level = "z"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 0 {
		t.Errorf("expected 0 results when lto is commented out, got %d", len(results))
	}
}

func TestParseCargoTomlContent_NonProfileSection(t *testing.T) {
	content := `
[package]
name = "my-contract"

[dependencies]
soroban-sdk = "21.0"
`
	results := ParseCargoTomlContent(content)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-profile sections, got %d", len(results))
	}
}

func TestParseCargoTomlContent_RealWorldSorobanContract(t *testing.T) {
	content := `
[package]
name = "soroban-token-contract"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
soroban-sdk = "21.0"

[dev-dependencies]
soroban-sdk = { version = "21.0", features = ["testutils"] }

[profile.release]
opt-level = "z"
overflow-checks = true
debug = 0
strip = "symbols"
debug-assertions = false
panic = "abort"
codegen-units = 1
lto = true

[profile.release-with-logs]
inherits = "release"
debug-assertions = true
`
	results := ParseCargoTomlContent(content)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Profile.Name != "release" {
		t.Errorf("profile = %q, want 'release'", results[0].Profile.Name)
	}
	if results[0].Severity != "error" {
		t.Errorf("severity = %q, want 'error'", results[0].Severity)
	}
}

// =============================================================================
// CheckCargoToml File Tests
// =============================================================================

func TestCheckCargoToml_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Cargo.toml")
	content := `
[profile.release]
lto = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	results, err := CheckCargoToml(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestCheckCargoToml_FileNotFound(t *testing.T) {
	_, err := CheckCargoToml("/nonexistent/Cargo.toml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// =============================================================================
// CheckProjectDir Tests
// =============================================================================

func TestCheckProjectDir(t *testing.T) {
	dir := t.TempDir()

	// Root Cargo.toml with LTO
	rootContent := `
[profile.release]
lto = true
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(rootContent), 0644); err != nil {
		t.Fatalf("failed to write root Cargo.toml: %v", err)
	}

	// Sub-project with thin LTO
	subDir := filepath.Join(dir, "contracts", "token")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	// Only direct children are checked, not nested
	subContent := `
[profile.release]
lto = "thin"
`
	directSubDir := filepath.Join(dir, "token")
	if err := os.MkdirAll(directSubDir, 0755); err != nil {
		t.Fatalf("failed to create direct subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directSubDir, "Cargo.toml"), []byte(subContent), 0644); err != nil {
		t.Fatalf("failed to write sub Cargo.toml: %v", err)
	}

	results, err := CheckProjectDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (root + sub), got %d", len(results))
	}
}

func TestCheckProjectDir_NoCargoFiles(t *testing.T) {
	dir := t.TempDir()

	results, err := CheckProjectDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// =============================================================================
// FormatWarning Tests
// =============================================================================

func TestFormatWarning_FatLTO(t *testing.T) {
	result := DetectionResult{
		Detected: true,
		Profile:  Profile{Name: "release", LTO: "true"},
		Severity: "error",
		Message:  "Profile [release] has lto = true (fat LTO).",
	}

	warning := FormatWarning(result)
	if warning == "" {
		t.Fatal("expected non-empty warning")
	}
	if !containsStr(warning, "WARNING: LTO") {
		t.Error("expected WARNING header for fat LTO")
	}
	if !containsStr(warning, "lto = false") {
		t.Error("expected fix suggestion")
	}
}

func TestFormatWarning_ThinLTO(t *testing.T) {
	result := DetectionResult{
		Detected: true,
		Profile:  Profile{Name: "release", LTO: "\"thin\""},
		Severity: "warning",
		Message:  "Profile [release] has lto = thin.",
	}

	warning := FormatWarning(result)
	if !containsStr(warning, "Note: Thin LTO") {
		t.Error("expected Note header for thin LTO")
	}
}

func TestFormatWarnings_Empty(t *testing.T) {
	output := FormatWarnings(nil)
	if output != "" {
		t.Errorf("expected empty string, got %q", output)
	}
}

func TestFormatWarnings_Multiple(t *testing.T) {
	results := []DetectionResult{
		{
			Detected: true,
			Profile:  Profile{Name: "release", LTO: "true"},
			Severity: "error",
			Message:  "first",
		},
		{
			Detected: true,
			Profile:  Profile{Name: "bench", LTO: "\"thin\""},
			Severity: "warning",
			Message:  "second",
		},
	}

	output := FormatWarnings(results)
	if !containsStr(output, "first") {
		t.Error("expected first message")
	}
	if !containsStr(output, "second") {
		t.Error("expected second message")
	}
}

// =============================================================================
// HasLTO / HasFatLTO Tests
// =============================================================================

func TestHasLTO(t *testing.T) {
	tests := []struct {
		name    string
		results []DetectionResult
		want    bool
	}{
		{"nil results", nil, false},
		{"empty results", []DetectionResult{}, false},
		{"detected", []DetectionResult{{Detected: true}}, true},
		{"not detected", []DetectionResult{{Detected: false}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasLTO(tt.results); got != tt.want {
				t.Errorf("HasLTO() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasFatLTO(t *testing.T) {
	tests := []struct {
		name    string
		results []DetectionResult
		want    bool
	}{
		{"nil results", nil, false},
		{"thin only", []DetectionResult{{Detected: true, Severity: "warning"}}, false},
		{"fat LTO", []DetectionResult{{Detected: true, Severity: "error"}}, true},
		{"mixed", []DetectionResult{
			{Detected: true, Severity: "warning"},
			{Detected: true, Severity: "error"},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasFatLTO(tt.results); got != tt.want {
				t.Errorf("HasFatLTO() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
