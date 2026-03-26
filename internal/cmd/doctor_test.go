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
}

func TestDoctorCommand(t *testing.T) {
	// Test that the command is registered
	if doctorCmd == nil {
		t.Fatal("doctorCmd should not be nil")
	}

	if doctorCmd.Use != "doctor" {
		t.Errorf("doctorCmd.Use = %v, want 'doctor'", doctorCmd.Use)
	}

	// Test that verbose flag exists
	verboseFlag := doctorCmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("doctor command should have --verbose flag")
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
