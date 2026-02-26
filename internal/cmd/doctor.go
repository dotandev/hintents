// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

type DependencyStatus struct {
	Name      string
	Installed bool
	Version   string
	Path      string
	FixHint   string
}

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	GroupID: "development",
	Short:   "Diagnose development environment setup",
	Long: `Check the status of required dependencies and development tools.

This command verifies:
  - Go installation and version
  - Rust toolchain (cargo, rustc)
  - Simulator binary (erst-sim)

Use this to troubleshoot installation issues or verify your setup.`,
	Example: `  # Check environment status
  erst doctor

  # View detailed diagnostics
  erst doctor --verbose`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Println("Erst Environment Diagnostics")
	fmt.Println("=============================")
	fmt.Println()

	dependencies := []DependencyStatus{
		checkGo(verbose),
		checkRust(verbose),
		checkCargo(verbose),
		checkSimulator(verbose),
	}

	// Print results
	allOK := true
	for _, dep := range dependencies {
		status := "[OK]"
		statusColor := "\033[32m" // Green
		if !dep.Installed {
			status = "[FAIL]"
			statusColor = "\033[31m" // Red
			allOK = false
		}

		fmt.Printf("%s%s\033[0m %s", statusColor, status, dep.Name)
		if dep.Installed && dep.Version != "" {
			fmt.Printf(" (%s)", dep.Version)
		}
		fmt.Println()

		if verbose && dep.Path != "" {
			fmt.Printf("  Path: %s\n", dep.Path)
		}

		if !dep.Installed && dep.FixHint != "" {
			fmt.Printf("  \033[33m→ %s\033[0m\n", dep.FixHint)
		}
	}

	fmt.Println()

	// Summary
	if allOK {
		fmt.Println("\033[32m[OK] All dependencies are installed and ready!\033[0m")
		return nil
	}

	fmt.Println("\033[33m⚠ Some dependencies are missing. Follow the hints above to fix.\033[0m")
	return nil
}

func checkGo(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "Go",
		FixHint: "Install Go from https://go.dev/doc/install (requires Go 1.21+)",
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		return dep
	}

	dep.Installed = true
	dep.Path = goPath

	// Get version
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		// Extract just the version number (e.g., "go1.21.0" from "go version go1.21.0 linux/amd64")
		parts := strings.Fields(version)
		if len(parts) >= 3 {
			dep.Version = parts[2]
		}
	}

	return dep
}

func checkRust(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "Rust (rustc)",
		FixHint: "Install Rust from https://rustup.rs/ or run: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh",
	}

	rustcPath, err := exec.LookPath("rustc")
	if err != nil {
		return dep
	}

	dep.Installed = true
	dep.Path = rustcPath

	// Get version
	cmd := exec.Command("rustc", "--version")
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		// Extract version (e.g., "rustc 1.75.0" from "rustc 1.75.0 (82e1608df 2023-12-21)")
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			dep.Version = parts[1]
		}
	}

	return dep
}

func checkCargo(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "Cargo",
		FixHint: "Cargo is included with Rust. Install from https://rustup.rs/",
	}

	cargoPath, err := exec.LookPath("cargo")
	if err != nil {
		return dep
	}

	dep.Installed = true
	dep.Path = cargoPath

	// Get version
	cmd := exec.Command("cargo", "--version")
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		// Extract version (e.g., "cargo 1.75.0" from "cargo 1.75.0 (1d8b05cdd 2023-11-20)")
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			dep.Version = parts[1]
		}
	}

	return dep
}

func checkSimulator(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "Simulator Binary (erst-sim)",
		FixHint: "Build the simulator: cd simulator && cargo build --release",
	}

	// Check multiple possible locations
	possiblePaths := []string{
		"simulator/target/release/erst-sim",
		"./erst-sim",
		"../simulator/target/release/erst-sim",
	}

	// Add platform-specific extension for Windows
	if runtime.GOOS == "windows" {
		for i, path := range possiblePaths {
			possiblePaths[i] = path + ".exe"
		}
	}

	// Also check in PATH
	if simPath, err := exec.LookPath("erst-sim"); err == nil {
		dep.Installed = true
		dep.Path = simPath
		dep.Version = "in PATH"
		return dep
	}

	// Check relative paths
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			dep.Installed = true
			dep.Path = absPath
			dep.Version = "local build"
			return dep
		}
	}

	return dep
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolP("verbose", "v", false, "Show detailed diagnostic information")
}
