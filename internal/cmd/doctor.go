// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"context"
	"encoding/json"
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

func joinPath(parts ...string) string {
	return strings.Join(parts, "/")
}

// FriendlyPath replaces the user's home directory with ~ for better readability
func FriendlyPath(path string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	// Normalize both paths to use forward slashes for comparison
	normalizedPath := filepath.ToSlash(path)
	normalizedHomeDir := filepath.ToSlash(homeDir)

	// Replace home directory with ~
	if strings.HasPrefix(normalizedPath, normalizedHomeDir) {
		return "~" + strings.TrimPrefix(normalizedPath, normalizedHomeDir)
	}

	return path
}

// DependencyID is a unique identifier for each dependency (Issue #8: type-safe dispatch)
type DependencyID string

const (
	DepGo                DependencyID = "go"
	DepRust              DependencyID = "rust"
	DepCargo             DependencyID = "cargo"
	DepSimulator         DependencyID = "simulator"
	DepCacheDir          DependencyID = "cache_dir"
	DepProtocolRegistry  DependencyID = "protocol_registry"
	DepGoModDependencies DependencyID = "go_mod_dependencies"
	DepConfigTOML        DependencyID = "toml_config"
	DepRPC               DependencyID = "rpc"
)

type DependencyStatus struct {
	ID        DependencyID // NEW: unique identifier for type-safe dispatch
	Name      string
	Installed bool
	Version   string
	Path      string
	FixHint   string
	Fixable   bool // NEW: indicates if this can be auto-fixed
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
  erst doctor --verbose

  # Fix common issues (with prompts)
  erst doctor --fix

  # Fix without prompts (CI mode)
  erst doctor --fix --yes`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	fix, _ := cmd.Flags().GetBool("fix")
	yes, _ := cmd.Flags().GetBool("yes")

	fmt.Println("Erst Environment Diagnostics")
	fmt.Println("=============================")
	fmt.Println()

	dependencies := []DependencyStatus{
		checkGo(verbose),
		checkRust(verbose),
		checkCargo(verbose),
		checkSimulator(verbose),
		checkCacheDir(verbose),
		checkProtocolRegistry(verbose),
		checkGoModDependencies(verbose),
		checkConfigTOML(verbose),
		checkRPC(verbose),
		checkDeepLink(verbose),
	}

	// Print results
	allOK := true
	fixableCount := 0
	for _, dep := range dependencies {
		status := "[OK]"
		statusColor := "\033[32m" // Green
		if !dep.Installed {
			status = "[FAIL]"
			statusColor = "\033[31m" // Red
			allOK = false
			if dep.Fixable {
				fixableCount++
				status = "[FAIL*]" // * indicates fixable
			}
		}

		fmt.Printf("%s%s\033[0m %s", statusColor, status, dep.Name)
		if dep.Installed && dep.Version != "" {
			fmt.Printf(" (%s)", dep.Version)
		}
		fmt.Println()

		if verbose && dep.Path != "" {
			fmt.Printf("  Path: %s\n", FriendlyPath(dep.Path))
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

	// NEW: Handle --fix mode
	if fix {
		fmt.Printf("\n\033[36m[FIX MODE]\033[0m Attempting to fix %d issue(s)\n", fixableCount)
		if yes {
			fmt.Println("(Running in non-interactive mode)")
		}
		fmt.Println()
		return runFixers(dependencies, yes, verbose)
	}

	fmt.Println("\033[33m⚠ Some dependencies are missing. Follow the hints above to fix.\033[0m")
	if fixableCount > 0 {
		fmt.Printf("\nTip: Run 'erst doctor --fix' to attempt automatic fixes for %d issue(s).\n", fixableCount)
	}
	return nil
}

func checkGo(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		ID:      DepGo,
		Name:    "Go",
		FixHint: "Install Go from https://go.dev/doc/install (requires Go 1.21+)",
		Fixable: false, // Cannot auto-fix Go installation
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
		ID:      DepRust,
		Name:    "Rust (rustc)",
		FixHint: "Install Rust from https://rustup.rs/ or run: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh",
		Fixable: false,
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
		ID:      DepCargo,
		Name:    "Cargo",
		FixHint: "Cargo is included with Rust. Install from https://rustup.rs/",
		Fixable: false,
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
		ID:      DepSimulator,
		Name:    "Simulator Binary (erst-sim)",
		FixHint: "Build the simulator: cd simulator && cargo build --release",
		Fixable: true, // CAN be auto-fixed
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
		versionOutput, validationErr := validateSimulatorBinary(simPath)
		if validationErr == nil {
			dep.Installed = true
			dep.Path = simPath
			dep.Version = strings.TrimSpace(versionOutput)
			return dep
		}
		dep.FixHint = validationErr.Error()
	}

	// Check relative paths
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			versionOutput, validationErr := validateSimulatorBinary(absPath)
			if validationErr != nil {
				dep.FixHint = validationErr.Error()
				continue
			}
			dep.Installed = true
			dep.Path = absPath
			dep.Version = strings.TrimSpace(versionOutput)
			return dep
		}
	}

	return dep
}

func validateSimulatorBinary(path string) (string, error) {
	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("erst-sim version check failed with non-zero exit code: %v", err)
		}
		return "", fmt.Errorf("erst-sim version check failed with non-zero exit code: %v (%s)", err, text)
	}

	if !strings.Contains(text, "erst-sim") {
		return "", fmt.Errorf("erst-sim validation failed: --version output did not contain \"erst-sim\"")
	}

	return text, nil
}

// checkCacheDir verifies the cache directory exists (NEW: Issue #9)
func checkCacheDir(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		ID:      DepCacheDir,
		Name:    "Cache directory (~/.erst)",
		FixHint: "Run 'erst doctor --fix' to create cache directory",
		Fixable: true, // CAN be auto-fixed
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		dep.FixHint = "Failed to determine home directory"
		return dep
	}

	cacheDir := joinPath(homeDir, ".erst")

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); err != nil {
		if os.IsNotExist(err) {
			return dep // Not installed
		}
		dep.FixHint = fmt.Sprintf("Cache directory exists but inaccessible: %v", err)
		return dep
	}

	// Verify subdirectories exist
	requiredDirs := []string{"transactions", "protocols", "contracts"}
	for _, subdir := range requiredDirs {
		path := joinPath(cacheDir, subdir)
		if _, err := os.Stat(path); err != nil {
			dep.FixHint = fmt.Sprintf("Missing subdirectory: %s", subdir)
			return dep
		}
	}

	dep.Installed = true
	dep.Path = cacheDir
	dep.Version = "configured"
	return dep
}

// checkProtocolRegistry verifies the protocol registry exists (NEW: Issue #9)
func checkProtocolRegistry(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		ID:      DepProtocolRegistry,
		Name:    "Protocol Registry",
		FixHint: "Run 'erst doctor --fix' to initialize protocol registry",
		Fixable: true,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		dep.FixHint = "Failed to determine home directory"
		return dep
	}

	registryFile := joinPath(homeDir, ".erst", "protocols", "registered.json")

	// Check if registry file exists and is valid
	if _, err := os.Stat(registryFile); err != nil {
		if os.IsNotExist(err) {
			return dep // Not installed, but fixable
		}
		dep.FixHint = fmt.Sprintf("Registry file exists but inaccessible: %v", err)
		return dep
	}

	// Verify it's valid JSON
	data, err := os.ReadFile(registryFile)
	if err != nil {
		dep.FixHint = fmt.Sprintf("Failed to read registry: %v", err)
		return dep
	}

	var registry map[string]interface{}
	if err := json.Unmarshal(data, &registry); err != nil {
		dep.FixHint = "Protocol registry is corrupted JSON"
		return dep
	}

	dep.Installed = true
	dep.Version = "configured"
	return dep
}

// checkGoModDependencies verifies go.mod dependencies are synchronized (NEW: Issue #9)
func checkGoModDependencies(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		ID:      DepGoModDependencies,
		Name:    "Go Module Dependencies",
		FixHint: "Run 'erst doctor --fix' to resolve dependencies",
		Fixable: true,
	}

	// Check if go.mod exists
	if _, err := os.Stat("go.mod"); err != nil {
		if os.IsNotExist(err) {
			return dep // Not present
		}
		dep.FixHint = fmt.Sprintf("go.mod inaccessible: %v", err)
		return dep
	}

	// Check go.sum exists
	if _, err := os.Stat("go.sum"); err != nil {
		if os.IsNotExist(err) {
			return dep // Missing go.sum, needs fix
		}
	}

	dep.Installed = true
	dep.Version = "synchronized"
	return dep
}

// checkConfigTOML verifies that any present configuration file can be parsed
// as basic TOML (naive syntax check). Missing file is treated as OK.
func checkConfigTOML(verbose bool) DependencyStatus {
	dep := DependencyStatus{
		ID:      DepConfigTOML,
		Name:    "TOML config",
		FixHint: "Fix syntax in .erst.toml or remove the malformed file",
		Fixable: false,
	}

	paths := []string{
		".erst.toml",
		joinPath(os.ExpandEnv("$HOME"), ".erst.toml"),
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
		ID:      DepRPC,
		Name:    "RPC endpoint",
		FixHint: "Set ERST_RPC_URL or ensure the default RPC is reachable",
		Fixable: false,
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

// NEW: runFixers orchestrates automatic fixes with ID-based dispatch (Issues #3, #7, #8)
func runFixers(deps []DependencyStatus, skipConfirm, verbose bool) error {
	var failedFixes []string
	var successFixes []string

	for _, dep := range deps {
		if dep.Installed || !dep.Fixable {
			continue // Skip already installed or non-fixable items
		}

		shouldFix := skipConfirm
		if !skipConfirm {
			// Prompt user for confirmation
			fmt.Printf("Fix %s? [y/N]: ", dep.Name)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			shouldFix = strings.HasPrefix(strings.ToLower(response), "y")
		}

		if !shouldFix {
			continue
		}

		// FIXED: Use ID-based dispatch instead of brittle string matching (Issue #8)
		var err error
		switch dep.ID {
		case DepCacheDir:
			err = FixMissingCacheDir(verbose)
		case DepSimulator:
			err = FixSimulatorBinary(verbose)
		case DepProtocolRegistry:
			err = FixProtocolRegistration(verbose)
		case DepGoModDependencies:
			err = FixGoModDependencies(verbose)
		default:
			continue
		}

		if err != nil {
			failedFixes = append(failedFixes, fmt.Sprintf("%s: %v", dep.Name, err))
		} else {
			successFixes = append(successFixes, dep.Name)
		}
	}

	// Summary
	fmt.Println("\n=== Fix Summary ===")
	if len(successFixes) > 0 {
		fmt.Printf("\033[32m[OK] Fixed (%d):\033[0m\n", len(successFixes))
		for _, fix := range successFixes {
			fmt.Printf("  [OK] %s\n", fix)
		}
	}

	if len(failedFixes) > 0 {
		fmt.Printf("\033[31m[FAIL] Failed (%d):\033[0m\n", len(failedFixes))
		for _, fix := range failedFixes {
			fmt.Printf("  [FAIL] %s\n", fix)
		}
		return fmt.Errorf("some fixes failed")
	}

	if len(successFixes) > 0 {
		fmt.Println("\033[32m[OK] All fixes applied successfully!\033[0m")
	} else {
		fmt.Println("[OK] No fixes needed")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolP("verbose", "v", false, "Show detailed diagnostic information")
	doctorCmd.Flags().BoolP("fix", "f", false, "Attempt to fix detected issues")
	doctorCmd.Flags().Bool("yes", false, "Skip confirmation prompts (use with --fix)")
}
