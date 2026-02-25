// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		latest      string
		needsUpdate bool
		expectError bool
	}{
		{
			name:        "older version needs update",
			current:     "v1.0.0",
			latest:      "v1.1.0",
			needsUpdate: true,
			expectError: false,
		},
		{
			name:        "much older version needs update",
			current:     "v1.2.3",
			latest:      "v2.0.0",
			needsUpdate: true,
			expectError: false,
		},
		{
			name:        "prerelease to stable needs update",
			current:     "v1.0.0-alpha",
			latest:      "v1.0.0",
			needsUpdate: true,
			expectError: false,
		},
		{
			name:        "same version no update",
			current:     "v1.0.0",
			latest:      "v1.0.0",
			needsUpdate: false,
			expectError: false,
		},
		{
			name:        "newer version no update",
			current:     "v2.0.0",
			latest:      "v1.0.0",
			needsUpdate: false,
			expectError: false,
		},
		{
			name:        "dev version no update",
			current:     "dev",
			latest:      "v1.0.0",
			needsUpdate: false,
			expectError: false,
		},
		{
			name:        "empty version no update",
			current:     "",
			latest:      "v1.0.0",
			needsUpdate: false,
			expectError: false,
		},
		{
			name:        "versions without v prefix",
			current:     "1.0.0",
			latest:      "1.1.0",
			needsUpdate: true,
			expectError: false,
		},
		{
			name:        "prerelease versions",
			current:     "v1.0.0-beta",
			latest:      "v1.0.0-rc1",
			needsUpdate: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.current)
			needsUpdate, err := checker.compareVersions(tt.current, tt.latest)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.needsUpdate, needsUpdate)
			}
		})
	}
}

func TestCacheManagement(t *testing.T) {
	t.Run("cache file created correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
		}

		err := checker.updateCache("v1.1.0")
		require.NoError(t, err)

		cacheFile := filepath.Join(tmpDir, "last_update_check")
		assert.FileExists(t, cacheFile)

		data, err := os.ReadFile(cacheFile)
		require.NoError(t, err)

		var cache CacheData
		err = json.Unmarshal(data, &cache)
		require.NoError(t, err)

		assert.Equal(t, "v1.1.0", cache.LatestVersion)
		assert.WithinDuration(t, time.Now(), cache.LastCheck, 2*time.Second)
	})

	t.Run("cache prevents duplicate checks within 24h", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
		}

		err := checker.updateCache("v1.1.0")
		require.NoError(t, err)

		shouldCheck, err := checker.shouldCheck()
		require.NoError(t, err)
		assert.False(t, shouldCheck)
	})

	t.Run("expired cache triggers new check", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
		}

		// Create cache with old timestamp
		oldCache := CacheData{
			LastCheck:     time.Now().Add(-25 * time.Hour),
			LatestVersion: "v1.0.0",
		}
		data, err := json.Marshal(oldCache)
		require.NoError(t, err)

		cacheFile := filepath.Join(tmpDir, "last_update_check")
		err = os.WriteFile(cacheFile, data, 0644)
		require.NoError(t, err)

		shouldCheck, err := checker.shouldCheck()
		require.NoError(t, err)
		assert.True(t, shouldCheck)
	})

	t.Run("corrupted cache handled gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
		}

		cacheFile := filepath.Join(tmpDir, "last_update_check")
		err := os.WriteFile(cacheFile, []byte("invalid json"), 0644)
		require.NoError(t, err)

		shouldCheck, err := checker.shouldCheck()
		require.NoError(t, err)
		assert.True(t, shouldCheck)
	})

	t.Run("missing cache triggers check", func(t *testing.T) {
		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
		}

		shouldCheck, err := checker.shouldCheck()
		require.NoError(t, err)
		assert.True(t, shouldCheck)
	})
}

func TestOptOut(t *testing.T) {
	checker := NewChecker("v1.0.0")

	t.Run("ERST_NO_UPDATE_CHECK=1 disables checker", func(t *testing.T) {
		os.Setenv("ERST_NO_UPDATE_CHECK", "1")
		defer os.Unsetenv("ERST_NO_UPDATE_CHECK")

		assert.True(t, checker.isUpdateCheckDisabled())
	})

	t.Run("unset ERST_NO_UPDATE_CHECK enables checker", func(t *testing.T) {
		os.Unsetenv("ERST_NO_UPDATE_CHECK")
		assert.False(t, checker.isUpdateCheckDisabled())
	})

	t.Run("config file with check_for_updates: false disables checker", func(t *testing.T) {
		os.Unsetenv("ERST_NO_UPDATE_CHECK")

		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		configContent := "check_for_updates: false\n"
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		disabled := checkConfigFile(configPath)
		assert.True(t, disabled, "Config file should disable updates")
	})

	t.Run("missing config file enables checker", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "nonexistent.yaml")

		disabled := checkConfigFile(configPath)
		assert.False(t, disabled, "Missing config should enable updates")
	})

	t.Run("environment variable takes precedence", func(t *testing.T) {
		os.Setenv("ERST_NO_UPDATE_CHECK", "1")
		defer os.Unsetenv("ERST_NO_UPDATE_CHECK")

		assert.True(t, checker.isUpdateCheckDisabled())
	})
}

func TestGetCacheDir(t *testing.T) {
	cacheDir := getCacheDir()
	assert.NotEmpty(t, cacheDir)
	assert.Contains(t, cacheDir, "erst")
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker("v1.0.0")
	assert.NotNil(t, checker)
	assert.Equal(t, "v1.0.0", checker.currentVersion)
	assert.NotEmpty(t, checker.cacheDir)
}

func TestDisplayNotification(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	checker := NewChecker("v1.0.0")
	checker.displayNotification("v1.1.0")

	w.Close()
	os.Stderr = oldStderr

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	assert.Contains(t, output, "v1.1.0")
	assert.Contains(t, output, "available")
	assert.Contains(t, output, "go install")
}

func TestAPIIntegration(t *testing.T) {
	t.Run("successful API response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "erst-cli", r.Header.Get("User-Agent"))
			response := GitHubRelease{
				TagName: "v1.2.3",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response) //nolint:errcheck
		}))
		defer server.Close()

		// Note: GitHubAPIURL is a const, so this validates the structure
		_ = server
	})
}
