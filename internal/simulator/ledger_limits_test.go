// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// makeEntry returns a base64-encoded string of n zero bytes, simulating an
// XDR-encoded ledger key or entry of the given decoded size.
func makeEntry(n int) string {
	return base64.StdEncoding.EncodeToString(make([]byte, n))
}

func TestCheckLedgerEntriesSize_UnderLimit(t *testing.T) {
	entries := map[string]string{
		makeEntry(64):  makeEntry(512),
		makeEntry(128): makeEntry(1024),
	}
	warning := CheckLedgerEntriesSize(entries)
	if warning != nil {
		t.Errorf("expected no warning for small entries, got: %v", warning)
	}
}

func TestCheckLedgerEntriesSize_ExactlyAtLimit(t *testing.T) {
	// One entry whose key+value decoded bytes == MaxLedgerEntriesSizeBytes.
	// key = 512 KiB, value = 512 KiB → total = 1 MiB exactly = no warning.
	half := MaxLedgerEntriesSizeBytes / 2
	entries := map[string]string{
		makeEntry(half): makeEntry(half),
	}
	warning := CheckLedgerEntriesSize(entries)
	if warning != nil {
		t.Errorf("expected no warning at exactly the limit, got: %v", warning)
	}
}

func TestCheckLedgerEntriesSize_OneByteOver(t *testing.T) {
	half := MaxLedgerEntriesSizeBytes / 2
	entries := map[string]string{
		makeEntry(half): makeEntry(half + 1), // 1 byte over
	}
	warning := CheckLedgerEntriesSize(entries)
	if warning == nil {
		t.Fatal("expected a warning for entries 1 byte over the limit, got nil")
	}
	if warning.TotalBytes <= MaxLedgerEntriesSizeBytes {
		t.Errorf("expected TotalBytes > limit, got TotalBytes=%d limit=%d",
			warning.TotalBytes, MaxLedgerEntriesSizeBytes)
	}
	if warning.LimitBytes != MaxLedgerEntriesSizeBytes {
		t.Errorf("expected LimitBytes=%d, got %d", MaxLedgerEntriesSizeBytes, warning.LimitBytes)
	}
}

func TestCheckLedgerEntriesSize_MultipleEntriesOverLimit(t *testing.T) {
	// 10 entries, each key=64KiB + value=128KiB → 10 * 192 KiB = 1920 KiB > 1 MiB
	entries := make(map[string]string, 10)
	for i := 0; i < 10; i++ {
		entries[makeEntry(64*1024)] = makeEntry(128 * 1024)
	}
	warning := CheckLedgerEntriesSize(entries)
	if warning == nil {
		t.Fatal("expected a warning for 10 large entries, got nil")
	}
	if warning.EntryCount != 10 {
		t.Errorf("expected EntryCount=10, got %d", warning.EntryCount)
	}
}

func TestCheckLedgerEntriesSize_EmptyMap(t *testing.T) {
	warning := CheckLedgerEntriesSize(map[string]string{})
	if warning != nil {
		t.Errorf("expected no warning for empty map, got: %v", warning)
	}
}

func TestCheckLedgerEntriesSize_InvalidBase64Skipped(t *testing.T) {
	// An invalid base64 value should be counted as zero bytes, not panic.
	entries := map[string]string{
		"not-valid-base64!!!": "also-not-valid!!!",
	}
	warning := CheckLedgerEntriesSize(entries)
	// Total = 0, so no warning expected.
	if warning != nil {
		t.Errorf("expected no warning when base64 decode fails, got: %v", warning)
	}
}

func TestLedgerSizeWarning_ErrorMessage(t *testing.T) {
	w := &LedgerSizeWarning{
		TotalBytes: 2 * 1024 * 1024,
		LimitBytes: MaxLedgerEntriesSizeBytes,
		EntryCount: 5,
	}
	msg := w.Error()
	if !strings.Contains(msg, "rejected") {
		t.Errorf("expected error message to mention rejection, got: %q", msg)
	}
	if !strings.Contains(msg, "5") {
		t.Errorf("expected error message to mention entry count, got: %q", msg)
	}
}

func TestWarnLedgerEntriesSize_WritesWarningWhenOverLimit(t *testing.T) {
	half := MaxLedgerEntriesSizeBytes / 2
	entries := map[string]string{
		makeEntry(half): makeEntry(half + 1),
	}

	var buf bytes.Buffer
	warned := WarnLedgerEntriesSize(entries, &buf)

	if !warned {
		t.Fatal("expected WarnLedgerEntriesSize to return true when over limit")
	}
	out := buf.String()
	if !strings.Contains(out, "WARNING") {
		t.Errorf("expected output to contain WARNING, got: %q", out)
	}
	if !strings.Contains(out, "rejected") {
		t.Errorf("expected output to mention rejection, got: %q", out)
	}
}

func TestWarnLedgerEntriesSize_SilentWhenUnderLimit(t *testing.T) {
	entries := map[string]string{
		makeEntry(64): makeEntry(128),
	}

	var buf bytes.Buffer
	warned := WarnLedgerEntriesSize(entries, &buf)

	if warned {
		t.Error("expected WarnLedgerEntriesSize to return false when under limit")
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output when under limit, got: %q", buf.String())
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int
		contains string
	}{
		{512, "512 bytes"},
		{1024, "KiB"},
		{1024 * 1024, "MiB"},
		{2 * 1024 * 1024, "MiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("formatBytes(%d) = %q, expected to contain %q", tt.input, got, tt.contains)
		}
	}
}