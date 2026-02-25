// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"strings"
	"testing"
)

func TestGetTermWidth_Default(t *testing.T) {
	// Under a test runner stdout is not a TTY, so getTermWidthSys returns 0.
	// Without COLUMNS set, we expect the hard-coded default of 80.
	t.Setenv("COLUMNS", "")
	w := getTermWidth()
	if w <= 0 {
		t.Fatalf("expected positive width, got %d", w)
	}
}

func TestGetTermWidth_ColumnsEnv(t *testing.T) {
	t.Setenv("COLUMNS", "120")
	w := getTermWidth()
	// Either the syscall returned a value or we used COLUMNS.
	if w <= 0 {
		t.Fatalf("expected positive width, got %d", w)
	}
}

func TestGetTermWidth_InvalidColumns(t *testing.T) {
	t.Setenv("COLUMNS", "not-a-number")
	w := getTermWidth()
	// Should fall through to default 80 (or syscall value).
	if w <= 0 {
		t.Fatalf("expected positive width, got %d", w)
	}
}

func TestWrapField_ShortValue(t *testing.T) {
	result := wrapField("Contract", "short", 80)
	if result != "Contract: short" {
		t.Fatalf("unexpected: %q", result)
	}
}

func TestWrapField_ExactFit(t *testing.T) {
	// value exactly fills available space — no wrapping needed.
	// "Contract: " = 10 chars, avail = 70, value = 70 chars.
	value := strings.Repeat("X", 70)
	result := wrapField("Contract", value, 80)
	if result != "Contract: "+value {
		t.Fatalf("expected single line, got %q", result)
	}
}

func TestWrapField_LongContractID(t *testing.T) {
	// A 56-character Stellar contract ID at a 30-wide terminal must wrap.
	id := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	result := wrapField("Contract", id, 30)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %d line(s): %q", len(lines), result)
	}
	// Continuation lines should be indented to align under the value.
	indent := strings.Repeat(" ", len("Contract: "))
	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, indent) {
			t.Fatalf("continuation line missing indent: %q", l)
		}
	}
}

func TestWrapField_NarrowTerminal(t *testing.T) {
	// Very narrow terminal (< len(prefix)+20): min avail=20 must still work.
	id := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	result := wrapField("Contract", id, 5)
	if len(result) == 0 {
		t.Fatal("expected non-empty result for narrow terminal")
	}
}

func TestWrapField_LongXDR(t *testing.T) {
	// XDR hashes are hex strings that can be 64+ chars — must reflow.
	xdr := strings.Repeat("a1b2c3d4", 12) // 96 chars
	result := wrapField("Code Hash", xdr, 40)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected XDR to wrap, got %d line(s): %q", len(lines), result)
	}
}

func TestSeparator_Wide(t *testing.T) {
	s := separator(100)
	if len(s) != 60 {
		t.Fatalf("expected 60-char separator for wide terminal, got %d", len(s))
	}
}

func TestSeparator_Narrow(t *testing.T) {
	s := separator(40)
	if len(s) != 40 {
		t.Fatalf("expected 40-char separator, got %d", len(s))
	}
}

func TestSeparator_VeryNarrow(t *testing.T) {
	s := separator(4)
	if len(s) != 10 {
		t.Fatalf("expected minimum 10-char separator, got %d", len(s))
	}
}
