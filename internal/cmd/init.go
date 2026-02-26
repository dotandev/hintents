// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	initForceFlag             bool
	initNetworkName           string
	initRPCURLFlag            string
	initNetworkPassphraseFlag string
	initInteractiveFlag       bool
)

var initCmd = &cobra.Command{
	Use:     "init [directory]",
	GroupID: "development",
	Short:   "Scaffold a local Erst debugging workspace",
	Long: `Create project-local scaffolding for Erst debugging workflows.

This command generates:
  - erst.toml
  - .gitignore entries for local artifacts
  - a small directory structure for traces, snapshots, overrides, and WASM files

When run in an interactive terminal, it launches a setup wizard to configure
the preferred RPC URL and network passphrase.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) == 1 {
			targetDir = args[0]
		}

		opts := initScaffoldOptions{
			Force:             initForceFlag,
			Network:           initNetworkName,
			RPCURL:            initRPCURLFlag,
			NetworkPassphrase: initNetworkPassphraseFlag,
		}

		if !isValidInitNetwork(opts.Network) {
			return fmt.Errorf("invalid network %q (valid: public, testnet, futurenet, standalone)", opts.Network)
		}

		if shouldRunInitWizard(cmd, initInteractiveFlag) {
			if err := runInitWizard(cmd, &opts); err != nil {
				return err
			}
		}

		if err := scaffoldErstProject(targetDir, opts); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Initialized Erst project scaffold in %s\n", targetDir)
		return nil
	},
}

type initScaffoldOptions struct {
	Force             bool
	Network           string
	RPCURL            string
	NetworkPassphrase string
}

func shouldRunInitWizard(cmd *cobra.Command, interactive bool) bool {
	if !interactive {
		return false
	}

	inFile, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}

	return isatty.IsTerminal(inFile.Fd())
}

func runInitWizard(cmd *cobra.Command, opts *initScaffoldOptions) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Erst init setup wizard")
	fmt.Fprintln(out, "Press Enter to accept defaults.")

	rpcURL, err := promptWithDefault(reader, out, "Preferred Soroban RPC URL", defaultRPCURLForNetwork(opts.Network, opts.RPCURL))
	if err != nil {
		return err
	}
	passphrase, err := promptWithDefault(reader, out, "Network passphrase", defaultPassphraseForNetwork(opts.Network, opts.NetworkPassphrase))
	if err != nil {
		return err
	}

	opts.RPCURL = rpcURL
	opts.NetworkPassphrase = passphrase

	return nil
}

func promptWithDefault(reader *bufio.Reader, out io.Writer, prompt, defaultValue string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", prompt, defaultValue)
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value := strings.TrimSpace(input)
	if value == "" {
		return defaultValue, nil
	}

	return value, nil
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

	if err := writeScaffoldFile(filepath.Join(root, "erst.toml"), renderProjectErstToml(opts), opts.Force); err != nil {
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

func defaultRPCURLForNetwork(network, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}

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

	return rpcURL
}

func defaultPassphraseForNetwork(network, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}

	if network == "" {
		network = "testnet"
	}

	passphrase := map[string]string{
		"public":     "Public Global Stellar Network ; September 2015",
		"testnet":    "Test SDF Network ; September 2015",
		"futurenet":  "Test SDF Future Network ; October 2022",
		"standalone": "Standalone Network ; February 2017",
	}[network]
	if passphrase == "" {
		passphrase = "Test SDF Network ; September 2015"
	}

	return passphrase
}

func renderProjectErstToml(opts initScaffoldOptions) string {
	network := opts.Network
	if network == "" {
		network = "testnet"
	}

	rpcURL := defaultRPCURLForNetwork(network, opts.RPCURL)
	passphrase := defaultPassphraseForNetwork(network, opts.NetworkPassphrase)

	return fmt.Sprintf(`# Erst project configuration for local debugging workflows
# CLI flags and environment variables override these values.

rpc_url = %s
network = %s
network_passphrase = %s
log_level = "info"
cache_path = ".erst/cache"

# Optional: point to a locally built simulator binary
# simulator_path = "./erst-sim"
`, strconv.Quote(rpcURL), strconv.Quote(network), strconv.Quote(passphrase))
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
	initCmd.Flags().BoolVar(&initInteractiveFlag, "interactive", true, "Run an interactive setup wizard for RPC URL and network passphrase")
	initCmd.Flags().StringVar(&initNetworkName, "network", "testnet", "Default network to write into erst.toml (public, testnet, futurenet, standalone)")
	initCmd.Flags().StringVar(&initRPCURLFlag, "rpc-url", "", "RPC URL to write into erst.toml (skips wizard default for this value)")
	initCmd.Flags().StringVar(&initNetworkPassphraseFlag, "network-passphrase", "", "Network passphrase to write into erst.toml (skips wizard default for this value)")

	_ = initCmd.RegisterFlagCompletionFunc("network", completeInitNetworkFlag)

	rootCmd.AddCommand(initCmd)
}
