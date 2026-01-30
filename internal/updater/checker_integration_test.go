// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigPrecedenceIntegration verifies the real-world precedence of
// configuration sources used by the update checker.
//
// Current implementation supports:
//   - Environment variable: ERST_NO_UPDATE_CHECK
//   - Config file:          check_for_updates: <bool> in config.yaml
//
// Precedence (highest first):
//  1. Environment variable
//  2. Config file
//
// This test exercises the full path resolution logic (getConfigPath) so we
// ensure the effective behavior matches the intended precedence.
func TestConfigPrecedenceIntegration(t *testing.T) {
	t.Helper()

	// Create an isolated config root and point the OS config directory there.
	tmpDir := t.TempDir()

	// Save original env vars so we can restore them.
	origHome := os.Getenv("HOME")
	origXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origAppData := os.Getenv("AppData")
	origErstNoUpdate := os.Getenv("ERST_NO_UPDATE_CHECK")

	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("XDG_CONFIG_HOME", origXDGConfigHome)
		_ = os.Setenv("AppData", origAppData)
		_ = os.Setenv("ERST_NO_UPDATE_CHECK", origErstNoUpdate)
	})

	require.NoError(t, os.Setenv("HOME", tmpDir))
	require.NoError(t, os.Setenv("XDG_CONFIG_HOME", tmpDir))
	require.NoError(t, os.Setenv("AppData", tmpDir))

	configPath := getConfigPath()
	require.NotEmpty(t, configPath)
	require.True(t, strings.HasPrefix(configPath, tmpDir), "config path should be under temp dir for this test")

	// Helper to write a minimal config file.
	writeConfig := func(content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))
	}

	t.Run("config disables updates when env is unset", func(t *testing.T) {
		require.NoError(t, os.Unsetenv("ERST_NO_UPDATE_CHECK"))

		writeConfig("check_for_updates: false\n")

		checker := NewChecker("v1.0.0")
		disabled := checker.isUpdateCheckDisabled()
		assert.True(t, disabled, "config file with check_for_updates: false should disable updates")
	})

	t.Run("config enables updates when env is unset", func(t *testing.T) {
		require.NoError(t, os.Unsetenv("ERST_NO_UPDATE_CHECK"))

		writeConfig("check_for_updates: true\n")

		checker := NewChecker("v1.0.0")
		disabled := checker.isUpdateCheckDisabled()
		assert.False(t, disabled, "config file with check_for_updates: true should keep updates enabled")
	})

	t.Run("environment variable takes precedence over config", func(t *testing.T) {
		// Config explicitly enables updates, but env var should still win.
		writeConfig("check_for_updates: true\n")

		require.NoError(t, os.Setenv("ERST_NO_UPDATE_CHECK", "1"))

		checker := NewChecker("v1.0.0")
		disabled := checker.isUpdateCheckDisabled()
		assert.True(t, disabled, "ERST_NO_UPDATE_CHECK should disable updates even if config enables them")
	})
}
