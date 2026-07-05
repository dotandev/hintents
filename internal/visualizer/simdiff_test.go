// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiffLedgerEntries(t *testing.T) {
	before := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	after := map[string]string{
		"key1": "value1",     // unchanged
		"key2": "value2_new", // modified
		"key4": "value4",     // added
	}

	entries := DiffLedgerEntries(before, after)

	// Verify counts
	added, removed, modified, unchanged := 0, 0, 0, 0
	for _, e := range entries {
		switch e.ChangeKind {
		case ledgerAdded:
			added++
		case ledgerRemoved:
			removed++
		case ledgerModified:
			modified++
		case ledgerUnchanged:
			unchanged++
		}
	}

	if added != 1 {
		t.Errorf("expected 1 added entry, got %d", added)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed entry, got %d", removed)
	}
	if modified != 1 {
		t.Errorf("expected 1 modified entry, got %d", modified)
	}
	if unchanged != 1 {
		t.Errorf("expected 1 unchanged entry, got %d", unchanged)
	}
}

func TestTruncateLedgerValue(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		maxLen   int
		expected string
	}{
		{"empty", "", 10, "<none>"},
		{"short", "abc", 10, "abc"},
		{"exact", "1234567890", 10, "1234567890"},
		{"long", "12345678901", 10, "123456789…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateLedgerValue(tt.val, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateLedgerValue(%q, %d) = %q, want %q",
					tt.val, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestShortenLedgerKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"short", "abc", "abc"},
		{"exact", "1234567890123456789012345678901234567890", "1234567890123456789012345678901234567890"},
		{"long", "12345678901234567890123456789012345678901234567890", "12345678…34567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenLedgerKey(tt.key)
			if result != tt.expected {
				t.Errorf("shortenLedgerKey(%q) = %q, want %q",
					tt.key, result, tt.expected)
			}
		})
	}
}

func TestBuildLedgerStateDiffReport(t *testing.T) {
	before := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	after := map[string]string{
		"key1": "value1",     // unchanged
		"key2": "value2_new", // modified
		"key4": "value4",     // added
		// key3 removed
	}

	report := BuildLedgerStateDiffReport(before, after, false)

	if report.BeforeCount != 3 || report.AfterCount != 3 {
		t.Errorf("counts: before=%d after=%d, want 3/3", report.BeforeCount, report.AfterCount)
	}
	if report.Added != 1 || report.Removed != 1 || report.Modified != 1 || report.Unchanged != 1 {
		t.Errorf("summary: added=%d removed=%d modified=%d unchanged=%d, want 1/1/1/1",
			report.Added, report.Removed, report.Modified, report.Unchanged)
	}

	// includeUnchanged=false drops the unchanged entry but keeps the counters.
	if len(report.Entries) != 3 {
		t.Fatalf("expected 3 entries with includeUnchanged=false, got %d", len(report.Entries))
	}

	byKey := make(map[string]LedgerDiffEntryReport, len(report.Entries))
	for _, e := range report.Entries {
		byKey[e.Key] = e
	}

	added, ok := byKey["key4"]
	if !ok || added.Change != ChangeAdded {
		t.Fatalf("key4 should be added, got %+v", added)
	}
	if added.Before != nil {
		t.Errorf("added entry must have nil Before, got %q", *added.Before)
	}
	if added.After == nil || *added.After != "value4" {
		t.Errorf("added entry After = %v, want value4", added.After)
	}

	removed, ok := byKey["key3"]
	if !ok || removed.Change != ChangeRemoved {
		t.Fatalf("key3 should be removed, got %+v", removed)
	}
	if removed.After != nil {
		t.Errorf("removed entry must have nil After, got %q", *removed.After)
	}
	if removed.Before == nil || *removed.Before != "value3" {
		t.Errorf("removed entry Before = %v, want value3", removed.Before)
	}

	modified, ok := byKey["key2"]
	if !ok || modified.Change != ChangeModified {
		t.Fatalf("key2 should be modified, got %+v", modified)
	}
	if modified.Before == nil || *modified.Before != "value2" || modified.After == nil || *modified.After != "value2_new" {
		t.Errorf("modified entry = %+v, want before=value2 after=value2_new", modified)
	}
}

func TestBuildLedgerStateDiffReportIncludeUnchanged(t *testing.T) {
	before := map[string]string{"key1": "value1", "key2": "value2"}
	after := map[string]string{"key1": "value1", "key2": "value2_new"}

	report := BuildLedgerStateDiffReport(before, after, true)
	if len(report.Entries) != 2 {
		t.Fatalf("expected 2 entries with includeUnchanged=true, got %d", len(report.Entries))
	}

	var sawUnchanged bool
	for _, e := range report.Entries {
		if e.Change == ChangeUnchanged {
			sawUnchanged = true
			if e.Before == nil || e.After == nil || *e.Before != *e.After {
				t.Errorf("unchanged entry should have equal before/after, got %+v", e)
			}
		}
	}
	if !sawUnchanged {
		t.Error("expected an unchanged entry to be included")
	}
}

func TestRenderLedgerStateDiffJSON(t *testing.T) {
	before := map[string]string{"key1": "value1"}
	after := map[string]string{"key1": "value1_new", "key2": "value2"}

	var buf bytes.Buffer
	if err := RenderLedgerStateDiffJSON(&buf, before, after, false); err != nil {
		t.Fatalf("RenderLedgerStateDiffJSON returned error: %v", err)
	}

	// Output must be valid JSON deserializing back into the report shape.
	var decoded LedgerStateDiffReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if decoded.Added != 1 || decoded.Modified != 1 {
		t.Errorf("decoded summary added=%d modified=%d, want 1/1", decoded.Added, decoded.Modified)
	}
	if len(decoded.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(decoded.Entries))
	}

	// The JSON must be free of ANSI escape codes (headless integration).
	if strings.Contains(buf.String(), "\x1b[") {
		t.Error("JSON output should not contain ANSI escape codes")
	}
}

func TestRenderLedgerStateDiffToWritesToWriter(t *testing.T) {
	before := map[string]string{"key1": "value1"}
	after := map[string]string{"key1": "value1_changed"}

	var buf bytes.Buffer
	RenderLedgerStateDiffTo(&buf, before, after, false)

	out := buf.String()
	if out == "" {
		t.Fatal("expected rendered output, got empty string")
	}
	if !strings.Contains(out, "LEDGER STATE DIFF") {
		t.Errorf("rendered output missing header, got:\n%s", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Errorf("rendered output missing summary, got:\n%s", out)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		w        int
		expected string
	}{
		{"empty", "", 5, "     "},
		{"short", "ab", 5, "ab   "},
		{"exact", "12345", 5, "12345"},
		{"long", "123456", 5, "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padRight(tt.s, tt.w)
			if result != tt.expected {
				t.Errorf("padRight(%q, %d) = %q, want %q",
					tt.s, tt.w, result, tt.expected)
			}
		})
	}
}
