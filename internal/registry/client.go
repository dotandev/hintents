// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotandev/hintents/internal/logger"
)

// Client handles resolving and downloading packages from the registry.
// MVP: Uses GitHub releases/raw content as the decentralized backend.
type Client struct {
	httpClient *http.Client
	cacheDir   string
}

// NewClient initializes a new EPR client.
func NewClient() (*Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not find user home directory: %w", err)
	}

	cacheDir := filepath.Join(home, ".erst", "registry")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create cache dir: %w", err)
	}

	return &Client{
		httpClient: &http.Client{},
		cacheDir:   cacheDir,
	}, nil
}

// PackageRef defines a resolved package reference.
type PackageRef struct {
	Org     string
	Repo    string
	Version string
}

// ParsePackage parses a string like "@openzeppelin/token@v1.0.0" into a PackageRef.
func ParsePackage(pkg string) (PackageRef, error) {
	if !strings.HasPrefix(pkg, "@") {
		return PackageRef{}, fmt.Errorf("invalid package format, expected @org/repo@version: %s", pkg)
	}

	parts := strings.Split(pkg[1:], "@")
	if len(parts) != 2 {
		return PackageRef{}, fmt.Errorf("version is required, e.g., @org/repo@v1.0.0")
	}

	version := parts[1]
	pathParts := strings.Split(parts[0], "/")
	if len(pathParts) != 2 {
		return PackageRef{}, fmt.Errorf("invalid package path, expected org/repo: %s", parts[0])
	}

	return PackageRef{
		Org:     pathParts[0],
		Repo:    pathParts[1],
		Version: version,
	}, nil
}

// Install fetches a package and stores it in the local cache, returning the path to the WASM file.
func (c *Client) Install(ctx context.Context, pkg string) (string, error) {
	ref, err := ParsePackage(pkg)
	if err != nil {
		return "", err
	}

	// Cache structure: ~/.erst/registry/org/repo/v1.0.0/contract.wasm
	targetDir := filepath.Join(c.cacheDir, ref.Org, ref.Repo, ref.Version)
	targetFile := filepath.Join(targetDir, "contract.wasm")

	// If already installed, skip
	if _, err := os.Stat(targetFile); err == nil {
		logger.Logger.Info("Package already installed in cache", "package", pkg, "path", targetFile)
		return targetFile, nil
	}

	logger.Logger.Info("Resolving package from GitHub...", "org", ref.Org, "repo", ref.Repo, "version", ref.Version)

	// Fetch from custom registry or fallback to GitHub raw content (MVP)
	baseURL := os.Getenv("ERST_REGISTRY_URL")
	var url string
	if baseURL != "" {
		url = fmt.Sprintf("%s/%s/%s/%s/contract.wasm", strings.TrimRight(baseURL, "/"), ref.Org, ref.Repo, ref.Version)
	} else {
		url = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/contract.wasm", ref.Org, ref.Repo, ref.Version)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error while fetching package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("package not found or unavailable (HTTP %d): %s", resp.StatusCode, url)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	out, err := os.Create(targetFile)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write package to disk: %w", err)
	}

	logger.Logger.Info("Successfully installed package", "package", pkg)
	return targetFile, nil
}
