// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest_MissingFile(t *testing.T) {
	tempDir := t.TempDir()

	manifest, err := LoadManifest(tempDir)
	require.Error(t, err)
	assert.Nil(t, manifest)
	assert.Contains(t, err.Error(), fmt.Sprintf("no erst.json found in %s", tempDir))
}

func TestLoadManifest_NoDependenciesKey(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "erst.json")
	content := []byte(`{"name": "test-pkg", "version": "1.0.0"}`)
	err := os.WriteFile(manifestPath, content, 0644)
	require.NoError(t, err)

	manifest, err := LoadManifest(tempDir)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "test-pkg", manifest.Name)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.NotNil(t, manifest.Dependencies)
	assert.Empty(t, manifest.Dependencies)
}

func TestManifest_SaveAndLoadRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	original := &Manifest{
		Name:    "demo-project",
		Version: "0.1.0",
		Dependencies: map[string]string{
			"@oz/token": "v1.0.0",
			"@std/math": "v2.1.0",
		},
	}

	err := SaveManifest(tempDir, original)
	require.NoError(t, err)

	loaded, err := LoadManifest(tempDir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, original, loaded)
}

func TestManifest_AddDependency(t *testing.T) {
	t.Run("valid packages", func(t *testing.T) {
		m := &Manifest{
			Dependencies: make(map[string]string),
		}

		err := m.AddDependency("@oz/token@v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", m.Dependencies["@oz/token"])

		err = m.AddDependency("@custom/contract@v2.5.1")
		require.NoError(t, err)
		assert.Equal(t, "v2.5.1", m.Dependencies["@custom/contract"])
	})

	t.Run("invalid packages", func(t *testing.T) {
		m := &Manifest{
			Dependencies: make(map[string]string),
		}

		invalidCases := []struct {
			name string
			pkg  string
		}{
			{name: "missing leading @", pkg: "oz/token@v1.0.0"},
			{name: "missing version separator", pkg: "@oz/token"},
			{name: "empty version", pkg: "@oz/token@"},
			{name: "invalid path structure", pkg: "@oz/token/extra@v1.0.0"},
			{name: "empty package string", pkg: ""},
		}

		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				err := m.AddDependency(tc.pkg)
				assert.Error(t, err)
			})
		}
	})
}
