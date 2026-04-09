// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
)

const (
	// GitHubAPIURL is the endpoint for fetching the latest release
	GitHubAPIURL = "https://api.github.com/repos/dotandev/hintents/releases/latest"
	// CheckInterval is how often we check for updates (24 hours)
	CheckInterval = 24 * time.Hour
	// RequestTimeout is the maximum time to wait for GitHub API
	RequestTimeout = 5 * time.Second
)

// Checker handles update checking logic
type Checker struct {
	currentVersion string
	cacheDir       string
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

// CacheData stores the last check timestamp and latest version
type CacheData struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

// NewChecker creates a new update checker
func NewChecker(currentVersion string) *Checker {
	cacheDir := getCacheDir()
	return &Checker{
		currentVersion: currentVersion,
		cacheDir:       cacheDir,
	}
}

// CheckForUpdates runs the update check in a goroutine (non-blocking)
func (c *Checker) CheckForUpdates() {
	// Check if update checking is disabled
	if c.isUpdateCheckDisabled() {
		return
	}

	// Check if we should perform the check based on cache
	shouldCheck, err := c.shouldCheck()
	if err != nil || !shouldCheck {
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	// Fetch latest version from GitHub
	latestVersion, err := c.fetchLatestVersion(ctx)
	if err != nil {
		// Silent failure - don't bother the user
		return
	}

	// Update cache with the latest version (banner is shown from cache at next run start)
	if err := c.updateCache(latestVersion); err != nil {
		// Silent failure
		return
	}
	// Do not display here; banner is shown once per run from ShowBannerFromCache to avoid
	// mid-output or duplicate messages.
}

// shouldCheck determines if we should check based on cache
func (c *Checker) shouldCheck() (bool, error) {
	cacheFile := filepath.Join(c.cacheDir, "last_update_check")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		// Cache doesn't exist or can't be read - should check
		return true, nil
	}

	var cache CacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache - should check
		return true, nil
	}

	// Check if enough time has passed
	return time.Since(cache.LastCheck) >= CheckInterval, nil
}

// fetchLatestVersion calls GitHub API to get the latest release
func (c *Checker) fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubAPIURL, nil)
	if err != nil {
		return "", err
	}

	// Set User-Agent header (GitHub API requires it)
	req.Header.Set("User-Agent", "erst-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{
		Timeout: RequestTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Handle rate limiting or other errors silently
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// FetchReleaseInfo gets full information for a specific version or latest
func (c *Checker) FetchReleaseInfo(ctx context.Context, version string) (*GitHubRelease, error) {
	url := GitHubAPIURL
	if version != "" && version != "latest" {
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		url = fmt.Sprintf("https://api.github.com/repos/dotandev/hintents/releases/tags/%s", version)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "erst-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{
		Timeout: RequestTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found", version)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from GitHub: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// compareVersions compares current vs latest version
func (c *Checker) compareVersions(current, latest string) (bool, error) {
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Skip comparison if running dev version
	if current == "dev" || current == "" {
		return false, nil
	}

	currentVer, err := version.NewVersion(current)
	if err != nil {
		return false, err
	}

	latestVer, err := version.NewVersion(latest)
	if err != nil {
		return false, err
	}

	// Return true if latest is greater than current
	return latestVer.GreaterThan(currentVer), nil
}

// displayNotification prints the update message to stderr (small one-line banner)
func (c *Checker) displayNotification(latestVersion string) {
	message := fmt.Sprintf(
		"Upgrade available: %s — run 'go install github.com/dotandev/hintents/cmd/erst@latest' to update\n",
		latestVersion,
	)
	fmt.Fprint(os.Stderr, message)
}

func (c *Checker) PerformUpdate(ctx context.Context, version string) error {
	// For idempotency, if the version requested is already what we have, skip
	if version != "" && version != "latest" {
		v := strings.TrimPrefix(version, "v")
		cur := strings.TrimPrefix(c.currentVersion, "v")
		if v == cur {
			return nil // Already at version
		}
	}

	target := "github.com/dotandev/hintents/cmd/erst@latest"
	if version != "" && version != "latest" {
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		target = fmt.Sprintf("github.com/dotandev/hintents/cmd/erst@%s", version)
	}

	fmt.Printf("Updating to %s via 'go install'...\n", target)

	// Prepare go install command
	cmd := exec.CommandContext(ctx, "go", "install", target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("Update successful. Please restart erst.")
	return nil
}

// ShowBannerFromCache reads the last update check cache and, if a newer version
// was found, prints a small "Upgrade available" banner to stderr. Called at CLI
// start so the banner appears once per run without blocking. Skips if update
// checking is disabled or cache is missing/invalid.
func ShowBannerFromCache(currentVersion string) {
	c := NewChecker(currentVersion)
	c.showBannerFromCache()
}

// ShowBannerFromCacheWithCacheDir is for testing: same as ShowBannerFromCache
// but uses the given cache directory (full path to the erst cache dir, e.g. …/erst).
func ShowBannerFromCacheWithCacheDir(currentVersion, cacheDir string) {
	c := &Checker{currentVersion: currentVersion, cacheDir: cacheDir}
	c.showBannerFromCache()
}

// showBannerFromCache reads cache and displays the banner if cached latest > current.
func (c *Checker) showBannerFromCache() {
	if c.isUpdateCheckDisabled() {
		return
	}
	cacheFile := filepath.Join(c.cacheDir, "last_update_check")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}
	var cache CacheData
	if err := json.Unmarshal(data, &cache); err != nil || cache.LatestVersion == "" {
		return
	}
	needsUpdate, err := c.compareVersions(c.currentVersion, cache.LatestVersion)
	if err != nil || !needsUpdate {
		return
	}
	c.displayNotification(cache.LatestVersion)
}

// updateCache updates the cache file with the latest check time and version
func (c *Checker) updateCache(latestVersion string) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}

	cache := CacheData{
		LastCheck:     time.Now(),
		LatestVersion: latestVersion,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	cacheFile := filepath.Join(c.cacheDir, "last_update_check")
	return os.WriteFile(cacheFile, data, 0644)
}

// isUpdateCheckDisabled checks if the user has opted out
func (c *Checker) isUpdateCheckDisabled() bool {
	// Check environment variable (takes precedence)
	if os.Getenv("ERST_NO_UPDATE_CHECK") != "" {
		return true
	}

	// Check config file
	configPath := getConfigPath()
	if configPath != "" {
		if disabled := checkConfigFile(configPath); disabled {
			return true
		}
	}

	return false
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	// Try to use OS-specific config directory
	if configDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(configDir, "erst", "config.yaml")
	}

	// Fallback to home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".config", "erst", "config.yaml")
	}

	return ""
}

// checkConfigFile reads the config file and checks if updates are disabled
func checkConfigFile(configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file doesn't exist or can't be read - updates are enabled
		return false
	}

	// Simple YAML parsing - look for "check_for_updates: false"
	// This is a basic implementation that avoids adding a YAML dependency
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Check for "check_for_updates: false" or "check_for_updates:false"
		if strings.HasPrefix(line, "check_for_updates:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "check_for_updates:"))
			if value == "false" {
				return true
			}
		}
	}

	return false
}

// getCacheDir returns the appropriate cache directory for the platform
func getCacheDir() string {
	// Try to use OS-specific cache directory
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, "erst")
	}

	// Fallback to home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".cache", "erst")
	}

	// Last resort - use temp directory
	return filepath.Join(os.TempDir(), "erst")
}
