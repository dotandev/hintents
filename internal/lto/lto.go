// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package lto detects Link Time Optimization (LTO) settings in Soroban
// contract projects and warns users about offset inaccuracies in DWARF
// source mappings.
//
// When a Rust contract is compiled with lto = true or lto = "fat", the
// compiler merges and re-optimises all codegen units during linking. This
// destructures predictable DWARF mappings, causing WASM instruction
// offsets reported in stack traces and source maps to point to incorrect
// source lines.
//
// This package provides:
//   - Cargo.toml/Cargo profile parsing to detect LTO settings
//   - WASM binary heuristic detection of LTO artifacts
//   - A warning message formatter for CLI output
package lto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Profile represents the build profile settings relevant to LTO detection.
type Profile struct {
	// Name is the profile name (e.g. "release", "dev").
	Name string
	// LTO is the raw lto value from Cargo.toml.
	LTO string
	// CodegenUnits is the codegen-units setting, if present.
	CodegenUnits *int
	// OptLevel is the opt-level setting, if present.
	OptLevel *int
}

// DetectionResult holds the outcome of an LTO check.
type DetectionResult struct {
	// Detected is true if LTO is enabled.
	Detected bool
	// Profile is the profile where LTO was found.
	Profile Profile
	// Severity is "warning" for thin LTO, "error" for fat LTO.
	Severity string
	// Message is a human-readable explanation.
	Message string
}

// LTOKind classifies the type of LTO detected.
type LTOKind int

const (
	LTONone LTOKind = iota
	LTOThin
	LTOFat
)

// ParseLTOValue interprets a Cargo.toml lto value and returns the LTO kind.
//
// Valid values in Cargo.toml:
//   - false, "off"  -> no LTO
//   - true, "fat"   -> fat LTO (worst for DWARF)
//   - "thin"        -> thin LTO (partially destructive)
func ParseLTOValue(value string) LTOKind {
	v := strings.TrimSpace(strings.ToLower(value))
	v = strings.Trim(v, "\"'")

	switch v {
	case "false", "off", "":
		return LTONone
	case "true", "fat":
		return LTOFat
	case "thin":
		return LTOThin
	default:
		return LTONone
	}
}

// CheckCargoToml scans a Cargo.toml file for LTO settings in any profile.
// Returns a list of detection results (one per profile with LTO enabled).
func CheckCargoToml(path string) ([]DetectionResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	return ParseCargoTomlContent(string(data)), nil
}

// CheckProjectDir scans a project directory for Cargo.toml files and
// checks all of them for LTO settings. It checks the root Cargo.toml
// and any workspace member Cargo.toml files.
func CheckProjectDir(dir string) ([]DetectionResult, error) {
	var results []DetectionResult

	rootToml := filepath.Join(dir, "Cargo.toml")
	if _, err := os.Stat(rootToml); err == nil {
		res, err := CheckCargoToml(rootToml)
		if err != nil {
			return nil, err
		}
		results = append(results, res...)
	}

	// Check common Soroban contract subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return results, nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subToml := filepath.Join(dir, entry.Name(), "Cargo.toml")
		if _, err := os.Stat(subToml); err == nil {
			res, err := CheckCargoToml(subToml)
			if err != nil {
				continue
			}
			for i := range res {
				res[i].Message = fmt.Sprintf("[%s] %s", entry.Name(), res[i].Message)
			}
			results = append(results, res...)
		}
	}

	return results, nil
}

// ParseCargoTomlContent parses raw Cargo.toml text and detects LTO settings.
// This is a lightweight parser that handles the common patterns without
// pulling in a full TOML library.
func ParseCargoTomlContent(content string) []DetectionResult {
	var results []DetectionResult

	lines := strings.Split(content, "\n")
	currentProfile := ""
	inProfile := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect profile section headers
		if strings.HasPrefix(trimmed, "[") {
			inProfile = false
			currentProfile = ""

			if strings.HasPrefix(trimmed, "[profile.") {
				// e.g. [profile.release]
				name := strings.TrimPrefix(trimmed, "[profile.")
				name = strings.TrimSuffix(name, "]")
				name = strings.TrimSpace(name)
				currentProfile = name
				inProfile = true
			}
			continue
		}

		// Parse key-value pairs within a profile section
		if inProfile && strings.Contains(trimmed, "=") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if key == "lto" {
				kind := ParseLTOValue(value)
				if kind != LTONone {
					result := DetectionResult{
						Detected: true,
						Profile: Profile{
							Name: currentProfile,
							LTO:  value,
						},
					}

					switch kind {
					case LTOFat:
						result.Severity = "error"
						result.Message = fmt.Sprintf(
							"Profile [%s] has lto = %s (fat LTO). "+
								"This will produce incorrect DWARF source mappings. "+
								"Set lto = false or remove the lto setting for accurate source-level debugging.",
							currentProfile, value,
						)
					case LTOThin:
						result.Severity = "warning"
						result.Message = fmt.Sprintf(
							"Profile [%s] has lto = %s. "+
								"Thin LTO may produce inaccurate DWARF source mappings. "+
								"Consider setting lto = false for reliable source-level debugging.",
							currentProfile, value,
						)
					default:
						continue
					}

					results = append(results, result)
				}
			}
		}
	}

	return results
}

// FormatWarning produces a CLI-ready warning message for an LTO detection result.
func FormatWarning(result DetectionResult) string {
	var b strings.Builder

	if result.Severity == "error" {
		b.WriteString("WARNING: LTO (Link Time Optimization) detected.\n")
	} else {
		b.WriteString("Note: Thin LTO detected.\n")
	}

	b.WriteString("  ")
	b.WriteString(result.Message)
	b.WriteString("\n\n")

	b.WriteString("  LTO destructures predictable DWARF debug mappings, causing\n")
	b.WriteString("  WASM instruction offsets in stack traces and source maps to\n")
	b.WriteString("  point to incorrect source lines.\n\n")

	b.WriteString("  To fix: in your Cargo.toml, set:\n")
	b.WriteString("    [profile.release]\n")
	b.WriteString("    lto = false\n")

	return b.String()
}

// FormatWarnings produces a combined warning for multiple detection results.
func FormatWarnings(results []DetectionResult) string {
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(FormatWarning(r))
	}
	return b.String()
}

// HasLTO returns true if any of the detection results indicate LTO is enabled.
func HasLTO(results []DetectionResult) bool {
	for _, r := range results {
		if r.Detected {
			return true
		}
	}
	return false
}

// HasFatLTO returns true if any result indicates fat LTO specifically.
func HasFatLTO(results []DetectionResult) bool {
	for _, r := range results {
		if r.Detected && r.Severity == "error" {
			return true
		}
	}
	return false
}
