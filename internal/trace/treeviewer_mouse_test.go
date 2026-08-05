// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Fixes #1719: on a terminal that can't safely interpret ANSI/VT escape
// sequences (e.g. legacy Windows cmd.exe without
// ENABLE_VIRTUAL_TERMINAL_PROCESSING), these sequences must never be
// written raw -- they'd otherwise render as text blocks like "^[[2J^[[H"
// instead of clearing the screen / hiding the cursor.

func TestClearScreen_FallsBackToBlankLinesWhenAnsiUnsupported(t *testing.T) {
	t.Setenv("NO_COLOR", "1") // Forces ansiOutputSupported() to false.

	tv := NewTreeViewerWithMouse(nil)
	tv.screenHeight = 10

	out := captureStdout(t, func() {
		tv.clearScreen()
	})

	assert.NotContains(t, out, "\x1b[2J")
	assert.NotContains(t, out, "\x1b[H")
	assert.Equal(t, strings.Repeat("\n", 10), out)
}

func TestClearScreen_UsesEscapeSequenceWhenAnsiSupported(t *testing.T) {
	// NO_COLOR/TERM=dumb are the only overrides this package checks before
	// the platform probe, so explicitly clearing them (rather than
	// emptying, which still counts as "set" for NO_COLOR) is required to
	// reach that probe. Whether the probe itself reports true depends on
	// the platform and on stdout being a real console/tty, which isn't
	// guaranteed under `go test` -- so this only asserts the *shape* of
	// behavior (escape sequence xor blank-line fallback), not which
	// branch fires in this environment.
	origNoColor, hadNoColor := lookupAndUnset(t, "NO_COLOR")
	origTerm, hadTerm := lookupAndUnset(t, "TERM")
	t.Cleanup(func() { restoreEnv(t, "NO_COLOR", origNoColor, hadNoColor) })
	t.Cleanup(func() { restoreEnv(t, "TERM", origTerm, hadTerm) })

	tv := NewTreeViewerWithMouse(nil)
	tv.screenHeight = 10

	out := captureStdout(t, func() {
		tv.clearScreen()
	})

	if ansiOutputSupported() {
		assert.Equal(t, "\x1b[2J\x1b[H", out)
	} else {
		assert.Equal(t, strings.Repeat("\n", 10), out)
	}
}

func TestClearScreen_DefaultsHeightWhenUnset(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	tv := NewTreeViewerWithMouse(nil)
	tv.screenHeight = 0 // Not yet sized (e.g. before first resize event).

	out := captureStdout(t, func() {
		tv.clearScreen()
	})

	assert.Equal(t, strings.Repeat("\n", 24), out)
}

func TestEnableRawMode_NoRawEscapeWhenAnsiUnsupported(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	tv := NewTreeViewerWithMouse(nil)
	out := captureStdout(t, func() {
		err := tv.enableRawMode()
		assert.NoError(t, err)
	})

	assert.NotContains(t, out, "\x1b[?25l")
}

func TestRestoreTerminalState_NoRawEscapeWhenAnsiUnsupported(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	tv := NewTreeViewerWithMouse(nil)
	out := captureStdout(t, func() {
		err := tv.restoreTerminalState("")
		assert.NoError(t, err)
	})

	assert.NotContains(t, out, "\x1b[?25h")
}

// lookupAndUnset unsets the given env var for the duration of the test and
// returns its prior value so it can be restored via restoreEnv. Unlike
// t.Setenv("", ""), this makes the variable genuinely absent, which matters
// for NO_COLOR: per https://no-color.org its mere presence (even set to an
// empty string) counts as "set".
func lookupAndUnset(t *testing.T, key string) (value string, wasSet bool) {
	t.Helper()
	value, wasSet = os.LookupEnv(key)
	_ = os.Unsetenv(key)
	return value, wasSet
}

func restoreEnv(t *testing.T, key, value string, wasSet bool) {
	t.Helper()
	if wasSet {
		_ = os.Setenv(key, value)
	}
}
