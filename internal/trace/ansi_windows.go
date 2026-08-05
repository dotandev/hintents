// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package trace

import (
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/sys/windows"
)

// enableANSISys attempts to turn on ENABLE_VIRTUAL_TERMINAL_PROCESSING on
// the stdout console handle so that ANSI/VT escape sequences render
// correctly instead of appearing as raw text blocks.
//
// This mode is only available on Windows 10 version 1511 (build 10586) and
// later; SetConsoleMode fails harmlessly on older Windows, and this returns
// false in that case (as well as when stdout isn't a console at all, e.g.
// piped/redirected output), signalling that callers must fall back to
// plain, escape-free output.
//
// Modern terminal hosts (Windows Terminal, VS Code's integrated terminal,
// recent PowerShell) already run with this mode on; enabling it here is a
// no-op for them and only matters for legacy cmd.exe / conhost sessions.
func enableANSISys() bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return false
	}

	handle := windows.Handle(os.Stdout.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true // Already enabled (e.g. Windows Terminal).
	}
	if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return false
	}
	return true
}
