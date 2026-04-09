// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package deeplink

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestMockURLFormat ensures the probe URL uses the correct scheme.
func TestMockURLFormat(t *testing.T) {
	if !hasPrefix(MockURL, Scheme+"://") {
		t.Errorf("MockURL %q does not start with %q", MockURL, Scheme+"://")
	}
}

// TestSchemeConstant ensures the scheme constant is lowercase and non-empty.
func TestSchemeConstant(t *testing.T) {
	if Scheme == "" {
		t.Fatal("Scheme must not be empty")
	}
	for _, c := range Scheme {
		if c >= 'A' && c <= 'Z' {
			t.Errorf("Scheme %q must be lowercase", Scheme)
		}
	}
}

// TestProbeTimeoutIsPositive guards against a zero or negative timeout.
func TestProbeTimeoutIsPositive(t *testing.T) {
	if probeTimeout <= 0 {
		t.Errorf("probeTimeout must be positive, got %v", probeTimeout)
	}
}

// TestTriggerMockLink_SelfBinary verifies that triggerMockLink succeeds when
// the binary under test handles --deep-link erst://doctor-probe correctly.
// This test builds a tiny stub binary that exits 0 immediately.
func TestTriggerMockLink_SelfBinary(t *testing.T) {
	stub := buildStubBinary(t, stubExitZero)
	if !triggerMockLink(stub) {
		t.Error("triggerMockLink should return true when binary exits 0")
	}
}

// TestTriggerMockLink_NonZeroExit verifies that a binary exiting non-zero
// causes triggerMockLink to return false.
func TestTriggerMockLink_NonZeroExit(t *testing.T) {
	stub := buildStubBinary(t, stubExitOne)
	if triggerMockLink(stub) {
		t.Error("triggerMockLink should return false when binary exits non-zero")
	}
}

// TestTriggerMockLink_Timeout verifies that a binary that hangs causes
// triggerMockLink to return false after the timeout.
func TestTriggerMockLink_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	stub := buildStubBinary(t, stubSleep)

	start := time.Now()
	result := triggerMockLink(stub)
	elapsed := time.Since(start)

	if result {
		t.Error("triggerMockLink should return false when binary times out")
	}
	// Should have waited at least probeTimeout (with some tolerance)
	if elapsed < probeTimeout-500*time.Millisecond {
		t.Errorf("triggerMockLink returned too quickly: %v (expected ~%v)", elapsed, probeTimeout)
	}
}

// TestCheckResult_UnresolvableSelf verifies that Check returns a non-nil Err
// when given an empty selfPath and os.Executable fails (simulated by a
// non-existent path).
func TestCheckResult_InvalidSelfPath(t *testing.T) {
	res := Check("/nonexistent/path/to/erst")
	// The binary doesn't exist so dispatch must fail.
	if res.Dispatched {
		t.Error("Dispatched should be false for a non-existent binary")
	}
}

// TestGenericFixSteps ensures every platform returns at least one fix step.
func TestGenericFixSteps(t *testing.T) {
	steps := genericFixSteps()
	if len(steps) == 0 {
		t.Error("genericFixSteps must return at least one step")
	}
	for i, s := range steps {
		if s == "" {
			t.Errorf("fix step %d is empty", i)
		}
	}
}

// TestGenericFixSteps_PlatformSpecific checks that the fix steps mention the
// correct OS-specific tool.
func TestGenericFixSteps_PlatformSpecific(t *testing.T) {
	steps := genericFixSteps()
	combined := ""
	for _, s := range steps {
		combined += s
	}

	switch runtime.GOOS {
	case "darwin":
		if !contains(combined, "open") && !contains(combined, "lsregister") {
			t.Error("macOS fix steps should mention 'open' or 'lsregister'")
		}
	case "windows":
		if !contains(combined, "registry") && !contains(combined, "HKEY") {
			t.Error("Windows fix steps should mention the registry")
		}
	default:
		if !contains(combined, "xdg") && !contains(combined, "desktop") {
			t.Error("Linux fix steps should mention xdg or .desktop")
		}
	}
}

// ---- helpers ----------------------------------------------------------------

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}()
}

// stub program sources – compiled on demand by buildStubBinary.
const stubExitZero = `package main

import "os"

func main() {
	// Simulate handling --deep-link erst://doctor-probe: exit 0.
	_ = os.Args
	os.Exit(0)
}
`

const stubExitOne = `package main

import "os"

func main() {
	os.Exit(1)
}
`

const stubSleep = `package main

import "time"

func main() {
	time.Sleep(30 * time.Second)
}
`

// buildStubBinary compiles src into a temporary binary and returns its path.
func buildStubBinary(t *testing.T, src string) string {
	t.Helper()

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0600); err != nil {
		t.Fatalf("write stub source: %v", err)
	}

	binName := "stub"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile stub: %v\n%s", err, out)
	}

	return binPath
}
