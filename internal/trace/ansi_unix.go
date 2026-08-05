// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !plan9 && !js && !wasip1

package trace

import (
	"os"

	"github.com/mattn/go-isatty"
)

// enableANSISys reports whether stdout is attached to a real terminal.
// Unlike Windows, POSIX terminals that identify as a tty reliably support
// ANSI/VT escape sequences, so no further capability negotiation is
// required here.
func enableANSISys() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
