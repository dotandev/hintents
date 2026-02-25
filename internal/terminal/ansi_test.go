// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestANSIRenderer_IsTTY(t *testing.T) {
	r := NewANSIRenderer()

	// Test NO_COLOR
	os.Setenv("NO_COLOR", "1")
	if r.IsTTY() {
		t.Error("IsTTY() should be false when NO_COLOR is set")
	}
	os.Unsetenv("NO_COLOR")

	// Test FORCE_COLOR
	os.Setenv("FORCE_COLOR", "1")
	if !r.IsTTY() {
		t.Error("IsTTY() should be true when FORCE_COLOR is set")
	}
	os.Unsetenv("FORCE_COLOR")

	// Test TERM=dumb
	os.Setenv("TERM", "dumb")
	if r.IsTTY() {
		t.Error("IsTTY() should be false when TERM=dumb")
	}
	os.Unsetenv("TERM")
}

func TestANSIRenderer_Colorize(t *testing.T) {
	r := NewANSIRenderer()
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	text := "hello"
	colored := r.Colorize(text, "red")
	if !strings.Contains(colored, "\033[31m") {
		t.Errorf("Expected red color code, got %q", colored)
	}

	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")
	plain := r.Colorize(text, "red")
	if strings.Contains(plain, "\033") {
		t.Errorf("Expected plain text when NO_COLOR is set, got %q", plain)
	}
}

func TestANSIRenderer_Symbols(t *testing.T) {
	r := NewANSIRenderer()
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	if r.Symbol("check") != "[OK]" {
		t.Errorf("Expected [OK] for check symbol, got %q", r.Symbol("check"))
	}

	os.Setenv("NO_COLOR", "1")
	if r.Symbol("check") != "[OK]" {
		t.Errorf("Expected [OK] for check symbol when NO_COLOR, got %q", r.Symbol("check"))
	}
}
