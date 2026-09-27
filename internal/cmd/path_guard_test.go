// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// safeWasmPath unit tests
// ---------------------------------------------------------------------------

func TestSafeWasmPath_TraversalRejected(t *testing.T) {
	traversalCases := []string{
		"../../etc/passwd",
		"../secret",
		"subdir/../../../etc/shadow",
		"/etc/passwd",           // absolute path outside any plausible workspace
		"dir/../../outside.txt", // net upward traversal
	}

	// Use a stable base directory (the test binary's temp dir isn't rooted
	// outside the filesystem, but we use t.TempDir() as a concrete base so
	// the test never depends on the real working directory).
	base := t.TempDir()

	for _, raw := range traversalCases {
		t.Run(raw, func(t *testing.T) {
			_, err := safeWasmPath(raw, base)
			if err == nil {
				t.Errorf("safeWasmPath(%q, %q): expected error for traversal path, got nil", raw, base)
			}
		})
	}
}

func TestSafeWasmPath_ValidPathAccepted(t *testing.T) {
	base := t.TempDir()

	validCases := []string{
		"contract.wasm",
		"./contract.wasm",
		"build/release/contract.wasm",
	}

	for _, raw := range validCases {
		t.Run(raw, func(t *testing.T) {
			got, err := safeWasmPath(raw, base)
			if err != nil {
				t.Fatalf("safeWasmPath(%q, %q): unexpected error: %v", raw, base, err)
			}
			if got == "" {
				t.Error("expected non-empty resolved path")
			}
		})
	}
}

func TestSafeWasmPath_EmptyPathRejected(t *testing.T) {
	base := t.TempDir()
	_, err := safeWasmPath("", base)
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// ---------------------------------------------------------------------------
// debug --wasm flag: PreRunE traversal check
// ---------------------------------------------------------------------------

func TestDebugCmd_WasmFlag_TraversalRejected(t *testing.T) {
	prev := wasmPath
	t.Cleanup(func() { wasmPath = prev })

	wasmPath = "../../etc/passwd"

	err := debugCmd.PreRunE(debugCmd, []string{})
	if err == nil {
		t.Fatal("expected PreRunE to reject traversal path for --wasm, got nil")
	}
	if !strings.Contains(err.Error(), "outside the workspace root") {
		t.Errorf("expected 'outside the workspace root' in error, got: %s", err.Error())
	}
}

func TestDebugCmd_WasmFlag_AbsoluteEscapeRejected(t *testing.T) {
	prev := wasmPath
	t.Cleanup(func() { wasmPath = prev })

	// Absolute path that does not share a prefix with the workspace root.
	// On Windows this would be something like C:\Windows\System32\... but we
	// use /etc/passwd as a portable indicator; on Windows filepath.Abs of it
	// still resolves to a path outside t.TempDir().
	wasmPath = "/etc/passwd"

	err := debugCmd.PreRunE(debugCmd, []string{})
	if err == nil {
		t.Fatal("expected PreRunE to reject out-of-root absolute path for --wasm, got nil")
	}
}

// ---------------------------------------------------------------------------
// upgrade --new-wasm flag: PreRunE traversal check
// ---------------------------------------------------------------------------

func TestUpgradeCmd_NewWasmFlag_TraversalRejected(t *testing.T) {
	prev := newWasmPath
	t.Cleanup(func() { newWasmPath = prev })

	newWasmPath = "../../etc/passwd"

	err := upgradeCmd.PreRunE(upgradeCmd, []string{"fakehash"})
	if err == nil {
		t.Fatal("expected PreRunE to reject traversal path for --new-wasm, got nil")
	}
	if !strings.Contains(err.Error(), "outside the workspace root") {
		t.Errorf("expected 'outside the workspace root' in error, got: %s", err.Error())
	}
}

func TestUpgradeCmd_NewWasmFlag_EmptyRejected(t *testing.T) {
	prev := newWasmPath
	t.Cleanup(func() { newWasmPath = prev })

	newWasmPath = ""

	err := upgradeCmd.PreRunE(upgradeCmd, []string{"fakehash"})
	if err == nil {
		t.Fatal("expected PreRunE to reject empty --new-wasm, got nil")
	}
}

// ---------------------------------------------------------------------------
// compare --wasm flag: PreRunE traversal check
// ---------------------------------------------------------------------------

func TestCompareCmd_WasmFlag_TraversalRejected(t *testing.T) {
	prev := cmpLocalWasmFlag
	t.Cleanup(func() { cmpLocalWasmFlag = prev })

	cmpLocalWasmFlag = "../../etc/passwd"

	err := compareCmd.PreRunE(compareCmd, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err == nil {
		t.Fatal("expected PreRunE to reject traversal path for --wasm, got nil")
	}
	if !strings.Contains(err.Error(), "outside the workspace root") {
		t.Errorf("expected 'outside the workspace root' in error, got: %s", err.Error())
	}
}

func TestCompareCmd_WasmFlag_AbsoluteEscapeRejected(t *testing.T) {
	prev := cmpLocalWasmFlag
	t.Cleanup(func() { cmpLocalWasmFlag = prev })

	cmpLocalWasmFlag = "/etc/passwd"

	err := compareCmd.PreRunE(compareCmd, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	if err == nil {
		t.Fatal("expected PreRunE to reject out-of-root absolute path for --wasm, got nil")
	}
}

// ---------------------------------------------------------------------------
// compare --bridge-wasm: parseContractWasmOverrideSpecs traversal check
// ---------------------------------------------------------------------------

func TestParseContractWasmOverrideSpecs_TraversalRejected(t *testing.T) {
	specs := []string{
		"CABC123=../../etc/passwd",
		"CABC456=../outside.wasm",
	}
	for _, spec := range specs {
		t.Run(spec, func(t *testing.T) {
			_, err := parseContractWasmOverrideSpecs([]string{spec})
			if err == nil {
				t.Fatalf("expected traversal to be rejected for spec %q, got nil", spec)
			}
		})
	}
}
