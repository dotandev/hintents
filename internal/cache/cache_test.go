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

package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	cacheDir := t.TempDir()
	config := Config{MaxSizeBytes: 1024}

	manager := NewManager(cacheDir, config)

	assert.Equal(t, cacheDir, manager.cacheDir)
	assert.Equal(t, int64(1024), manager.config.MaxSizeBytes)
}

func TestGetCacheDir(t *testing.T) {
	cacheDir := t.TempDir()
	manager := NewManager(cacheDir, DefaultConfig())

	dir, err := manager.GetCacheDir()
	require.NoError(t, err)
	assert.Equal(t, cacheDir, dir)

	// Verify directory exists
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGetCacheSize(t *testing.T) {
	cacheDir := t.TempDir()
	manager := NewManager(cacheDir, DefaultConfig())

	// Initial cache should be empty
	size, err := manager.GetCacheSize()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Create a test file
	testFile := filepath.Join(cacheDir, "test.txt")
	testData := []byte("test data")
	err = os.WriteFile(testFile, testData, 0644)
	require.NoError(t, err)

	// Check size again
	size, err = manager.GetCacheSize()
	require.NoError(t, err)
	assert.Equal(t, int64(len(testData)), size)
}

func TestListCachedFiles(t *testing.T) {
	cacheDir := t.TempDir()
	manager := NewManager(cacheDir, DefaultConfig())

	// Create test files
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, file := range testFiles {
		path := filepath.Join(cacheDir, file)
		err := os.WriteFile(path, []byte("data"), 0644)
		require.NoError(t, err)
	}

	// List files
	files, err := manager.ListCachedFiles()
	require.NoError(t, err)
	assert.Equal(t, len(testFiles), len(files))

	// Verify all files are listed
	fileMap := make(map[string]bool)
	for _, f := range files {
		fileMap[filepath.Base(f.Path)] = true
	}
	for _, name := range testFiles {
		assert.True(t, fileMap[name], "File %s not found in list", name)
	}
}

func TestSortFilesByAccessTime(t *testing.T) {
	now := time.Now()

	files := []FileInfo{
		{Path: "file1", LastAccess: now.Add(2 * time.Hour)},
		{Path: "file3", LastAccess: now},
		{Path: "file2", LastAccess: now.Add(1 * time.Hour)},
	}

	SortFilesByAccessTime(files)

	// Verify sorting (oldest first)
	assert.Equal(t, "file3", files[0].Path)
	assert.Equal(t, "file2", files[1].Path)
	assert.Equal(t, "file1", files[2].Path)
}

func TestCleanLRU(t *testing.T) {
	cacheDir := t.TempDir()
	maxSize := int64(100) // Small size to trigger cleanup easily
	config := Config{MaxSizeBytes: maxSize}
	manager := NewManager(cacheDir, config)

	// Create test files that exceed the limit.
	// Create 3 files of 50 bytes each = 150 bytes total.
	for i := 1; i <= 3; i++ {
		path := filepath.Join(cacheDir, fmt.Sprintf("file%d", i))
		data := make([]byte, 50)
		err := os.WriteFile(path, data, 0644)
		require.NoError(t, err)

		// Add delay so files have different modification times
		time.Sleep(10 * time.Millisecond)
	}

	// Verify initial size exceeds limit
	size, err := manager.GetCacheSize()
	require.NoError(t, err)
	assert.Greater(t, size, maxSize)

	// Run cleanup
	status, err := manager.CleanLRU()
	require.NoError(t, err)

	assert.Greater(t, status.FilesDeleted, 0)
	assert.Greater(t, status.SpaceFreed, int64(0))
	assert.Less(t, status.FinalSize, size)
}

func TestCleanLRUEmptyCache(t *testing.T) {
	cacheDir := t.TempDir()
	manager := NewManager(cacheDir, DefaultConfig())

	status, err := manager.CleanLRU()
	require.NoError(t, err)

	assert.Equal(t, 0, status.FilesDeleted)
	assert.Equal(t, int64(0), status.SpaceFreed)
	assert.Equal(t, int64(0), status.OriginalSize)
}

func TestCleanLRUWithinLimit(t *testing.T) {
	cacheDir := t.TempDir()
	config := Config{MaxSizeBytes: 1024 * 1024} // 1MB
	manager := NewManager(cacheDir, config)

	// Create a small file (1KB)
	path := filepath.Join(cacheDir, "small.txt")
	data := make([]byte, 1024)
	err := os.WriteFile(path, data, 0644)
	require.NoError(t, err)

	// Run cleanup - should not delete anything
	status, err := manager.CleanLRU()
	require.NoError(t, err)

	assert.Equal(t, 0, status.FilesDeleted)
	assert.Equal(t, int64(0), status.SpaceFreed)
	assert.Equal(t, int64(1024), status.OriginalSize)
	assert.Equal(t, int64(1024), status.FinalSize)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1536 * 1024 * 1024, "1.50 GB"},
	}

	for _, test := range tests {
		result := formatBytes(test.bytes)
		assert.Equal(t, test.expected, result)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.Equal(t, int64(1024*1024*1024), config.MaxSizeBytes)
}
