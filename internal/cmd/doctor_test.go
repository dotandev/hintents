// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"os/exec"
	"testing"
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
