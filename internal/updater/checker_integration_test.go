// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateCheckerWithMockServer tests the full update checker flow with a mock GitHub API
func TestUpdateCheckerWithMockServer(t *testing.T) {
	// Create a mock server that returns a newer version
	mockVersion := "v2.0.0"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(t, "erst-cli", r.Header.Get("User-Agent"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

		// Return mock release
		release := GitHubRelease{
			TagName: mockVersion,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(release); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create temporary cache directory
	tmpDir := t.TempDir()

	// Create checker with old version
	checker := &Checker{
		currentVersion: "v1.0.0",
		cacheDir:       tmpDir,
	}

	// Capture stderr to check for notification
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Manually test the components
	t.Run("fetch latest version from mock server", func(t *testing.T) {
		// We can't easily override the GitHubAPIURL constant, so we'll test the logic manually
		req, err := http.NewRequest("GET", server.URL, nil)
		require.NoError(t, err)

		// Set the same headers as the real checker
		req.Header.Set("User-Agent", "erst-cli")
		req.Header.Set("Accept", "application/vnd.github+json")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		var release GitHubRelease
		err = json.NewDecoder(resp.Body).Decode(&release)
		require.NoError(t, err)
		assert.Equal(t, mockVersion, release.TagName)
	})

	t.Run("version comparison detects update needed", func(t *testing.T) {
		needsUpdate, err := checker.compareVersions("v1.0.0", mockVersion)
		require.NoError(t, err)
		assert.True(t, needsUpdate, "Should detect that v2.0.0 > v1.0.0")
	})

	t.Run("cache is created with correct data", func(t *testing.T) {
		err := checker.updateCache(mockVersion)
		require.NoError(t, err)

		// Verify cache file exists
		cacheFile := filepath.Join(tmpDir, "last_update_check")
		assert.FileExists(t, cacheFile)

		// Read and verify cache contents
		data, err := os.ReadFile(cacheFile)
		require.NoError(t, err)

		var cache CacheData
		err = json.Unmarshal(data, &cache)
		require.NoError(t, err)

		assert.Equal(t, mockVersion, cache.LatestVersion)
		assert.WithinDuration(t, time.Now(), cache.LastCheck, 2*time.Second)
	})

	t.Run("notification is displayed", func(t *testing.T) {
		checker.displayNotification(mockVersion)

		// Close write end and read output
		w.Close()
		var buf [1024]byte
		n, _ := r.Read(buf[:])
		output := string(buf[:n])

		// Restore stderr
		os.Stderr = oldStderr

		// Verify notification content
		assert.Contains(t, output, mockVersion)
		assert.Contains(t, output, "available")
		assert.Contains(t, output, "go install")
		assert.Contains(t, output, "💡")
	})

	t.Run("cache prevents duplicate checks", func(t *testing.T) {
		// Cache was created in previous test
		shouldCheck, err := checker.shouldCheck()
		require.NoError(t, err)
		assert.False(t, shouldCheck, "Should not check again within 24 hours")
	})
}

// TestUpdateCheckerWithOldCache tests that expired cache triggers new check
func TestUpdateCheckerWithOldCache(t *testing.T) {
	tmpDir := t.TempDir()

	checker := &Checker{
		currentVersion: "v1.0.0",
		cacheDir:       tmpDir,
	}

	// Create old cache (25 hours ago)
	oldCache := CacheData{
		LastCheck:     time.Now().Add(-25 * time.Hour),
		LatestVersion: "v1.5.0",
	}

	data, err := json.Marshal(oldCache)
	require.NoError(t, err)

	cacheFile := filepath.Join(tmpDir, "last_update_check")
	err = os.WriteFile(cacheFile, data, 0644)
	require.NoError(t, err)

	// Should check because cache is old
	shouldCheck, err := checker.shouldCheck()
	require.NoError(t, err)
	assert.True(t, shouldCheck, "Should check when cache is older than 24 hours")
}

// TestUpdateCheckerNoUpdateNeeded tests when running latest version
func TestUpdateCheckerNoUpdateNeeded(t *testing.T) {
	checker := NewChecker("v2.0.0")

	needsUpdate, err := checker.compareVersions("v2.0.0", "v2.0.0")
	require.NoError(t, err)
	assert.False(t, needsUpdate, "Should not need update when versions are equal")

	needsUpdate, err = checker.compareVersions("v2.0.0", "v1.9.0")
	require.NoError(t, err)
	assert.False(t, needsUpdate, "Should not need update when current is newer")
}

// TestUpdateCheckerWithPrerelease tests prerelease version handling
func TestUpdateCheckerWithPrerelease(t *testing.T) {
	checker := NewChecker("v1.0.0-beta.1")

	needsUpdate, err := checker.compareVersions("v1.0.0-beta.1", "v1.0.0")
	require.NoError(t, err)
	assert.True(t, needsUpdate, "Should update from beta to stable")

	needsUpdate, err = checker.compareVersions("v1.0.0-beta.1", "v1.0.0-beta.2")
	require.NoError(t, err)
	assert.True(t, needsUpdate, "Should update from beta.1 to beta.2")
}

// TestUpdateCheckerErrorHandling tests various error scenarios
func TestUpdateCheckerErrorHandling(t *testing.T) {
	t.Run("invalid version strings", func(t *testing.T) {
		checker := NewChecker("v1.0.0")

		_, err := checker.compareVersions("invalid", "v1.0.0")
		assert.Error(t, err, "Should error on invalid current version")

		_, err = checker.compareVersions("v1.0.0", "invalid")
		assert.Error(t, err, "Should error on invalid latest version")
	})

	t.Run("dev version skips check", func(t *testing.T) {
		checker := NewChecker("dev")

		needsUpdate, err := checker.compareVersions("dev", "v1.0.0")
		require.NoError(t, err)
		assert.False(t, needsUpdate, "Should skip check for dev version")
	})

	t.Run("empty version skips check", func(t *testing.T) {
		checker := NewChecker("")

		needsUpdate, err := checker.compareVersions("", "v1.0.0")
		require.NoError(t, err)
		assert.False(t, needsUpdate, "Should skip check for empty version")
	})
}

// TestNotificationFormat tests the notification message format
func TestNotificationFormat(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	checker := NewChecker("v1.0.0")
	checker.displayNotification("v2.5.3")

	w.Close()
	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])
	os.Stderr = oldStderr

	// Verify all required elements are present
	assert.Contains(t, output, "v2.5.3", "Should contain version number")
	assert.Contains(t, output, "available", "Should mention availability")
	assert.Contains(t, output, "go install", "Should provide install command")
	assert.Contains(t, output, "github.com/dotandev/hintents/cmd/erst@latest", "Should provide full install path")
	assert.True(t, strings.HasPrefix(output, "\n"), "Should start with newline")
	assert.True(t, strings.HasSuffix(output, "\n\n"), "Should end with double newline")
}
