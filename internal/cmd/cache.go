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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dotandev/hintents/internal/cache"
	"github.com/spf13/cobra"
)

var (
	cacheForceFlag bool
)

// getCacheDir returns the default cache directory
func getCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	return filepath.Join(homeDir, ".erst", "cache")
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage transaction and simulation cache",
	Long: `Manage the local cache that stores transaction data and simulation results.
Caching improves performance and enables offline analysis.

Cache location: ~/.erst/cache (configurable via ERST_CACHE_DIR)

Available subcommands:
  status  - View cache size and usage statistics
  clean   - Remove old files using LRU strategy
  clear   - Delete all cached data`,
	Example: `  # Check cache status
  erst cache status

  # Clean old cache entries
  erst cache clean

  # Force clean without confirmation
  erst cache clean --force

  # Clear all cache
  erst cache clear --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display cache statistics",
	Long:  `Display the current cache size, number of cached files, and disk usage statistics.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cacheDir := getCacheDir()
		manager := cache.NewManager(cacheDir, cache.DefaultConfig())

		size, err := manager.GetCacheSize()
		if err != nil {
			return fmt.Errorf("Error: failed to calculate cache size: %w", err)
		}

		files, err := manager.ListCachedFiles()
		if err != nil {
			return fmt.Errorf("Error: failed to list cache files: %w", err)
		}

		fmt.Printf("Cache directory: %s\n", cacheDir)
		fmt.Printf("Cache size: %s\n", formatBytes(size))
		fmt.Printf("Files cached: %d\n", len(files))
		fmt.Printf("Maximum size: %s\n", formatBytes(cache.DefaultConfig().MaxSizeBytes))

		if size > cache.DefaultConfig().MaxSizeBytes {
			fmt.Printf("\n⚠️  Cache size exceeds maximum limit. Run 'erst cache clean' to free space.\n")
		}

		return nil
	},
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove old cached files using LRU strategy",
	Long: `Remove old cached files using LRU (Least Recently Used) strategy.

This command will:
  1. Identify the oldest cached files
  2. Prompt for confirmation before deletion
  3. Delete files until cache size is reduced to 50% of maximum

Use --force to skip the confirmation prompt.`,
	Example: `  # Clean cache with confirmation
  erst cache clean

  # Force clean without prompt
  erst cache clean --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cacheDir := getCacheDir()
		manager := cache.NewManager(cacheDir, cache.DefaultConfig())

		status, err := manager.Clean(cacheForceFlag)
		if err != nil {
			return fmt.Errorf("Error: cache cleanup failed: %w", err)
		}

		if status.FilesDeleted == 0 && status.OriginalSize > 0 {
			fmt.Println("No files needed to be deleted")
		}

		return nil
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all cached files",
	Long: `Remove all cached files from the cache directory.

⚠️  Warning: This action cannot be undone. Use --force to skip confirmation.`,
	Example: `  # Clear cache with confirmation
  erst cache clear

  # Force clear without prompt
  erst cache clear --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cacheDir := getCacheDir()

		// Check if cache exists
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			fmt.Println("Cache directory does not exist")
			return nil
		}

		// Get confirmation unless force flag is set
		if !cacheForceFlag {
			fmt.Printf("This will delete ALL cached files in %s\n", cacheDir)
			fmt.Print("Are you sure? (yes/no): ")
			var response string
			if _, err := fmt.Scanln(&response); err != nil {
				return fmt.Errorf("Error: failed to read confirmation input: %w", err)
			}
			if response != "yes" && response != "y" {
				fmt.Println("Cache clear cancelled")
				return nil
			}
		}

		err := os.RemoveAll(cacheDir)
		if err != nil {
			return fmt.Errorf("Error: failed to clear cache directory: %w", err)
		}

		fmt.Println("Cache cleared successfully")
		return nil
	},
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

func init() {
	// Add subcommands to cache command
	cacheCmd.AddCommand(cacheStatusCmd)
	cacheCmd.AddCommand(cacheCleanCmd)
	cacheCmd.AddCommand(cacheClearCmd)

	// Add flags
	cacheCleanCmd.Flags().BoolVarP(&cacheForceFlag, "force", "f", false, "Skip confirmation prompt")
	cacheClearCmd.Flags().BoolVarP(&cacheForceFlag, "force", "f", false, "Skip confirmation prompt")

	// Add cache command to root
	rootCmd.AddCommand(cacheCmd)
}
