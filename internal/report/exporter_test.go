// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExporterCreation(t *testing.T) {
	tmpDir := t.TempDir()

	exporter, err := NewExporter(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating exporter: %v", err)
	}

	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}

	if exporter.outputDir != tmpDir {
		t.Errorf("expected output dir %q, got %q", tmpDir, exporter.outputDir)
	}
}

func TestHTMLExport(t *testing.T) {
	tmpDir := t.TempDir()
	exporter, _ := NewExporter(tmpDir)

	report := NewBuilder("Test Report").
		WithTransactionHash("0xhtml").
		SetSummary("success", "50ms", 5, 0, 0, 100.0).
		Build()

	path, err := exporter.Export(report, "html")
	if err != nil {
		t.Fatalf("unexpected error exporting HTML: %v", err)
	}

	if path == "" {
		t.Error("expected non-empty path")
	}

	if !strings.HasSuffix(path, ".html") {
		t.Errorf("expected .html extension, got %q", path)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("exported file not found: %v", err)
	}
}

func TestPDFExport(t *testing.T) {
	tmpDir := t.TempDir()
	exporter, _ := NewExporter(tmpDir)

	report := NewBuilder("Test Report").
		WithTransactionHash("0xpdf").
		SetSummary("success", "75ms", 8, 1, 0, 98.5).
		Build()

	path, err := exporter.Export(report, "pdf")
	if err != nil {
		t.Fatalf("unexpected error exporting PDF: %v", err)
	}

	if !strings.HasSuffix(path, ".pdf") {
		t.Errorf("expected .pdf extension, got %q", path)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("exported file not found: %v", err)
	}
}

func TestJSONExportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	exporter, _ := NewExporter(tmpDir)

	report := NewBuilder("Test Report").
		WithTransactionHash("0xjson").
		SetSummary("success", "60ms", 10, 2, 0, 97.5).
		Build()

	path, err := exporter.Export(report, "json")
	if err != nil {
		t.Fatalf("unexpected error exporting JSON: %v", err)
	}

	if !strings.HasSuffix(path, ".json") {
		t.Errorf("expected .json extension, got %q", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read exported file: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON file")
	}

	if !strings.Contains(string(data), "0xjson") {
		t.Error("expected transaction hash in exported JSON")
	}
}

func TestMultipleFormatsExport(t *testing.T) {
	tmpDir := t.TempDir()
	exporter, _ := NewExporter(tmpDir)

	report := NewBuilder("Test Report").
		WithTransactionHash("0xmulti").
		SetSummary("success", "80ms", 12, 1, 0, 98.0).
		Build()

	results, err := exporter.ExportMultiple(report, []string{"html", "json", "pdf"})
	if err != nil {
		t.Fatalf("unexpected error exporting multiple: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 exported formats, got %d", len(results))
	}

	if _, ok := results["html"]; !ok {
		t.Error("expected HTML export path in results")
	}

	if _, ok := results["json"]; !ok {
		t.Error("expected JSON export path in results")
	}

	if _, ok := results["pdf"]; !ok {
		t.Error("expected PDF export path in results")
	}
}

func TestFilenameGeneration(t *testing.T) {
	filename := generateFilename("My Test Report", "html")

	if !strings.HasSuffix(filename, ".html") {
		t.Errorf("expected .html extension, got %q", filename)
	}

	if !strings.Contains(filename, "my_test_report") {
		t.Errorf("expected sanitized title in filename, got %q", filename)
	}

	if !strings.Contains(filename, "-") {
		t.Error("expected timestamp separator in filename")
	}
}

func TestInvalidOutputDir(t *testing.T) {
	invalidDir := `C:\INVALID|PATH` // Illegal characters on Windows

	_, err := NewExporter(invalidDir)
	if err == nil {
		t.Error("expected error creating exporter with invalid path")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Title", "normal_title"},
		{"Title With @#$%", "title_with_"},
		{"UPPERCASE", "uppercase"},
		{"Multiple___Underscores", "multiple_underscores"},
		{"123 Numbers 456", "123_numbers_456"},
	}

	for _, test := range tests {
		result := sanitizeFilename(test.input)
		if result != test.expected {
			t.Errorf("sanitizeFilename(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestExportDirectoryCreation(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "output", "dir")

	exporter, err := NewExporter(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(tmpDir); err != nil {
		t.Errorf("output directory not created: %v", err)
	}

	if exporter.outputDir != tmpDir {
		t.Errorf("expected output dir %q, got %q", tmpDir, exporter.outputDir)
	}
}

func TestUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	exporter, _ := NewExporter(tmpDir)

	report := NewBuilder("Test Report").Build()

	_, err := exporter.Export(report, "unsupported")
	if err == nil {
		t.Error("expected error for unsupported format")
	}

	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected unsupported format error, got: %v", err)
	}
}

func TestLongFilenameHandling(t *testing.T) {
	longTitle := strings.Repeat("A", 100)
	filename := generateFilename(longTitle, "html")

	if len(filename) > 75 {
		t.Errorf("filename too long: %d chars, expected <= 75", len(filename))
	}
}
