// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldErstProjectCreatesFilesAndDirs(t *testing.T) {
	root := t.TempDir()

	err := scaffoldErstProject(root, initScaffoldOptions{Network: "testnet"})
	require.NoError(t, err)

	for _, rel := range []string{
		"erst.toml",
		".gitignore",
		".erst/cache",
		".erst/snapshots",
		".erst/traces",
		"overrides",
		"wasm",
	} {
		_, statErr := os.Stat(filepath.Join(root, rel))
		assert.NoError(t, statErr, "expected %s to exist", rel)
	}

	erstToml, err := os.ReadFile(filepath.Join(root, "erst.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(erstToml), `network = "testnet"`)
	assert.Contains(t, string(erstToml), `cache_path = ".erst/cache"`)

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), "# Erst local debugging artifacts")
	assert.Contains(t, string(gitignore), ".erst/traces/")
}

func TestScaffoldErstProjectDoesNotOverwriteErstTomlWithoutForce(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "erst.toml")
	require.NoError(t, os.WriteFile(path, []byte("network = \"public\"\n"), 0644))

	err := scaffoldErstProject(root, initScaffoldOptions{Network: "testnet"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "erst.toml already exists")

	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "network = \"public\"\n", string(content))
}

func TestEnsureGitignoreBlockIsIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	initial := "node_modules/\n"
	require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

	block := renderProjectGitignoreBlock()
	require.NoError(t, ensureGitignoreBlock(path, block))
	require.NoError(t, ensureGitignoreBlock(path, block))

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	text := string(content)
	assert.True(t, strings.Contains(text, initial))
	assert.Equal(t, 1, strings.Count(text, "# Erst local debugging artifacts"))
}

func TestRenderProjectErstTomlStandaloneNetwork(t *testing.T) {
	content := renderProjectErstToml("standalone")
	assert.Contains(t, content, `rpc_url = "http://localhost:8000"`)
	assert.Contains(t, content, `network = "standalone"`)
}
