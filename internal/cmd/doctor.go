// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/deeplink"
	"github.com/dotandev/hintents/internal/rpc"

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
  - Go installation and version (matches go.mod)
  - Rust toolchain (cargo, rustc)
  - Simulator binary (erst-sim)
  - Syntax of TOML config files
  - Reachability of the configured RPC endpoint
  - Deep link registration (erst:// URL scheme)

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
		checkConfigTOML(verbose),
		checkRPC(verbose),
		checkDeepLink(verbose),
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

	// compare against go.mod requirement if available
	if dep.Installed && dep.Version != "" {
		if data, err := os.ReadFile("go.mod"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "go ") {
					req := strings.TrimSpace(strings.TrimPrefix(line, "go "))
					if req != "" && !strings.HasPrefix(dep.Version, req) {
						dep.FixHint = fmt.Sprintf("go.mod requests %s but installed %s", req, dep.Version)
					}
					break
				}
			}
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

// checkConfigTOML verifies that any present configuration file can be parsed
// as basic TOML (naive syntax check). Missing file is treated as OK.
func checkConfigTOML(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "TOML config",
		FixHint: "Fix syntax in .erst.toml or remove the malformed file",
	}

	paths := []string{
		".erst.toml",
		filepath.Join(os.ExpandEnv("$HOME"), ".erst.toml"),
		"/etc/erst/config.toml",
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		// simple syntax sniff: non-empty, non-comment lines must contain 'key = value'
		for ln, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "[") {
				continue
			}
			eqIdx := strings.Index(trim, "=")
			if eqIdx < 0 {
				if verbose {
					dep.FixHint = fmt.Sprintf("%s (line %d missing '=')", dep.FixHint, ln+1)
				}
				return dep
			}
			// key must be non-empty and value must be non-empty
			key := strings.TrimSpace(trim[:eqIdx])
			val := strings.TrimSpace(trim[eqIdx+1:])
			if key == "" || val == "" {
				if verbose {
					dep.FixHint = fmt.Sprintf("%s (line %d has invalid key=value)", dep.FixHint, ln+1)
				}
				return dep
			}
		}

		dep.Installed = true
		return dep
	}

	dep.Installed = true // no config file - nothing to parse
	return dep
}

// checkRPC attempts a health ping to the current rpc endpoint
func checkRPC(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name:    "RPC endpoint",
		FixHint: "Set ERST_RPC_URL or ensure the default RPC is reachable",
	}

	cfg := config.DefaultConfig()
	url := cfg.RpcUrl
	if env := os.Getenv("ERST_RPC_URL"); env != "" {
		url = env
	}

	client, err := rpc.NewClient(rpc.WithHorizonURL(url), rpc.WithSorobanURL(url))
	if err != nil {
		return dep
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.GetHealth(ctx); err != nil {
		if verbose {
			dep.FixHint = "RPC health check failed: " + err.Error()
		}
		return dep
	}
	dep.Installed = true
	return dep
}

// checkDeepLink verifies that the erst:// URL scheme is registered with the OS
// and that dispatching a mock link actually reaches the current binary.
func checkDeepLink(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		Name: "Deep link (erst:// scheme)",
	}

	selfPath, err := os.Executable()
	if err != nil {
		dep.FixHint = "Cannot determine binary path: " + err.Error()
		return dep
	}

	result := deeplink.Check(selfPath)

	if result.Err != nil {
		dep.FixHint = result.Err.Error()
		if len(result.FixSteps) > 0 {
			dep.FixHint = result.FixSteps[0]
		}
		return dep
	}

	if !result.Registered {
		dep.FixHint = buildDeepLinkFixHint(result.FixSteps)
		return dep
	}

	if !result.Dispatched {
		dep.FixHint = "Scheme is registered but dispatch failed. Try: erst install-scheme"
		if len(result.FixSteps) > 0 {
			dep.FixHint = result.FixSteps[0]
		}
		return dep
	}

	dep.Installed = true
	if verbose && result.Handler != "" {
		dep.Path = result.Handler
	}
	dep.Version = "dispatch OK"
	return dep
}

func buildDeepLinkFixHint(steps []string) string {
	if len(steps) == 0 {
		return "Run 'erst install-scheme' to register the erst:// URL scheme"
	}
	return steps[0]
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolP("verbose", "v", false, "Show detailed diagnostic information")
}
