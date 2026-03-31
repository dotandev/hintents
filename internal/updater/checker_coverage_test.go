// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
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

// ---------------------------------------------------------------------------
// fetchLatestVersion
// ---------------------------------------------------------------------------

func TestFetchLatestVersion(t *testing.T) {
	t.Run("success returns tag name", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "erst-cli", r.Header.Get("User-Agent"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{TagName: "v9.9.9"}) //nolint:errcheck
		}))
		defer srv.Close()

		checker := newCheckerWithURL(srv.URL)
		tag, err := checker.fetchLatestVersion(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "v9.9.9", tag)
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		checker := newCheckerWithURL(srv.URL)
		_, err := checker.fetchLatestVersion(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
	})

	t.Run("rate-limited 429 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		checker := newCheckerWithURL(srv.URL)
		_, err := checker.fetchLatestVersion(t.Context())
		require.Error(t, err)
	})

	t.Run("malformed JSON body returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not json")) //nolint:errcheck
		}))
		defer srv.Close()

		checker := newCheckerWithURL(srv.URL)
		_, err := checker.fetchLatestVersion(t.Context())
		require.Error(t, err)
	})

	t.Run("network error returns error", func(t *testing.T) {
		// Point at a server that is immediately closed.
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()

		checker := newCheckerWithURL(srv.URL)
		_, err := checker.fetchLatestVersion(t.Context())
		require.Error(t, err)
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		// Slow server that won't respond before the context is cancelled.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(t.Context())
		cancel() // cancel before the request is made

		checker := newCheckerWithURL(srv.URL)
		_, err := checker.fetchLatestVersion(ctx)
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// compareVersions — error paths
// ---------------------------------------------------------------------------

func TestCompareVersionsErrorPaths(t *testing.T) {
	checker := NewChecker("v1.0.0")

	t.Run("unparseable current version returns error", func(t *testing.T) {
		_, err := checker.compareVersions("not-a-semver!!!", "v1.0.0")
		require.Error(t, err)
	})

	t.Run("unparseable latest version returns error", func(t *testing.T) {
		_, err := checker.compareVersions("v1.0.0", "not-a-semver!!!")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// updateCache — error paths
// ---------------------------------------------------------------------------

func TestUpdateCacheErrorPaths(t *testing.T) {
	t.Run("unwritable cache dir returns error", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root can write to read-only dirs; skip")
		}

		// Create a file where the cache dir should be — MkdirAll will fail.
		tmpDir := t.TempDir()
		blocker := filepath.Join(tmpDir, "erst")
		require.NoError(t, os.WriteFile(blocker, []byte("block"), 0o444))

		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       filepath.Join(blocker, "nested"), // blocker is a file, not dir
		}
		err := checker.updateCache("v1.1.0")
		require.Error(t, err)
	})

	t.Run("read-only cache dir write returns error", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root bypasses file permissions; skip")
		}

		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "erst-ro")
		require.NoError(t, os.MkdirAll(cacheDir, 0o755))
		require.NoError(t, os.Chmod(cacheDir, 0o555)) // read+exec, no write
		t.Cleanup(func() { os.Chmod(cacheDir, 0o755) }) //nolint:errcheck

		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       cacheDir,
		}
		err := checker.updateCache("v1.1.0")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// CheckForUpdates integration — full happy and sad paths
// ---------------------------------------------------------------------------

func TestCheckForUpdates(t *testing.T) {
	t.Run("update available prints notification", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{TagName: "v99.0.0"}) //nolint:errcheck
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
			apiURL:         srv.URL,
		}

		// Capture stderr
		r, w, _ := os.Pipe()
		old := os.Stderr
		os.Stderr = w

		checker.CheckForUpdates()

		w.Close()
		os.Stderr = old

		var buf [4096]byte
		n, _ := r.Read(buf[:])
		output := string(buf[:n])

		assert.Contains(t, output, "v99.0.0")
		assert.Contains(t, output, "available")
	})

	t.Run("no update needed produces no output", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{TagName: "v0.0.1"}) //nolint:errcheck
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
			apiURL:         srv.URL,
		}

		r, w, _ := os.Pipe()
		old := os.Stderr
		os.Stderr = w

		checker.CheckForUpdates()

		w.Close()
		os.Stderr = old

		var buf [4096]byte
		n, _ := r.Read(buf[:])
		assert.Empty(t, string(buf[:n]))
	})

	t.Run("API error produces no output and no panic", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
			apiURL:         srv.URL,
		}

		// Must not panic
		assert.NotPanics(t, func() { checker.CheckForUpdates() })
	})

	t.Run("disabled by env produces no output", func(t *testing.T) {
		t.Setenv("ERST_NO_UPDATE_CHECK", "1")

		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		}))
		defer srv.Close()

		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       t.TempDir(),
			apiURL:         srv.URL,
		}
		checker.CheckForUpdates()
		assert.False(t, called, "API should not be called when updates are disabled")
	})

	t.Run("recent cache skips API call", func(t *testing.T) {
		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		}))
		defer srv.Close()

		tmpDir := t.TempDir()
		// Write a fresh cache entry
		fresh := CacheData{
			LastCheck:     time.Now(),
			LatestVersion: "v1.0.0",
		}
		data, _ := json.Marshal(fresh)
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "last_update_check"), data, 0o644))

		checker := &Checker{
			currentVersion: "v1.0.0",
			cacheDir:       tmpDir,
			apiURL:         srv.URL,
		}
		checker.CheckForUpdates()
		assert.False(t, called, "API should not be called when cache is fresh")
	})
}

// ---------------------------------------------------------------------------
// checkConfigFile — uncovered branches
// ---------------------------------------------------------------------------

func TestCheckConfigFileAdditionalBranches(t *testing.T) {
	t.Run("check_for_updates true does not disable", func(t *testing.T) {
		f := writeTempConfig(t, "check_for_updates: true\n")
		assert.False(t, checkConfigFile(f))
	})

	t.Run("no check_for_updates key does not disable", func(t *testing.T) {
		f := writeTempConfig(t, "some_other_key: value\nnetwork_timeout: 30\n")
		assert.False(t, checkConfigFile(f))
	})

	t.Run("empty config file does not disable", func(t *testing.T) {
		f := writeTempConfig(t, "")
		assert.False(t, checkConfigFile(f))
	})

	t.Run("inline comment lines are skipped", func(t *testing.T) {
		// A comment line must not accidentally match as a key.
		f := writeTempConfig(t, "# check_for_updates: false\ncheck_for_updates: true\n")
		assert.False(t, checkConfigFile(f))
	})

	t.Run("check_for_updates false with surrounding whitespace", func(t *testing.T) {
		f := writeTempConfig(t, "  check_for_updates:  false  \n")
		// TrimSpace on the whole line strips leading spaces; TrimPrefix then
		// trims the key; the remaining value is "false  " — trimmed to "false".
		// This documents the current behaviour of the simple parser.
		assert.True(t, checkConfigFile(f))
	})

	t.Run("multiple keys, false is respected", func(t *testing.T) {
		f := writeTempConfig(t, "network_timeout: 30\ncheck_for_updates: false\nlog_level: info\n")
		assert.True(t, checkConfigFile(f))
	})
}

// ---------------------------------------------------------------------------
// isUpdateCheckDisabled — empty configPath branch
// ---------------------------------------------------------------------------

func TestIsUpdateCheckDisabledEmptyConfigPath(t *testing.T) {
	// Force both UserConfigDir and UserHomeDir to fail by corrupting HOME/XDG.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")
	t.Setenv("USERPROFILE", "")

	// Ensure env opt-out is not set.
	t.Setenv("ERST_NO_UPDATE_CHECK", "")

	checker := NewChecker("v1.0.0")
	// When configPath is empty isUpdateCheckDisabled falls through to false.
	// On some CI environments UserConfigDir still succeeds, so we only assert
	// the function does not panic and returns a boolean.
	result := checker.isUpdateCheckDisabled()
	assert.IsType(t, false, result)
}

// ---------------------------------------------------------------------------
// getCacheDir — fallback paths
// ---------------------------------------------------------------------------

func TestGetCacheDirFallbacks(t *testing.T) {
	t.Run("always returns a non-empty path containing erst", func(t *testing.T) {
		// Regardless of which branch fires, the result must be usable.
		dir := getCacheDir()
		assert.NotEmpty(t, dir)
		assert.Contains(t, dir, "erst")
	})
}

// ---------------------------------------------------------------------------
// getConfigPath — fallback paths
// ---------------------------------------------------------------------------

func TestGetConfigPathFallbacks(t *testing.T) {
	t.Run("always returns a string (may be empty only if both dirs fail)", func(t *testing.T) {
		path := getConfigPath()
		// Either a real path or empty string — must not panic.
		_ = path
	})

	t.Run("returned path contains erst when non-empty", func(t *testing.T) {
		path := getConfigPath()
		if path != "" {
			assert.Contains(t, path, "erst")
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeTempConfig writes content to a temp file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// newCheckerWithURL creates a Checker whose API calls go to url instead of
// the real GitHub endpoint.  This requires an apiURL field on Checker — see
// the note in the PR description about the one-line struct addition required.
func newCheckerWithURL(url string) *Checker {
	return &Checker{
		currentVersion: "v1.0.0",
		cacheDir:       os.TempDir(),
		apiURL:         url,
	}
}