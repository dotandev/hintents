// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	initForceFlag   bool
	initNetworkName string
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Scaffold a local Erst debugging workspace",
	Long: `Create project-local scaffolding for Erst debugging workflows.

This command generates:
  - erst.toml
  - .gitignore entries for local artifacts
  - a small directory structure for traces, snapshots, overrides, and WASM files`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) == 1 {
			targetDir = args[0]
		}

		if !isValidInitNetwork(initNetworkName) {
			return fmt.Errorf("invalid network %q (valid: public, testnet, futurenet, standalone)", initNetworkName)
		}

		if err := scaffoldErstProject(targetDir, initScaffoldOptions{
			Force:   initForceFlag,
			Network: initNetworkName,
		}); err != nil {
			return err
		}

		fmt.Printf("Initialized Erst project scaffold in %s\n", targetDir)
		return nil
	},
}

type initScaffoldOptions struct {
	Force   bool
	Network string
}

func scaffoldErstProject(targetDir string, opts initScaffoldOptions) error {
	root := targetDir
	if root == "" {
		root = "."
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	dirs := []string{
		".erst/cache",
		".erst/snapshots",
		".erst/traces",
		"overrides",
		"wasm",
	}
	for _, rel := range dirs {
		if err := os.MkdirAll(filepath.Join(root, rel), 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", rel, err)
		}
	}

	if err := writeScaffoldFile(filepath.Join(root, "erst.toml"), renderProjectErstToml(opts.Network), opts.Force); err != nil {
		return err
	}

	if err := ensureGitignoreBlock(filepath.Join(root, ".gitignore"), renderProjectGitignoreBlock()); err != nil {
		return err
	}

	return nil
}

func writeScaffoldFile(path, content string, force bool) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) == content {
			return nil
		}
		if !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", filepath.Base(path))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func ensureGitignoreBlock(path, block string) error {
	const marker = "# Erst local debugging artifacts"

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read .gitignore: %w", err)
	}

	if errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(path, []byte(block), 0644)
	}

	content := string(existing)
	if strings.Contains(content, marker) {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + block

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}
	return nil
}

func renderProjectErstToml(network string) string {
	if network == "" {
		network = "testnet"
	}

	rpcURL := map[string]string{
		"public":     "https://soroban.stellar.org",
		"testnet":    "https://soroban-testnet.stellar.org",
		"futurenet":  "https://soroban-futurenet.stellar.org",
		"standalone": "http://localhost:8000",
	}[network]
	if rpcURL == "" {
		rpcURL = "https://soroban-testnet.stellar.org"
	}

	return fmt.Sprintf(`# Erst project configuration for local debugging workflows
# CLI flags and environment variables override these values.

rpc_url = "%s"
network = "%s"
log_level = "info"
cache_path = ".erst/cache"

# Optional: point to a locally built simulator binary
# simulator_path = "./erst-sim"
`, rpcURL, network)
}

func renderProjectGitignoreBlock() string {
	return `# Erst local debugging artifacts
.erst/cache/
.erst/snapshots/
.erst/traces/
*.trace.json
*.flamegraph.svg
`
}

func isValidInitNetwork(network string) bool {
	if network == "" {
		return true
	}
	valid := []string{"public", "testnet", "futurenet", "standalone"}
	for _, candidate := range valid {
		if candidate == network {
			return true
		}
	}
	return false
}

func init() {
	initCmd.Flags().BoolVar(&initForceFlag, "force", false, "Overwrite generated files when they already exist")
	initCmd.Flags().StringVar(&initNetworkName, "network", "testnet", "Default network to write into erst.toml (public, testnet, futurenet, standalone)")
	rootCmd.AddCommand(initCmd)
}
