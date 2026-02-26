// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"os"
	"strings"
	"testing"
)

func TestANSIRenderer_IsTTY(t *testing.T) {
	// Test NO_COLOR
	os.Setenv("NO_COLOR", "1")
	r1 := NewANSIRenderer()
	if r1.IsTTY() {
		t.Error("IsTTY() should be false when NO_COLOR is set")
	}
	os.Unsetenv("NO_COLOR")

	// Test FORCE_COLOR
	os.Setenv("FORCE_COLOR", "1")
	r2 := NewANSIRenderer()
	if !r2.IsTTY() {
		t.Error("IsTTY() should be true when FORCE_COLOR is set")
	}
	os.Unsetenv("FORCE_COLOR")

	// Test TERM=dumb
	os.Setenv("TERM", "dumb")
	r3 := NewANSIRenderer()
	if r3.IsTTY() {
		t.Error("IsTTY() should be false when TERM=dumb")
	}
	os.Unsetenv("TERM")
}

func TestANSIRenderer_Colorize(t *testing.T) {
	os.Setenv("FORCE_COLOR", "1")
	r1 := NewANSIRenderer()

	text := "hello"
	colored := r1.Colorize(text, "red")
	os.Unsetenv("FORCE_COLOR")
	if !strings.Contains(colored, "\033[31m") {
		t.Errorf("Expected red color code, got %q", colored)
	}

	os.Setenv("NO_COLOR", "1")
	r2 := NewANSIRenderer()
	plain := r2.Colorize(text, "red")
	os.Unsetenv("NO_COLOR")
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
