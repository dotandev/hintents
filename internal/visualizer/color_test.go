// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"strings"
	"testing"
)

func TestNoColorDisablesColors(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	if ColorEnabled() {
		t.Error("ColorEnabled() should be false when NO_COLOR is set")
	}

	out := Colorize("hello", "red")
	if strings.Contains(out, "\033") {
		t.Errorf("Colorize should not contain ANSI when NO_COLOR set, got: %q", out)
	}
	if out != "hello" {
		t.Errorf("Colorize should return plain text, got: %q", out)
	}
}

func TestTermDumbDisablesColors(t *testing.T) {
	oldTerm := os.Getenv("TERM")
	defer func() { os.Setenv("TERM", oldTerm) }()
	os.Unsetenv("NO_COLOR")
	os.Setenv("TERM", "dumb")

	if ColorEnabled() {
		t.Error("ColorEnabled() should be false when TERM=dumb")
	}
}

func TestSymbolReturnsPlainASCIIWhenDisabled(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	for name, wantPlain := range map[string]string{
		"check":   "[OK]",
		"cross":   "[X]",
		"warn":    "[!]",
		"arrow_r": "->",
	} {
		got := Symbol(name)
		if got != wantPlain {
			t.Errorf("Symbol(%q) = %q, want %q (NO_COLOR should force plain ASCII)", name, got, wantPlain)
		}
	}
}

func TestSuccessWarningErrorNoEscapeWhenDisabled(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	for _, s := range []string{Success(), Warning(), Error()} {
		if strings.Contains(s, "\033") {
			t.Errorf("Output contains ANSI escape when disabled: %q", s)
		}
	}
}

func TestNoColorOverridesForceColor(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	os.Setenv("FORCE_COLOR", "1")
	defer func() {
		os.Unsetenv("NO_COLOR")
		os.Unsetenv("FORCE_COLOR")
	}()

	if ColorEnabled() {
		t.Error("NO_COLOR must take precedence over FORCE_COLOR")
	}
	out := Colorize("test", "red")
	if out != "test" {
		t.Errorf("NO_COLOR+FORCE_COLOR: expected plain text, got %q", out)
	}
}

func TestForceColorEnablesColorsWhenSet(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	// FORCE_COLOR=1 should enable colors (even when piped / not TTY)
	if !ColorEnabled() {
		t.Error("ColorEnabled() should be true when FORCE_COLOR=1 and NO_COLOR unset")
	}
	out := Colorize("hello", "red")
	if !strings.Contains(out, "\033") {
		t.Errorf("FORCE_COLOR=1: Colorize should emit ANSI, got plain: %q", out)
	}
}

func TestContractBoundaryPlainText(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	out := ContractBoundary("CABC", "CXYZ")
	expected := "--- contract boundary: CABC -> CXYZ ---"
	if out != expected {
		t.Errorf("ContractBoundary() = %q, want %q", out, expected)
	}
	if strings.Contains(out, "\033") {
		t.Errorf("ContractBoundary should not contain ANSI when NO_COLOR set, got: %q", out)
	}
}

func TestContractBoundaryWithColor(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	out := ContractBoundary("CABC", "CXYZ")
	if !strings.Contains(out, "CABC") || !strings.Contains(out, "CXYZ") {
		t.Errorf("ContractBoundary should contain both contract IDs, got: %q", out)
	}
	if !strings.Contains(out, "\033") {
		t.Errorf("ContractBoundary should contain ANSI codes when colors enabled, got: %q", out)
	}
}
