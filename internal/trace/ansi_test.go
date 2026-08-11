// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"os"
	"testing"
)

func TestAnsiOutputSupported_NoColorDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ansiOutputSupported() {
		t.Fatal("expected ansiOutputSupported to be false when NO_COLOR is set")
	}
}

func TestAnsiOutputSupported_TermDumbDisables(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if ansiOutputSupported() {
		t.Fatal("expected ansiOutputSupported to be false when TERM=dumb")
	}
}

func TestAnsiOutputSupported_DoesNotPanic(t *testing.T) {
	// Without NO_COLOR/TERM=dumb, the result depends on the platform and
	// whether stdout is a real console/tty (see enableANSISys in
	// ansi_windows.go / ansi_unix.go / ansi_other.go). Under `go test`
	// stdout is typically not a tty, so this is expected to be false, but
	// the important invariant tested here is that the platform-specific
	// capability probe never panics regardless of environment.
	//
	// NO_COLOR is unset (not just emptied) because its mere presence, even
	// with an empty value, counts as "set" per https://no-color.org and
	// would otherwise short-circuit this test before it reaches the
	// platform probe.
	origNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	origTerm, hadTerm := os.LookupEnv("TERM")
	_ = os.Unsetenv("NO_COLOR")
	_ = os.Unsetenv("TERM")
	t.Cleanup(func() {
		if hadNoColor {
			_ = os.Setenv("NO_COLOR", origNoColor)
		}
		if hadTerm {
			_ = os.Setenv("TERM", origTerm)
		}
	})

	_ = ansiOutputSupported()
}
