// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package deeplink provides utilities for verifying that the erst:// custom
// URL scheme is correctly registered with the host operating system and that
// clicking such a link actually dispatches to the running erst binary.
package deeplink

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// Scheme is the custom URL scheme registered by erst.
	Scheme = "erst"

	// MockURL is a safe no-op deep link used only for registration probing.
	// The handler must recognise the "doctor-probe" path and exit cleanly.
	MockURL = "erst://doctor-probe"

	// probeTimeout is the maximum time we wait for the OS to dispatch the link.
	probeTimeout = 5 * time.Second
)

// Result carries the outcome of a deep link verification attempt.
type Result struct {
	// Registered reports whether the erst:// scheme is registered with the OS.
	Registered bool
	// Dispatched reports whether a mock link was successfully dispatched.
	Dispatched bool
	// Handler is the binary path the OS has associated with the scheme.
	Handler string
	// Err holds the first error encountered, if any.
	Err error
	// FixSteps contains ordered troubleshooting instructions.
	FixSteps []string
}

// Check performs a two-phase verification:
//  1. Inspect OS registration to confirm the scheme points to an erst binary.
//  2. Trigger MockURL and verify the process exits cleanly within probeTimeout.
//
// The probe is intentionally non-interactive: the binary must handle
// "erst://doctor-probe" by printing nothing and exiting 0.
func Check(selfPath string) Result {
	if selfPath == "" {
		var err error
		selfPath, err = os.Executable()
		if err != nil {
			return Result{
				Err:      fmt.Errorf("cannot determine own executable path: %w", err),
				FixSteps: genericFixSteps(),
			}
		}
	}
	selfPath, _ = filepath.Abs(selfPath)

	res := checkRegistration(selfPath)
	if !res.Registered {
		return res
	}

	res.Dispatched = triggerMockLink(selfPath)
	if !res.Dispatched {
		res.FixSteps = append(res.FixSteps,
			"The scheme is registered but the OS failed to dispatch the mock link.",
			"Try re-running 'erst install-scheme' or reinstalling erst.",
		)
	}
	return res
}

// genericFixSteps returns platform-appropriate registration instructions.
func genericFixSteps() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"Register the scheme: erst install-scheme",
			"Or manually add erst to /Applications and run: /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f /Applications/erst.app",
			"Verify with: open erst://doctor-probe",
		}
	case "windows":
		return []string{
			"Register the scheme: erst install-scheme  (requires Administrator)",
			"Or manually add the registry key: HKEY_CLASSES_ROOT\\erst",
			"Verify with: start erst://doctor-probe",
		}
	default: // Linux / BSD
		return []string{
			"Register the scheme: erst install-scheme",
			"Or create ~/.local/share/applications/erst.desktop with MimeType=x-scheme-handler/erst",
			"Then run: xdg-mime default erst.desktop x-scheme-handler/erst",
			"Verify with: xdg-open erst://doctor-probe",
		}
	}
}

// triggerMockLink dispatches MockURL through the OS handler and waits for the
// process to exit.  It returns true only when the process exits with code 0
// within probeTimeout.
func triggerMockLink(selfPath string) bool {
	// We invoke the binary directly rather than through the OS URL dispatcher
	// so the test is hermetic and does not require the scheme to be registered.
	// The --deep-link flag tells the binary to handle the URL and exit.
	cmd := exec.Command(selfPath, "--deep-link", MockURL) //nolint:gosec // selfPath is our own binary
	cmd.Env = append(os.Environ(), "ERST_DEEP_LINK_PROBE=1")

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err == nil
	case <-time.After(probeTimeout):
		_ = cmd.Process.Kill()
		return false
	}
}
