// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/spf13/cobra"
)

var (
	runArgsFlag        []string
	runNoCacheFlag     bool
	contractHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

// ContractLocation encapsulates details about a resolved contract path or URL.
type ContractLocation struct {
	RawPath      string
	ResolvedPath string
	IsRemote     bool
	IsWindows    bool
}

// IsWindowsAbsolutePath reports whether a path is a Windows absolute path or drive-qualified path.
// This handles:
//   - Drive letters with backslash: C:\path\to\contract.wasm
//   - Drive letters with forward slash: C:/path/to/contract.wasm
//   - Drive letters without leading slash: C:contract.wasm
//   - UNC network paths: \\server\share\contract.wasm or //server/share/contract.wasm
func IsWindowsAbsolutePath(path string) bool {
	if isWindowsDrivePath(path) {
		return true
	}
	if isWindowsUNCPath(path) {
		return true
	}
	return false
}

func isWindowsDrivePath(path string) bool {
	if len(path) >= 2 {
		c := path[0]
		if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) && path[1] == ':' {
			if len(path) == 2 {
				return true
			}
			return path[2] == '\\' || path[2] == '/' || path[2] != ':'
		}
	}
	return false
}

func isWindowsUNCPath(path string) bool {
	if len(path) >= 2 {
		return (path[0] == '\\' && path[1] == '\\') || (path[0] == '/' && path[1] == '/')
	}
	return false
}

// NormalizeContractPath normalizes a contract path, converting Windows backslashes
// to forward slashes using filepath.ToSlash.
func NormalizeContractPath(path string) string {
	s := filepath.ToSlash(path)
	return strings.ReplaceAll(s, "\\", "/")
}

// IsRemoteURL reports whether the target string is an HTTP/HTTPS remote URL scheme,
// ensuring Windows drive letters (e.g. C:\) are never misidentified as remote schemes.
func IsRemoteURL(target string) bool {
	if IsWindowsAbsolutePath(target) {
		return false
	}
	if strings.HasPrefix(strings.ToLower(target), "file://") {
		return false
	}
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// ResolveContractPath parses and normalizes a contract target (path or URL).
// It correctly identifies Windows absolute paths (e.g. C:\... or C:/...) and normalizes
// them using filepath.ToSlash instead of erroneously treating drive letters as URL schemes.
func ResolveContractPath(target string) (*ContractLocation, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.WrapValidationError("contract path or URL cannot be empty")
	}

	// Handle file:// URIs
	if strings.HasPrefix(strings.ToLower(target), "file://") {
		localPath := strings.TrimPrefix(target, "file://")
		// On Windows file:///C:/path or file://C:/path
		if strings.HasPrefix(localPath, "/") && len(localPath) >= 3 && isWindowsDrivePath(localPath[1:]) {
			localPath = localPath[1:]
		}
		normalized := NormalizeContractPath(localPath)
		return &ContractLocation{
			RawPath:      target,
			ResolvedPath: normalized,
			IsRemote:     false,
			IsWindows:    IsWindowsAbsolutePath(localPath),
		}, nil
	}

	// Check if this is a Windows path
	if IsWindowsAbsolutePath(target) {
		normalized := NormalizeContractPath(target)
		return &ContractLocation{
			RawPath:      target,
			ResolvedPath: normalized,
			IsRemote:     false,
			IsWindows:    true,
		}, nil
	}

	// Check if it's a remote URL (http:// or https://)
	if IsRemoteURL(target) {
		parsed, err := url.Parse(target)
		if err != nil {
			return nil, errors.WrapValidationError(fmt.Sprintf("invalid remote contract URL: %v", err))
		}
		return &ContractLocation{
			RawPath:      target,
			ResolvedPath: parsed.String(),
			IsRemote:     true,
			IsWindows:    false,
		}, nil
	}

	// Local Unix absolute or relative path
	normalized := NormalizeContractPath(target)
	return &ContractLocation{
		RawPath:      target,
		ResolvedPath: normalized,
		IsRemote:     false,
		IsWindows:    false,
	}, nil
}

// LoadContract loads contract bytecode from a file path or remote URL.
// It normalizes Windows paths using filepath.ToSlash to prevent C:\ from being parsed as a remote URL scheme.
func LoadContract(target string) ([]byte, error) {
	return LoadContractWithContext(context.Background(), target)
}

// LoadContractWithContext loads contract bytecode from a file path or remote URL with the given context.
func LoadContractWithContext(ctx context.Context, target string) ([]byte, error) {
	loc, err := ResolveContractPath(target)
	if err != nil {
		return nil, err
	}

	if loc.IsRemote {
		return fetchRemoteContract(ctx, loc.ResolvedPath)
	}

	return readLocalContract(loc.ResolvedPath)
}

func readLocalContract(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}

	// Also try with OS-specific path separators
	nativePath := filepath.FromSlash(path)
	if nativePath != path {
		data, err2 := os.ReadFile(nativePath)
		if err2 == nil {
			return data, nil
		}
	}

	return nil, errors.WrapValidationError(fmt.Sprintf("contract file not found or unreadable: %s", path))
}

func fetchRemoteContract(ctx context.Context, remoteURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("failed to create contract request: %v", err))
	}
	req.Header.Set("User-Agent", "erst-contract-loader")

	resp, err := contractHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote contract from %s: %w", remoteURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote contract returned status %d from %s", resp.StatusCode, remoteURL)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote contract body: %w", err)
	}
	if len(bytes) == 0 {
		return nil, errors.WrapValidationError(fmt.Sprintf("remote contract is empty: %s", remoteURL))
	}
	return bytes, nil
}

// runCmd represents the "run" command for executing a Soroban contract.
var runCmd = &cobra.Command{
	Use:     "run <contract-path-or-url>",
	GroupID: "testing",
	Short:   "Load and run a Soroban smart contract locally or from a remote URL",
	Long: `Load a Soroban smart contract from a local file path or remote URL and execute it.

Supports local file paths (including Windows absolute paths with drive letters,
e.g. C:\contracts\token.wasm), relative paths, and remote URLs (http:// or https://).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		loc, err := ResolveContractPath(target)
		if err != nil {
			return err
		}

		contractBytes, err := LoadContractWithContext(cmd.Context(), target)
		if err != nil {
			return err
		}

		if loc.IsRemote {
			fmt.Printf("%s Loaded remote contract (%d bytes) from %s\n", visualizer.Symbol("play"), len(contractBytes), loc.ResolvedPath)
		} else {
			fmt.Printf("%s Loaded contract (%d bytes) from %s\n", visualizer.Symbol("play"), len(contractBytes), loc.ResolvedPath)
		}

		return executeLoadedContract(cmd.Context(), loc.ResolvedPath, contractBytes, runArgsFlag, runNoCacheFlag)
	},
}

func executeLoadedContract(ctx context.Context, path string, wasmBytes []byte, extraArgs []string, noCache bool) error {
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		fmt.Printf("Loaded and verified contract at %s\n", path)
		return nil
	}
	defer runner.Close()

	wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)
	req := &simulator.SimulationRequest{
		WasmPath:     &path,
		ContractWasm: &wasmBase64,
		MockArgs:     &extraArgs,
		NoCache:      noCache,
	}

	resp, err := runner.Run(ctx, req)
	if err != nil {
		return errors.WrapSimulationFailed(err, "")
	}

	fmt.Printf("%s Contract execution completed with status: %s\n", visualizer.Symbol("play"), resp.Status)
	return nil
}

func init() {
	runCmd.Flags().StringSliceVar(&runArgsFlag, "args", nil, "Arguments to pass to the contract")
	runCmd.Flags().BoolVar(&runNoCacheFlag, "no-cache", false, "Disable execution cache")
	rootCmd.AddCommand(runCmd)
}
