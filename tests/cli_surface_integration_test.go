// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRootFromTestFile(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
}

func buildErstBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	binName := "erst"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/erst")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build erst binary: %v\nstderr:\n%s", err, stderr.String())
	}
	return binPath
}

func runBinary(t *testing.T, binPath, cwd string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = cwd
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf(
			"command failed: %s %v\nerr: %v\nstdout:\n%s\nstderr:\n%s",
			binPath,
			args,
			err,
			stdout.String(),
			stderr.String(),
		)
	}
}

func TestErstBinaryFullCLISurface(t *testing.T) {
	repoRoot := repoRootFromTestFile(t)
	binPath := buildErstBinary(t, repoRoot)

	homeDir := t.TempDir()
	env := append(os.Environ(),
		"HOME="+homeDir,
		"USERPROFILE="+homeDir,
		"ERST_SIM_COVERAGE_LCOV_PATH=",
		"ERST_SIM_PATH=",
	)

	cases := []struct {
		name string
		cwd  string
		args []string
	}{
		{name: "root-help", cwd: repoRoot, args: []string{"--help"}},
		{name: "version", cwd: repoRoot, args: []string{"version"}},
		{name: "debug-help", cwd: repoRoot, args: []string{"debug", "--help"}},
		{name: "explain-help", cwd: repoRoot, args: []string{"explain", "--help"}},
		{name: "auth-debug-help", cwd: repoRoot, args: []string{"auth-debug", "--help"}},
		{name: "compare-help", cwd: repoRoot, args: []string{"compare", "--help"}},
		{name: "stats-help", cwd: repoRoot, args: []string{"stats", "--help"}},
		{name: "trace-help", cwd: repoRoot, args: []string{"trace", "--help"}},
		{name: "report-help", cwd: repoRoot, args: []string{"report", "--help"}},
		{name: "profile-help", cwd: repoRoot, args: []string{"profile", "--help"}},
		{name: "rpc-help", cwd: repoRoot, args: []string{"rpc", "--help"}},
		{name: "rpc-health-help", cwd: repoRoot, args: []string{"rpc", "health", "--help"}},
		{name: "cache-help", cwd: repoRoot, args: []string{"cache", "--help"}},
		{name: "cache-status-help", cwd: repoRoot, args: []string{"cache", "status", "--help"}},
		{name: "cache-clean-help", cwd: repoRoot, args: []string{"cache", "clean", "--help"}},
		{name: "cache-clear-help", cwd: repoRoot, args: []string{"cache", "clear", "--help"}},
		{name: "session-help", cwd: repoRoot, args: []string{"session", "--help"}},
		{name: "session-save-help", cwd: repoRoot, args: []string{"session", "save", "--help"}},
		{name: "session-resume-help", cwd: repoRoot, args: []string{"session", "resume", "--help"}},
		{name: "session-list-help", cwd: repoRoot, args: []string{"session", "list", "--help"}},
		{name: "session-delete-help", cwd: repoRoot, args: []string{"session", "delete", "--help"}},
		{name: "search-help", cwd: repoRoot, args: []string{"search", "--help"}},
		{name: "doctor-help", cwd: repoRoot, args: []string{"doctor", "--help"}},
		{name: "abi-help", cwd: repoRoot, args: []string{"abi", "--help"}},
		{name: "dry-run-help", cwd: repoRoot, args: []string{"dry-run", "--help"}},
		{name: "simulate-upgrade-help", cwd: repoRoot, args: []string{"simulate-upgrade", "--help"}},
		{name: "fuzz-help", cwd: repoRoot, args: []string{"fuzz", "--help"}},
		{name: "shell-help", cwd: repoRoot, args: []string{"shell", "--help"}},
		{name: "xdr-help", cwd: repoRoot, args: []string{"xdr", "--help"}},
		{name: "daemon-help", cwd: repoRoot, args: []string{"daemon", "--help"}},
		{name: "completion-help", cwd: repoRoot, args: []string{"completion", "--help"}},
		{name: "init-help", cwd: repoRoot, args: []string{"init", "--help"}},
		{name: "wizard-help", cwd: repoRoot, args: []string{"wizard", "--help"}},
		{name: "export-help", cwd: repoRoot, args: []string{"export", "--help"}},
		{name: "init-local-environment", cwd: repoRoot, args: []string{"init", filepath.Join(homeDir, "workspace"), "--force", "--network", "testnet"}},
		{name: "cache-status-local-environment", cwd: repoRoot, args: []string{"cache", "status"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runBinary(t, binPath, tc.cwd, env, tc.args...)
		})
	}
}
