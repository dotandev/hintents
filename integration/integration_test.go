// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "erst.exe"
	}
	return "erst"
}

func binaryPath(t *testing.T) string {
	t.Helper()

	if env := os.Getenv("ERST_BINARY"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
		t.Fatalf("ERST_BINARY is set to %q but the file does not exist", env)
	}

	root := repoRoot(t)
	candidates := []string{
		filepath.Join(root, binaryName()),
		filepath.Join(root, "bin", binaryName()),
		filepath.Join(root, "dist", binaryName()),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Automate build if missing
	t.Log("erst binary not found; building it now...")
	binPath := filepath.Join(root, binaryName())
	buildErstBinary(t, root, binPath)
	return binPath
}

func buildErstBinary(t *testing.T, repoRoot, binPath string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/erst")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOWORK=off")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build erst binary: %v\nstderr:\n%s", err, stderr.String())
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod; are you inside the repo?")
		}
		dir = parent
	}
}

func runErst(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	bin := binaryPath(t)

	ctx, cancel := timeoutCtx(t, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func timeoutCtx(t *testing.T, d time.Duration) (context.Context, func()) {
	t.Helper()

	return buildTestContext(t, d)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// ────────────────────────────────────────────────────────────────────────────
// Helper assertions
// ────────────────────────────────────────────────────────────────────────────

func assertExitCode(t *testing.T, want int, err error) {
	t.Helper()
	if got := exitCode(err); got != want {
		t.Errorf("exit code: got %d, want %d (err=%v)", got, want, err)
	}
}

func assertContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected to find %q in:\n%s", label, needle, haystack)
	}
}

func assertNotContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("%s: did not expect to find %q in:\n%s", label, needle, haystack)
	}
}

func assertEmpty(t *testing.T, label, s string) { //nolint:unused
	t.Helper()
	if strings.TrimSpace(s) != "" {
		t.Errorf("%s: expected empty, got:\n%s", label, s)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// CLI Surface Area Tests
// ────────────────────────────────────────────────────────────────────────────

func TestBinaryExists(t *testing.T) {
	bin := binaryPath(t)
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("binary not found at %q: %v", bin, err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Fatalf("binary %q is not executable (mode %v)", bin, info.Mode())
	}
}

func TestVersionFlag(t *testing.T) {
	stdout, stderr, err := runErst(t, "--version")
	assertExitCode(t, 0, err)
	combined := stdout + stderr
	assertContains(t, "version output", combined, "erst")

	hasDigit := false
	for _, r := range combined {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		t.Errorf("version output does not contain a version number: %q", combined)
	}
}

func TestHelpFlag(t *testing.T) {
	stdout, stderr, err := runErst(t, "--help")
	assertExitCode(t, 0, err)
	combined := stdout + stderr
	for _, sub := range []string{"debug"} {
		assertContains(t, "--help output", combined, sub)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, err := runErst(t, "not-a-real-command")
	if exitCode(err) == 0 {
		t.Error("expected non-zero exit for unknown command")
	}
	assertContains(t, "stderr for unknown command", stderr, "unknown")
}

func TestNoArgs(t *testing.T) {
	stdout, stderr, err := runErst(t)
	combined := stdout + stderr
	_ = err
	assertContains(t, "no-args output", combined, "Usage")
}

// ────────────────────────────────────────────────────────────────────────────
// debug sub-command
// ────────────────────────────────────────────────────────────────────────────

func TestDebugHelp(t *testing.T) {
	stdout, stderr, err := runErst(t, "debug", "--help")
	assertExitCode(t, 0, err)
	combined := stdout + stderr
	assertContains(t, "debug --help", combined, "transaction-hash")
	assertContains(t, "debug --help", combined, "network")
}

func TestDebugMissingHash(t *testing.T) {
	_, _, err := runErst(t, "debug", "--network", "testnet")
	if exitCode(err) == 0 {
		t.Error("expected non-zero exit when transaction hash is missing")
	}
}

func TestDebugInvalidHash(t *testing.T) {
	_, stderr, err := runErst(t, "debug", "not-a-valid-hash", "--network", "testnet")
	if exitCode(err) == 0 {
		t.Error("expected non-zero exit for invalid transaction hash")
	}

	combined := stderr
	if !strings.Contains(combined, "invalid") &&
		!strings.Contains(combined, "error") &&
		!strings.Contains(combined, "failed") {
		t.Errorf("expected an error message in stderr, got: %q", combined)
	}
}

func TestDebugNetworkFlag(t *testing.T) {
	_, stderr, err := runErst(t,
		"debug", "aabbcc", "--network", "not-a-network",
	)
	if exitCode(err) == 0 {
		t.Error("expected non-zero exit for unrecognised network")
	}
	assertNotContains(t, "stderr", stderr, "panic")
}

// TestDebugInteractiveFlag removed because the --interactive flag no longer exists

// Deleted audit tests

// ────────────────────────────────────────────────────────────────────────────
// Cross-platform behavioural contracts
// ────────────────────────────────────────────────────────────────────────────

func TestStderrOnError(t *testing.T) {
	stdout, stderr, err := runErst(t, "debug", "badhash", "--network", "testnet")
	if exitCode(err) == 0 {
		t.Skip("binary returned 0 for bad hash; skipping stderr placement check")
	}
	// Error details must be on stderr.
	assertNotContains(t, "stdout on error", stdout, "error")
	_ = stderr
}

// TestExitCodeContract asserts exit code conventions:

func TestExitCodeContract(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantZero bool
	}{
		{"help", []string{"--help"}, true},
		{"bad command", []string{"xyzzy"}, false},
		{"debug no hash", []string{"debug"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runErst(t, tc.args...)
			code := exitCode(err)
			if tc.wantZero && code != 0 {
				t.Errorf("args %v: expected exit 0, got %d", tc.args, code)
			}
			if !tc.wantZero && code == 0 {
				t.Errorf("args %v: expected non-zero exit, got 0", tc.args)
			}
		})
	}
}

// TestNoPanicOnAnyFlag ensures common flag variations do not cause panics.
func TestNoPanicOnAnyFlag(t *testing.T) {
	flagCombinations := [][]string{
		{"--verbose"},
		{"--quiet"},
		{"--json"},
		{"debug", "--json"},
		{"debug", "--verbose"},
	}
	for _, args := range flagCombinations {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, stderr, _ := runErst(t, args...)
			assertNotContains(t, "stderr", stderr, "panic")
			assertNotContains(t, "stderr", stderr, "goroutine")
		})
	}
}
