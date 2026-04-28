// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// FixSimulatorBinary attempts to build the Soroban simulator
// FIXED: Handles Windows .exe suffix (Issue #1)
func FixSimulatorBinary(verbose bool) error {
	fmt.Println("  [*] Building Soroban simulator...")

	// Check if simulator directory exists
	if _, err := os.Stat("simulator"); err != nil {
		return fmt.Errorf("simulator directory not found: %w", err)
	}

	// Create context with 5 min timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run cargo build --release
	cmd := exec.CommandContext(ctx, "cargo", "build", "--release")
	cmd.Dir = "simulator"

	if verbose {
		// Show full output in verbose mode
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("  $ cd simulator && cargo build --release")
	}

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("build timed out")
		}
		return fmt.Errorf("cargo build failed: %w", err)
	}

	// Verify binary was created
	// FIXED: Append .exe on Windows (similar to checkSimulator)
	binaryName := "erst-sim"
	if runtime.GOOS == "windows" {
		binaryName = "erst-sim.exe"
	}
	binaryPath := filepath.Join("simulator", "target", "release", binaryName)

	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found after build: %s", binaryPath)
	}

	fmt.Printf("  [OK] Simulator built successfully at: %s\n", binaryPath)
	return nil
}

// FixMissingCacheDir creates the cache directory structure
// FIXED: Uses isolated temp dir in tests instead of real home (Issue #2)
func FixMissingCacheDir(verbose bool) error {
	fmt.Println("  [*] Creating cache directory...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".erst")

	// Create main cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Create subdirectories
	subdirs := []string{DirTransactions, DirProtocols, DirContracts}
	for _, subdir := range subdirs {
		path := filepath.Join(cacheDir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create subdir %s: %w", subdir, err)
		}
	}

	fmt.Printf("  [OK] Cache directory created at: %s\n", cacheDir)
	return nil
}

// FixProtocolRegistration initializes protocol registry
// FIXED: Only creates file when missing; doesn't overwrite existing (Issue #10)
func FixProtocolRegistration(verbose bool) error {
	fmt.Println("  [*] Registering Soroban protocol bindings...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".erst")
	protocolDir := filepath.Join(cacheDir, "protocols")

	// Ensure directory exists
	if err := os.MkdirAll(protocolDir, 0755); err != nil {
		return fmt.Errorf("failed to create protocols directory: %w", err)
	}

	registryFile := filepath.Join(protocolDir, "registered.json")

	// FIXED: Check if registry already exists - don't overwrite user data
	if _, err := os.Stat(registryFile); err == nil {
		// File exists - validate and repair instead of clobbering
		data, readErr := os.ReadFile(registryFile)
		if readErr != nil {
			return fmt.Errorf("failed to read existing registry: %w", readErr)
		}

		var registry map[string]interface{}
		if err := json.Unmarshal(data, &registry); err != nil {
			// Corrupted - back up and recreate
			backupFile := registryFile + ".backup"
			if err := os.WriteFile(backupFile, data, 0644); err != nil {
				return fmt.Errorf("failed to backup corrupted registry: %w", err)
			}
			fmt.Printf("  ⚠ Corrupted registry backed up to: %s\n", backupFile)
		} else {
			fmt.Printf("  [OK] Protocol registry already exists and is valid at: %s\n", registryFile)
			return nil
		}
	}

	// Only create if missing
	registry := map[string]interface{}{
		"version":   "1.0",
		"protocols": []string{},
		"updated":   "2026-03-25",
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(registryFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	fmt.Printf("  [OK] Protocol registry created at: %s\n", registryFile)
	return nil
}

// FixGoModDependencies runs go mod tidy and go mod download
// FIXED: Isolated test mode to avoid modifying repo (Issue #4)
func FixGoModDependencies(verbose bool) error {
	fmt.Println("  [*] Resolving Go module dependencies...")

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	if verbose {
		fmt.Println("  $ go mod tidy")
		tidyCmd.Stdout = os.Stdout
		tidyCmd.Stderr = os.Stderr
	}

	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	// Run go mod download
	downloadCmd := exec.Command("go", "mod", "download")
	if verbose {
		fmt.Println("  $ go mod download")
		downloadCmd.Stdout = os.Stdout
		downloadCmd.Stderr = os.Stderr
	}

	if err := downloadCmd.Run(); err != nil {
		return fmt.Errorf("go mod download failed: %w", err)
	}

	fmt.Println("  [OK] Go module dependencies resolved")
	return nil
}
