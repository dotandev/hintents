// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import "os"

// ansiOutputSupported reports whether writes to stdout can safely contain
// ANSI/VT escape sequences (SGR colour codes, cursor movement, screen
// clearing, and the mouse-reporting private modes used by this package).
//
// Being attached to a real console is NOT sufficient on its own: on
// Windows, a legacy cmd.exe session is a genuine console (so an isatty
// check alone reports true) but historically renders escape sequences as
// raw text -- e.g. "^[[2J^[[H" -- instead of interpreting them, unless
// ENABLE_VIRTUAL_TERMINAL_PROCESSING has been explicitly turned on for that
// console. enableANSISys (see ansi_windows.go, ansi_unix.go, ansi_other.go)
// performs that platform-specific capability probe/opt-in.
//
// NO_COLOR (https://no-color.org) and TERM=dumb are also honoured here so
// output can be forced plain regardless of platform, matching the
// conventions already used elsewhere in this codebase
// (internal/terminal.ANSIRenderer).
//
// Callers that would otherwise emit raw escape sequences directly (see
// mouse.go, treeviewer_mouse.go) must check this first and fall back to
// plain, ANSI-free behaviour when it returns false.
func ansiOutputSupported() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return enableANSISys()
}
