// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	// TERM=dumb disables - we can't easily test ColorEnabled() without TTY,
	// but we can verify the logic doesn't panic
	_ = termDumb()
	if !termDumb() {
		t.Error("termDumb() should be true when TERM=dumb")
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
