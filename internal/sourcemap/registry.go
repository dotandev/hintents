// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

const (
	// StellarExpertBaseURL is the base URL for the stellar.expert explorer API.
	StellarExpertBaseURL = "https://api.stellar.expert/explorer"

	// DefaultRequestTimeout is the default timeout for HTTP requests.
	DefaultRequestTimeout = 30 * time.Second

	// MaxResponseSize is the maximum allowed response size (10 MB).
	MaxResponseSize = 10 * 1024 * 1024
)

// NetworkID identifies a Stellar network for API calls.
type NetworkID string

const (
	NetworkPublic  NetworkID = "public"
	NetworkTestnet NetworkID = "testnet"
)

// ContractInfo holds metadata about a verified contract from stellar.expert.
type ContractInfo struct {
	// ContractID is the Stellar contract address (C... format).
	ContractID string `json:"contract"`
	// WasmHash is the hex-encoded hash of the deployed WASM bytecode.
	WasmHash string `json:"wasm_hash"`
	// Repository is the URL of the source code repository (e.g. GitHub).
	Repository string `json:"repository"`
	// Verified indicates whether the contract source has been verified.
	Verified bool `json:"verified"`
	// Creator is the account that deployed the contract.
	Creator string `json:"creator"`
}

// SourceCode represents downloaded source code for a contract.
type SourceCode struct {
	// ContractID is the contract address this source belongs to.
	ContractID string
	// WasmHash is the hash of the WASM this source was verified against.
	WasmHash string
	// Repository is the source repository URL.
	Repository string
	// Files maps relative file paths to their contents.
	Files map[string]string
	// FetchedAt is the time this source was retrieved.
	FetchedAt time.Time
}

// WasmCode holds downloaded WASM bytecode for a contract.
type WasmCode struct {
	// WasmHash is the hash identifying this WASM code.
	WasmHash string
	// Code is the raw WASM bytecode.
	Code []byte
	// FetchedAt is the time this code was retrieved.
	FetchedAt time.Time
}

// RegistryClient fetches verified contract information and source code
// from public registries like stellar.expert.
type RegistryClient struct {
	httpClient *http.Client
	baseURL    string
	network    NetworkID
}

// RegistryOption is a functional option for configuring RegistryClient.
type RegistryOption func(*RegistryClient)

// WithHTTPClient sets a custom HTTP client for the registry.
func WithHTTPClient(client *http.Client) RegistryOption {
	return func(rc *RegistryClient) {
		rc.httpClient = client
	}
}

// WithBaseURL overrides the default stellar.expert API base URL.
func WithBaseURL(url string) RegistryOption {
	return func(rc *RegistryClient) {
		rc.baseURL = strings.TrimRight(url, "/")
	}
}

// WithNetwork sets the Stellar network to query.
func WithNetwork(network NetworkID) RegistryOption {
	return func(rc *RegistryClient) {
		rc.network = network
	}
}

// NewRegistryClient creates a new registry client for fetching verified source codes.
func NewRegistryClient(opts ...RegistryOption) *RegistryClient {
	rc := &RegistryClient{
		httpClient: &http.Client{
			Timeout: DefaultRequestTimeout,
		},
		baseURL: StellarExpertBaseURL,
		network: NetworkPublic,
	}
	for _, opt := range opts {
		opt(rc)
	}
	return rc
}

// GetContractInfo fetches contract metadata from stellar.expert.
// Returns nil with no error if the contract is not found or not verified.
func (rc *RegistryClient) GetContractInfo(ctx context.Context, contractID string) (*ContractInfo, error) {
	if err := validateContractID(contractID); err != nil {
		return nil, fmt.Errorf("invalid contract ID: %w", err)
	}

	url := fmt.Sprintf("%s/%s/contract/%s", rc.baseURL, rc.network, contractID)

	logger.Logger.Debug("Fetching contract info from registry",
		"contract_id", contractID,
		"url", url,
	)

	body, statusCode, err := rc.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contract info: %w", err)
	}

	if statusCode == http.StatusNotFound {
		logger.Logger.Debug("Contract not found in registry", "contract_id", contractID)
		return nil, nil
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from registry for contract %s", statusCode, contractID)
	}

	var info ContractInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse contract info response: %w", err)
	}

	info.ContractID = contractID

	logger.Logger.Info("Contract info retrieved",
		"contract_id", contractID,
		"wasm_hash", info.WasmHash,
		"verified", info.Verified,
		"repository", info.Repository,
	)

	return &info, nil
}

// GetWasmCode downloads the WASM bytecode for a contract from stellar.expert.
func (rc *RegistryClient) GetWasmCode(ctx context.Context, contractID string) (*WasmCode, error) {
	if err := validateContractID(contractID); err != nil {
		return nil, fmt.Errorf("invalid contract ID: %w", err)
	}

	url := fmt.Sprintf("%s/%s/contract/%s/wasm", rc.baseURL, rc.network, contractID)

	logger.Logger.Debug("Fetching WASM code from registry",
		"contract_id", contractID,
		"url", url,
	)

	body, statusCode, err := rc.doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch WASM code: %w", err)
	}

	if statusCode == http.StatusNotFound {
		return nil, nil
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching WASM for contract %s", statusCode, contractID)
	}

	return &WasmCode{
		Code:      body,
		FetchedAt: time.Now(),
	}, nil
}

// FetchVerifiedSource attempts to download verified source code for a contract.
// It first checks stellar.expert for contract info and verification status,
// then retrieves the source from the linked repository if available.
//
// Returns nil with no error if the contract is not found or not verified.
func (rc *RegistryClient) FetchVerifiedSource(ctx context.Context, contractID string) (*SourceCode, error) {
	info, err := rc.GetContractInfo(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up contract: %w", err)
	}

	if info == nil {
		logger.Logger.Debug("Contract not registered", "contract_id", contractID)
		return nil, nil
	}

	if !info.Verified {
		logger.Logger.Debug("Contract is not verified", "contract_id", contractID)
		return nil, nil
	}

	if info.Repository == "" {
		logger.Logger.Debug("No repository link for verified contract", "contract_id", contractID)
		return nil, nil
	}

	files, err := rc.fetchRepositorySource(ctx, info.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch source from repository: %w", err)
	}

	return &SourceCode{
		ContractID: contractID,
		WasmHash:   info.WasmHash,
		Repository: info.Repository,
		Files:      files,
		FetchedAt:  time.Now(),
	}, nil
}

// fetchRepositorySource downloads source files from a GitHub repository URL.
// It parses the repo URL, determines the default branch, and fetches Rust source files.
func (rc *RegistryClient) fetchRepositorySource(ctx context.Context, repoURL string) (map[string]string, error) {
	owner, repo, err := parseGitHubURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("unsupported repository URL %q: %w", repoURL, err)
	}

	logger.Logger.Debug("Fetching source from GitHub",
		"owner", owner,
		"repo", repo,
	)

	// Fetch repo metadata to determine default branch
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	body, statusCode, err := rc.doGet(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository metadata: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for %s/%s", statusCode, owner, repo)
	}

	var repoMeta struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &repoMeta); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub repo metadata: %w", err)
	}

	branch := repoMeta.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	// Fetch the file tree
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, branch)
	body, statusCode, err = rc.doGet(ctx, treeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository tree: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for tree of %s/%s", statusCode, owner, repo)
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int    `json:"size"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("failed to parse repository tree: %w", err)
	}

	// Filter to Soroban-relevant source files
	files := make(map[string]string)
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}
		if !isSourceFile(entry.Path) {
			continue
		}
		// Skip excessively large files
		if entry.Size > MaxResponseSize {
			logger.Logger.Warn("Skipping large file", "path", entry.Path, "size", entry.Size)
			continue
		}

		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, entry.Path)
		content, statusCode, err := rc.doGet(ctx, rawURL)
		if err != nil {
			logger.Logger.Warn("Failed to fetch file", "path", entry.Path, "error", err)
			continue
		}
		if statusCode != http.StatusOK {
			logger.Logger.Warn("Non-200 status for file", "path", entry.Path, "status", statusCode)
			continue
		}
		files[entry.Path] = string(content)
	}

	logger.Logger.Info("Source files downloaded",
		"owner", owner,
		"repo", repo,
		"file_count", len(files),
	)

	return files, nil
}

// doGet performs an HTTP GET request and returns the response body and status code.
func (rc *RegistryClient) doGet(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "erst/sourcemap")

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, resp.StatusCode, nil
}

// validateContractID checks that a contract ID looks valid (C... format, 56 chars).
func validateContractID(id string) error {
	if len(id) == 0 {
		return fmt.Errorf("contract ID is empty")
	}
	if !strings.HasPrefix(id, "C") {
		return fmt.Errorf("contract ID must start with 'C', got %q", id[:1])
	}
	if len(id) != 56 {
		return fmt.Errorf("contract ID must be 56 characters, got %d", len(id))
	}
	return nil
}

// parseGitHubURL extracts owner and repo from a GitHub URL.
// Supports formats like:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - github.com/owner/repo
func parseGitHubURL(rawURL string) (string, string, error) {
	s := rawURL
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	parts := strings.SplitN(s, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot extract owner/repo from %q", rawURL)
	}

	return parts[0], parts[1], nil
}

// isSourceFile returns true if the file path is a Soroban-relevant source file.
func isSourceFile(path string) bool {
	// Rust source files
	if strings.HasSuffix(path, ".rs") {
		return true
	}
	// Cargo manifest
	if strings.HasSuffix(path, "Cargo.toml") {
		return true
	}
	// Cargo lock (useful for reproducible builds)
	if strings.HasSuffix(path, "Cargo.lock") {
		return true
	}
	return false
}
