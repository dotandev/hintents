// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

const (
	defaultGuidedConfigNetwork   = "testnet"
	defaultGuidedConfigCachePath = ".erst/cache"
	defaultRPCProbeTimeout       = 3 * time.Second
)

var (
	configInitForceFlag       bool
	configInitNoProbeFlag     bool
	configInitOutputPathFlag  string
	configInitNetworkFlag     string
	configInitRPCURLsFlag     string
	configInitCachePathFlag   string
	configInitInteractiveFlag bool
)

var configCmd = &cobra.Command{
	Use:     "config",
	GroupID: "utility",
	Short:   "Manage Erst configuration",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a guided erst.toml configuration",
	Long: `Create a well-commented erst.toml file through a guided setup flow.

The wizard asks for:
  - Network
  - RPC URL list
  - Cache location

When probing is enabled, Erst will benchmark known RPC endpoints from the
current machine and suggest the fastest reachable URLs first.`,
	Args: cobra.NoArgs,
	RunE: runConfigInit,
}

type guidedConfigOptions struct {
	Force       bool
	Interactive bool
	NoProbe     bool
	OutputPath  string
	Network     string
	RPCURLs     []string
	CachePath   string
}

type rpcProbeResult struct {
	URL     string
	Latency time.Duration
}

var detectRecommendedRPCsFunc = detectRecommendedRPCs

func runConfigInit(cmd *cobra.Command, args []string) error {
	opts := guidedConfigOptions{
		Force:       configInitForceFlag,
		Interactive: configInitInteractiveFlag,
		NoProbe:     configInitNoProbeFlag,
		OutputPath:  configInitOutputPathFlag,
		Network:     strings.TrimSpace(configInitNetworkFlag),
		RPCURLs:     splitAndTrimCSV(configInitRPCURLsFlag),
		CachePath:   strings.TrimSpace(configInitCachePathFlag),
	}

	if opts.OutputPath == "" {
		opts.OutputPath = "erst.toml"
	}
	if opts.Network == "" {
		opts.Network = defaultGuidedConfigNetwork
	}
	if opts.CachePath == "" {
		opts.CachePath = defaultGuidedConfigCachePath
	}
	if !isValidInitNetwork(opts.Network) {
		return fmt.Errorf("invalid network %q (valid: public, testnet, futurenet, standalone)", opts.Network)
	}

	return runConfigInitWithOptions(cmd, opts)
}

func runConfigInitWithOptions(cmd *cobra.Command, opts guidedConfigOptions) error {
	if shouldRunConfigWizard(cmd, opts.Interactive) {
		if err := runConfigWizard(cmd, &opts); err != nil {
			return err
		}
	}

	if len(opts.RPCURLs) == 0 {
		opts.RPCURLs = fallbackRPCURLsForNetwork(opts.Network)
	}
	opts.RPCURLs = uniqueStrings(opts.RPCURLs)

	if len(opts.RPCURLs) == 0 {
		return fmt.Errorf("no RPC URLs available for network %q", opts.Network)
	}

	content := renderGuidedErstToml(opts)
	if err := writeGuidedConfigFile(opts.OutputPath, content, opts.Force); err != nil {
		return err
	}

	if err := ensureCacheDirectory(filepath.Dir(opts.OutputPath), opts.CachePath); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Generated %s\n", opts.OutputPath) //nolint:errcheck
	return nil
}

func shouldRunConfigWizard(cmd *cobra.Command, interactive bool) bool {
	if !interactive {
		return false
	}

	inFile, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}

	return isatty.IsTerminal(inFile.Fd())
}

func runConfigWizard(cmd *cobra.Command, opts *guidedConfigOptions) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintln(out, "Erst config init")                  //nolint:errcheck
	_, _ = fmt.Fprintln(out, "Press Enter to accept the default.") //nolint:errcheck

	network, err := promptNetwork(reader, out, opts.Network)
	if err != nil {
		return err
	}
	opts.Network = network

	recommended := opts.RPCURLs
	if len(recommended) == 0 {
		recommended = fallbackRPCURLsForNetwork(opts.Network)
		if !opts.NoProbe && opts.Network != "standalone" {
			if detected, detectErr := detectRecommendedRPCsFunc(cmd.Context(), opts.Network); detectErr == nil && len(detected) > 0 {
				recommended = detected
			}
		}
	}

	rpcPrompt := strings.Join(recommended, ", ")
	if opts.Network == "standalone" {
		rpcPrompt = defaultRPCURLForNetwork(opts.Network, "")
	}

	rpcInput, err := promptWithDefault(reader, out, "RPC URLs (comma-separated)", rpcPrompt)
	if err != nil {
		return err
	}
	opts.RPCURLs = uniqueStrings(splitAndTrimCSV(rpcInput))
	if len(opts.RPCURLs) == 0 {
		opts.RPCURLs = uniqueStrings(recommended)
	}

	cachePath, err := promptWithDefault(reader, out, "Cache location", opts.CachePath)
	if err != nil {
		return err
	}
	opts.CachePath = strings.TrimSpace(cachePath)

	return nil
}

func promptNetwork(reader *bufio.Reader, out io.Writer, defaultValue string) (string, error) {
	options := []string{"public", "testnet", "futurenet", "standalone"}
	_, _ = fmt.Fprintf(out, "Network [%s] (%s): ", defaultValue, strings.Join(options, ", ")) //nolint:errcheck
	input, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return defaultValue, nil
	}
	if !isValidInitNetwork(value) {
		return "", fmt.Errorf("invalid network %q", value)
	}
	return value, nil
}

func splitAndTrimCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func writeGuidedConfigFile(path, content string, force bool) error {
	if path == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return writeScaffoldFile(path, content, force)
}

func ensureCacheDirectory(baseDir, cachePath string) error {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		return nil
	}

	target := cachePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(baseDir, target)
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", target, err)
	}

	return nil
}

func fallbackRPCURLsForNetwork(network string) []string {
	switch network {
	case "public":
		return []string{
			"https://soroban.stellar.org",
			"https://mainnet.stellar.validationcloud.io/v1/soroban-rpc-demo",
		}
	case "testnet":
		return []string{
			"https://soroban-testnet.stellar.org",
		}
	case "futurenet":
		return []string{
			"https://rpc-futurenet.stellar.org",
			"https://soroban-futurenet.stellar.org",
		}
	case "standalone":
		return []string{
			"http://localhost:8000",
		}
	default:
		return []string{defaultRPCURLForNetwork(network, "")}
	}
}

func renderGuidedErstToml(opts guidedConfigOptions) string {
	network := opts.Network
	if network == "" {
		network = defaultGuidedConfigNetwork
	}

	rpcURLs := uniqueStrings(opts.RPCURLs)
	if len(rpcURLs) == 0 {
		rpcURLs = fallbackRPCURLsForNetwork(network)
	}
	primaryRPC := rpcURLs[0]
	passphrase := defaultPassphraseForNetwork(network, "")
	cachePath := opts.CachePath
	if cachePath == "" {
		cachePath = defaultGuidedConfigCachePath
	}

	var buf bytes.Buffer
	buf.WriteString("# Erst configuration generated by `erst config init`\n")
	buf.WriteString("#\n")
	buf.WriteString("# `rpc_url` is the primary endpoint used first.\n")
	buf.WriteString("# `rpc_urls` is an ordered failover list. When probing succeeds, it is sorted\n")
	buf.WriteString("# by measured latency from this machine so the closest endpoint is tried first.\n")
	buf.WriteString("# CLI flags and ERST_* environment variables override values in this file.\n\n")
	buf.WriteString("rpc_url = " + strconv.Quote(primaryRPC) + "\n")
	buf.WriteString("rpc_urls = " + renderTomlArray(rpcURLs) + "\n\n")
	buf.WriteString("# Network selection controls defaults like the network passphrase.\n")
	buf.WriteString("network = " + strconv.Quote(network) + "\n")
	buf.WriteString("network_passphrase = " + strconv.Quote(passphrase) + "\n\n")
	buf.WriteString("# Cache directory for fetched ledger data and decoded artifacts.\n")
	buf.WriteString("cache_path = " + strconv.Quote(cachePath) + "\n\n")
	buf.WriteString("# Optional quality-of-life settings.\n")
	buf.WriteString("log_level = \"info\"\n")
	buf.WriteString("request_timeout = 15\n\n")
	buf.WriteString("# Optional: point to a locally built simulator binary.\n")
	buf.WriteString("# simulator_path = \"/absolute/path/to/erst-sim\"\n")

	return buf.String()
}

func renderTomlArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func detectRecommendedRPCs(ctx context.Context, network string) ([]string, error) {
	candidates := uniqueStrings(fallbackRPCURLsForNetwork(network))
	if len(candidates) == 0 {
		return nil, nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, defaultRPCProbeTimeout)
	defer cancel()

	results := make(chan rpcProbeResult, len(candidates))
	for _, url := range candidates {
		go func(target string) {
			if latency, err := probeRPCURL(probeCtx, target); err == nil {
				results <- rpcProbeResult{URL: target, Latency: latency}
			}
		}(url)
	}

	collected := make([]rpcProbeResult, 0, len(candidates))
	for range candidates {
		select {
		case result := <-results:
			collected = append(collected, result)
		case <-probeCtx.Done():
			goto done
		}
	}

done:
	if len(collected) == 0 {
		return candidates, probeCtx.Err()
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].Latency == collected[j].Latency {
			return collected[i].URL < collected[j].URL
		}
		return collected[i].Latency < collected[j].Latency
	})

	recommended := make([]string, 0, len(collected))
	for _, result := range collected {
		recommended = append(recommended, result.URL)
	}

	for _, candidate := range candidates {
		found := false
		for _, url := range recommended {
			if url == candidate {
				found = true
				break
			}
		}
		if !found {
			recommended = append(recommended, candidate)
		}
	}

	return recommended, nil
}

func probeRPCURL(ctx context.Context, url string) (time.Duration, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getHealth",
	})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultRPCProbeTimeout}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return 0, fmt.Errorf("health probe failed: HTTP %d", resp.StatusCode)
	}

	return time.Since(start), nil
}

func init() {
	configInitCmd.Flags().BoolVar(&configInitForceFlag, "force", false, "Overwrite erst.toml if it already exists")
	configInitCmd.Flags().BoolVar(&configInitNoProbeFlag, "no-probe", false, "Skip RPC endpoint probing and use built-in defaults")
	configInitCmd.Flags().BoolVar(&configInitInteractiveFlag, "interactive", true, "Run the guided setup prompts")
	configInitCmd.Flags().StringVar(&configInitOutputPathFlag, "output", "erst.toml", "Path to the generated TOML file")
	configInitCmd.Flags().StringVar(&configInitNetworkFlag, "network", defaultGuidedConfigNetwork, "Default network to configure")
	configInitCmd.Flags().StringVar(&configInitRPCURLsFlag, "rpc-urls", "", "Comma-separated RPC URLs to write without probing")
	configInitCmd.Flags().StringVar(&configInitCachePathFlag, "cache-path", defaultGuidedConfigCachePath, "Cache path to write into erst.toml")

	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
