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
	"sort"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// Config holds cache configuration
type Config struct {
	// MaxSizeBytes is the maximum cache size in bytes (default 1GB)
	MaxSizeBytes int64
}

// DefaultConfig returns the default cache configuration
func DefaultConfig() Config {
	return Config{
		MaxSizeBytes: 1024 * 1024 * 1024, // 1GB
	}
}

// Manager handles cache operations including cleanup
type Manager struct {
	cacheDir string
	config   Config
}

// NewManager creates a new cache manager
func NewManager(cacheDir string, config Config) *Manager {
	return &Manager{
		cacheDir: cacheDir,
		config:   config,
	}
}

// FileInfo contains information about a cached file
type FileInfo struct {
	Path       string
	Size       int64
	LastAccess time.Time
	ModTime    time.Time
}

// GetCacheDir returns the cache directory path (creates if not exists)
func (m *Manager) GetCacheDir() (string, error) {
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return m.cacheDir, nil
}

// GetCacheSize returns the current size of the cache in bytes
func (m *Manager) GetCacheSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(m.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("failed to calculate cache size: %w", err)
	}

	return totalSize, nil
}

// ListCachedFiles returns a list of all cached files
func (m *Manager) ListCachedFiles() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(m.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, FileInfo{
				Path:       path,
				Size:       info.Size(),
				LastAccess: info.ModTime(),
				ModTime:    info.ModTime(),
			})
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to list cache files: %w", err)
	}

	return files, nil
}

// SortFilesByAccessTime sorts files by access time (oldest first)
func SortFilesByAccessTime(files []FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].LastAccess.Before(files[j].LastAccess)
	})
}

// CleanupStatus contains information about a cleanup operation
type CleanupStatus struct {
	FilesDeleted int
	SpaceFreed   int64
	OriginalSize int64
	FinalSize    int64
	DeletedFiles []string
}

// CleanLRU performs LRU (Least Recently Used) cleanup to ensure cache size is within limit
// Returns the cleanup status and any errors
func (m *Manager) CleanLRU() (*CleanupStatus, error) {
	// Check if cache directory exists
	if _, err := os.Stat(m.cacheDir); os.IsNotExist(err) {
		return &CleanupStatus{
			OriginalSize: 0,
			FinalSize:    0,
		}, nil
	}

	// Get current cache size
	originalSize, err := m.GetCacheSize()
	if err != nil {
		return nil, err
	}

	status := &CleanupStatus{
		OriginalSize: originalSize,
		DeletedFiles: []string{},
	}

	// Check if cleanup is needed
	if originalSize <= m.config.MaxSizeBytes {
		status.FinalSize = originalSize
		logger.Logger.Info("Cache size within limit", "current", originalSize, "limit", m.config.MaxSizeBytes)
		return status, nil
	}

	// Get list of cached files
	files, err := m.ListCachedFiles()
	if err != nil {
		return nil, err
	}

	// Sort by access time (oldest first)
	SortFilesByAccessTime(files)

	// Delete files until cache size is under limit
	targetSize := m.config.MaxSizeBytes / 2 // Target 50% of max size after cleanup
	currentSize := originalSize

	for _, file := range files {
		if currentSize <= targetSize {
			break
		}

		err := os.Remove(file.Path)
		if err != nil {
			logger.Logger.Warn("Failed to delete cache file", "path", file.Path, "error", err)
			continue
		}

		status.FilesDeleted++
		status.SpaceFreed += file.Size
		status.DeletedFiles = append(status.DeletedFiles, file.Path)
		currentSize -= file.Size

		logger.Logger.Debug("Deleted cache file", "path", file.Path, "size", file.Size)
	}

	status.FinalSize = currentSize

	logger.Logger.Info("Cache cleanup completed",
		"files_deleted", status.FilesDeleted,
		"space_freed", status.SpaceFreed,
		"original_size", status.OriginalSize,
		"final_size", status.FinalSize)

	return status, nil
}

// Clean performs a complete cache cleanup with user confirmation
// It will prompt the user before deleting files
func (m *Manager) Clean(force bool) (*CleanupStatus, error) {
	// Check if cache directory exists
	if _, err := os.Stat(m.cacheDir); os.IsNotExist(err) {
		fmt.Println("Cache directory does not exist")
		return &CleanupStatus{}, nil
	}

	// Get current cache size
	originalSize, err := m.GetCacheSize()
	if err != nil {
		return nil, err
	}

	status := &CleanupStatus{
		OriginalSize: originalSize,
		DeletedFiles: []string{},
	}

	// Format sizes for display
	originalSizeStr := formatBytes(originalSize)

	if originalSize == 0 {
		fmt.Printf("Cache is empty (0 B)\n")
		status.FinalSize = 0
		return status, nil
	}

	// Show warning and get confirmation
	fmt.Printf("Cache size: %s\n", originalSizeStr)
	fmt.Printf("Maximum size: %s\n", formatBytes(m.config.MaxSizeBytes))

	if !force {
		fmt.Print("\nThis will delete the oldest cached files. Continue? (yes/no): ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return status, fmt.Errorf("failed to read input: %w", err)
		}
		if response != "yes" && response != "y" {
			fmt.Println("Cache cleanup cancelled")
			status.FinalSize = originalSize
			return status, nil
		}
	}

	fmt.Println("\nCleaning cache (Least Recently Used files first)...")

	// Get list of cached files
	files, err := m.ListCachedFiles()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		fmt.Println("No cached files found")
		status.FinalSize = 0
		return status, nil
	}

	// Sort by access time (oldest first)
	SortFilesByAccessTime(files)

	// Delete oldest files
	targetSize := m.config.MaxSizeBytes / 2 // Target 50% of max size
	currentSize := originalSize

	for _, file := range files {
		if currentSize <= targetSize {
			break
		}

		err := os.Remove(file.Path)
		if err != nil {
			logger.Logger.Warn("Failed to delete cache file", "path", file.Path, "error", err)
			continue
		}

		status.FilesDeleted++
		status.SpaceFreed += file.Size
		status.DeletedFiles = append(status.DeletedFiles, filepath.Base(file.Path))
		currentSize -= file.Size

		logger.Logger.Debug("Deleted cache file", "path", file.Path, "size", file.Size)
	}

	status.FinalSize = currentSize

	// Print summary
	fmt.Printf("\nCleanup complete!\n")
	fmt.Printf("Files deleted: %d\n", status.FilesDeleted)
	fmt.Printf("Space freed: %s\n", formatBytes(status.SpaceFreed))
	fmt.Printf("Final cache size: %s\n", formatBytes(status.FinalSize))

	return status, nil
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%.0f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}
