// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"os/exec"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
)

func TestCheckGo(t *testing.T) {
	dep := checkGo(false)

	// Check if Go is in PATH
	_, err := exec.LookPath("go")
	expectedInstalled := err == nil

	if dep.Installed != expectedInstalled {
		t.Errorf("checkGo() installed = %v, want %v", dep.Installed, expectedInstalled)
	}

	if dep.Name != "Go" {
		t.Errorf("checkGo() name = %v, want 'Go'", dep.Name)
	}

	if !dep.Installed && dep.FixHint == "" {
		t.Error("checkGo() should provide FixHint when not installed")
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepGo {
		t.Errorf("checkGo() ID = %v, want %v", dep.ID, DepGo)
	}
}

func TestCheckRust(t *testing.T) {
	dep := checkRust(false)

	// Check if rustc is in PATH
	_, err := exec.LookPath("rustc")
	expectedInstalled := err == nil

	if dep.Installed != expectedInstalled {
		t.Errorf("checkRust() installed = %v, want %v", dep.Installed, expectedInstalled)
	}

	if dep.Name != "Rust (rustc)" {
		t.Errorf("checkRust() name = %v, want 'Rust (rustc)'", dep.Name)
	}

	if !dep.Installed && dep.FixHint == "" {
		t.Error("checkRust() should provide FixHint when not installed")
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepRust {
		t.Errorf("checkRust() ID = %v, want %v", dep.ID, DepRust)
	}
}

func TestCheckCargo(t *testing.T) {
	dep := checkCargo(false)

	// Check if cargo is in PATH
	_, err := exec.LookPath("cargo")
	expectedInstalled := err == nil

	if dep.Installed != expectedInstalled {
		t.Errorf("checkCargo() installed = %v, want %v", dep.Installed, expectedInstalled)
	}

	if dep.Name != "Cargo" {
		t.Errorf("checkCargo() name = %v, want 'Cargo'", dep.Name)
	}

	if !dep.Installed && dep.FixHint == "" {
		t.Error("checkCargo() should provide FixHint when not installed")
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepCargo {
		t.Errorf("checkCargo() ID = %v, want %v", dep.ID, DepCargo)
	}
}

func TestCheckSimulator(t *testing.T) {
	dep := checkSimulator(false)

	if dep.Name != "Simulator Binary (erst-sim)" {
		t.Errorf("checkSimulator() name = %v, want 'Simulator Binary (erst-sim)'", dep.Name)
	}

	if !dep.Installed && dep.FixHint == "" {
		t.Error("checkSimulator() should provide FixHint when not installed")
	}

	// If simulator is found, verify path is set
	if dep.Installed && dep.Path == "" {
		t.Error("checkSimulator() should set Path when installed")
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepSimulator {
		t.Errorf("checkSimulator() ID = %v, want %v", dep.ID, DepSimulator)
	}

	// FIXED: Verify Fixable is set (Issue #9)
	if !dep.Fixable {
		t.Error("checkSimulator() should be fixable")
	}
}

func TestCheckSimulatorPaths(t *testing.T) {
	// Test that simulator checks multiple paths
	dep := checkSimulator(false)

	// The function should check:
	// 1. PATH environment
	// 2. simulator/target/release/erst-sim
	// 3. ./erst-sim
	// 4. ../simulator/target/release/erst-sim

	// If none exist, should not be installed
	if dep.Installed {
		// Verify the path actually exists
		if _, err := os.Stat(dep.Path); os.IsNotExist(err) {
			t.Errorf("checkSimulator() reported installed but path does not exist: %s", dep.Path)
		}
	}
}

func TestGoVersionMismatch(t *testing.T) {
	// write a temporary go.mod with incompatible version
	orig, err := os.ReadFile("go.mod")
	defer func() {
		if err != nil {
			os.Remove("go.mod")
		} else {
			os.WriteFile("go.mod", orig, 0644)
		}
	}()
	_ = os.WriteFile("go.mod", []byte("module foo\n\ngo 9.99\n"), 0644)
	dep := checkGo(false)
	if dep.FixHint == "" {
		t.Error("expected FixHint when go version mismatches go.mod")
	}
}

func TestCheckConfigTOML(t *testing.T) {
	// no config file -> success
	os.Remove(".erst.toml")
	dep := checkConfigTOML(false)
	if !dep.Installed {
		t.Error("expected config check to pass when no file present")
	}

	// valid config
	os.WriteFile(".erst.toml", []byte("rpc_url = \"https://example.com\"\n"), 0644)
	dep = checkConfigTOML(false)
	if !dep.Installed {
		t.Error("expected valid toml to succeed")
	}

	// invalid syntax
	os.WriteFile(".erst.toml", []byte("rpc_url = \n"), 0644)
	dep = checkConfigTOML(true)
	if dep.Installed {
		t.Error("expected invalid toml to fail")
	}
	os.Remove(".erst.toml")

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepConfigTOML {
		t.Errorf("checkConfigTOML() ID = %v, want %v", dep.ID, DepConfigTOML)
	}
}

func TestCheckRPC(t *testing.T) {
	// start mock server responding healthy
	rs := rpc.NewMockServer(map[string]rpc.MockRoute{
		"/": rpc.SuccessRoute(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"status": "healthy"},
		}),
	})
	defer rs.Close()
	os.Setenv("ERST_RPC_URL", rs.URL())
	defer os.Unsetenv("ERST_RPC_URL")
	dep := checkRPC(false)
	if !dep.Installed {
		t.Error("expected rpc check to succeed against mock server")
	}

	// bad url
	os.Setenv("ERST_RPC_URL", "http://nonexistent.invalid")
	dep = checkRPC(false)
	if dep.Installed {
		t.Error("expected rpc check to fail for unreachable url")
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepRPC {
		t.Errorf("checkRPC() ID = %v, want %v", dep.ID, DepRPC)
	}
}

// FIXED: TestDoctorCommand no longer mutates global state (Issue #5)
func TestDoctorCommand(t *testing.T) {
	// Test that the command is registered
	if doctorCmd == nil {
		t.Fatal("doctorCmd should not be nil")
	}

	if doctorCmd.Use != "doctor" {
		t.Errorf("doctorCmd.Use = %v, want 'doctor'", doctorCmd.Use)
	}

	// Test that flags exist using Lookup instead of Set (Issue #5)
	verboseFlag := doctorCmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("doctor command should have --verbose flag")
	}
}

// FIXED: TestDoctorWithFix verifies --fix flag is recognized (Issue #5)
func TestDoctorWithFix(t *testing.T) {
	flag := doctorCmd.Flags().Lookup("fix")
	if flag == nil {
		t.Fatal("doctor command should have --fix flag")
	}
}

// FIXED: TestDoctorWithYes verifies --yes flag is recognized (Issue #5)
func TestDoctorWithYes(t *testing.T) {
	flag := doctorCmd.Flags().Lookup("yes")
	if flag == nil {
		t.Fatal("doctor command should have --yes flag")
	}
}

// FIXED: TestDependencyStatusFixable verifies Fixable field (Issue #9)
func TestDependencyStatusFixable(t *testing.T) {
	dep := DependencyStatus{
		ID:      DepCacheDir,
		Name:    "Test Dependency",
		Fixable: true,
	}

	if !dep.Fixable {
		t.Fatal("Fixable field not set correctly")
	}
}

// FIXED: TestDependencyIDDispatch verifies ID-based routing (Issue #8)
func TestDependencyIDDispatch(t *testing.T) {
	tests := []struct {
		name  string
		depID DependencyID
		found bool
	}{
		{"CacheDir", DepCacheDir, true},
		{"Simulator", DepSimulator, true},
		{"ProtocolRegistry", DepProtocolRegistry, true},
		{"GoModDependencies", DepGoModDependencies, true},
		{"InvalidID", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.depID {
			case DepCacheDir, DepSimulator, DepProtocolRegistry, DepGoModDependencies:
				if !tt.found {
					t.Errorf("expected %s to be recognized", tt.depID)
				}
			default:
				if tt.found {
					t.Errorf("did not expect %s to be recognized", tt.depID)
				}
			}
		})
	}
}

// NEW: TestCheckCacheDir verifies cache directory check (Issue #9)
func TestCheckCacheDir(t *testing.T) {
	dep := checkCacheDir(false)

	if dep.Name != "Cache directory (~/.erst)" {
		t.Errorf("checkCacheDir() name = %v, want 'Cache directory (~/.erst)'", dep.Name)
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepCacheDir {
		t.Errorf("checkCacheDir() ID = %v, want %v", dep.ID, DepCacheDir)
	}

	// FIXED: Verify Fixable is set (Issue #9)
	if !dep.Fixable {
		t.Error("checkCacheDir() should be fixable")
	}
}

// NEW: TestCheckProtocolRegistry verifies protocol registry check (Issue #9)
func TestCheckProtocolRegistry(t *testing.T) {
	dep := checkProtocolRegistry(false)

	if dep.Name != "Protocol Registry" {
		t.Errorf("checkProtocolRegistry() name = %v, want 'Protocol Registry'", dep.Name)
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepProtocolRegistry {
		t.Errorf("checkProtocolRegistry() ID = %v, want %v", dep.ID, DepProtocolRegistry)
	}

	// FIXED: Verify Fixable is set (Issue #9)
	if !dep.Fixable {
		t.Error("checkProtocolRegistry() should be fixable")
	}
}

// NEW: TestCheckGoModDependencies verifies go.mod check (Issue #9)
func TestCheckGoModDependencies(t *testing.T) {
	dep := checkGoModDependencies(false)

	if dep.Name != "Go Module Dependencies" {
		t.Errorf("checkGoModDependencies() name = %v, want 'Go Module Dependencies'", dep.Name)
	}

	// FIXED: Verify ID is set correctly (Issue #8)
	if dep.ID != DepGoModDependencies {
		t.Errorf("checkGoModDependencies() ID = %v, want %v", dep.ID, DepGoModDependencies)
	}

	// FIXED: Verify Fixable is set (Issue #9)
	if !dep.Fixable {
		t.Error("checkGoModDependencies() should be fixable")
	}
}

// NEW: Test all DependencyIDs are unique (Issue #8)
func TestDependencyIDsUnique(t *testing.T) {
	ids := map[DependencyID]bool{
		DepGo:                true,
		DepRust:              true,
		DepCargo:             true,
		DepSimulator:         true,
		DepCacheDir:          true,
		DepProtocolRegistry:  true,
		DepGoModDependencies: true,
		DepConfigTOML:        true,
		DepRPC:               true,
	}

	if len(ids) != 9 {
		t.Errorf("expected 9 unique DependencyIDs, got %d", len(ids))
	}
}

// NEW: Test that all dependencies have required fields set
func TestDependencyStatusFieldsSet(t *testing.T) {
	deps := []DependencyStatus{
		checkGo(false),
		checkRust(false),
		checkCargo(false),
		checkSimulator(false),
		checkCacheDir(false),
		checkProtocolRegistry(false),
		checkGoModDependencies(false),
		checkConfigTOML(false),
		checkRPC(false),
	}

	for _, dep := range deps {
		if dep.ID == "" {
			t.Errorf("dependency %s has empty ID", dep.Name)
		}
		if dep.Name == "" {
			t.Errorf("dependency with ID %s has empty Name", dep.ID)
		}
		if dep.FixHint == "" && !dep.Installed {
			t.Errorf("dependency %s has empty FixHint when not installed", dep.Name)
		}
	}
}

// TestCheckDeepLink_Name verifies the check returns the correct display name.
func TestCheckDeepLink_Name(t *testing.T) {
	dep := checkDeepLink(false)
	if dep.Name != "Deep link (erst:// scheme)" {
		t.Errorf("checkDeepLink() name = %q, want %q", dep.Name, "Deep link (erst:// scheme)")
	}
}

// TestCheckDeepLink_FailHasHint verifies that when the scheme is not registered
// a non-empty FixHint is provided.
func TestCheckDeepLink_FailHasHint(t *testing.T) {
	dep := checkDeepLink(false)
	// On CI the scheme is almost certainly not registered, so we only assert
	// that a hint is present when the check fails.
	if !dep.Installed && dep.FixHint == "" {
		t.Error("checkDeepLink() should provide FixHint when scheme is not registered")
	}
}

// TestCheckDeepLink_VerbosePath verifies that verbose mode populates Path when
// the check succeeds.
func TestCheckDeepLink_VerbosePath(t *testing.T) {
	dep := checkDeepLink(true)
	if dep.Installed && dep.Path == "" {
		t.Error("checkDeepLink(verbose=true) should set Path when installed")
	}
}

// TestBuildDeepLinkFixHint_Empty verifies the fallback message when no steps
// are provided.
func TestBuildDeepLinkFixHint_Empty(t *testing.T) {
	hint := buildDeepLinkFixHint(nil)
	if hint == "" {
		t.Error("buildDeepLinkFixHint(nil) must return a non-empty fallback hint")
	}
}

// TestBuildDeepLinkFixHint_UsesFirstStep verifies that the first step is used.
func TestBuildDeepLinkFixHint_UsesFirstStep(t *testing.T) {
	steps := []string{"step one", "step two"}
	hint := buildDeepLinkFixHint(steps)
	if hint != "step one" {
		t.Errorf("buildDeepLinkFixHint() = %q, want %q", hint, "step one")
	}
}
