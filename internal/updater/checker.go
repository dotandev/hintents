// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

	// Update cache with the latest version
	if err := c.updateCache(latestVersion); err != nil {
		// Silent failure
		return
	}

	// Compare versions
	needsUpdate, err := c.compareVersions(c.currentVersion, latestVersion)
	if err != nil || !needsUpdate {
		return
	}

	// Display notification
	c.displayNotification(latestVersion)
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

// displayNotification prints the update message to stderr
func (c *Checker) displayNotification(latestVersion string) {
	message := fmt.Sprintf(
		"\n💡 A new version (%s) is available! Run 'go install github.com/dotandev/hintents/cmd/erst@latest' to update.\n\n",
		latestVersion,
	)
	fmt.Fprint(os.Stderr, message)
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
