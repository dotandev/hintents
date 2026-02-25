// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

const (
	// DefaultCacheTTL is how long cached source entries remain valid.
	DefaultCacheTTL = 24 * time.Hour
)

// CacheEntry represents a cached source code entry.
type CacheEntry struct {
	Source   *SourceCode `json:"source"`
	CachedAt time.Time   `json:"cached_at"`
	TTL      string      `json:"ttl"`
}

// SourceCache provides local disk caching for downloaded source code.
// It prevents redundant network requests for previously fetched sources.
type SourceCache struct {
	cacheDir string
	ttl      time.Duration
	mu       sync.RWMutex
}

// NewSourceCache creates a new source cache at the given directory.
func NewSourceCache(cacheDir string) (*SourceCache, error) {
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %q: %w", cacheDir, err)
	}
	return &SourceCache{
		cacheDir: cacheDir,
		ttl:      DefaultCacheTTL,
	}, nil
}

// SetTTL overrides the default cache TTL.
func (sc *SourceCache) SetTTL(ttl time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.ttl = ttl
}

// Get retrieves a cached source code entry for a contract.
// Returns nil if not cached or if the cache entry has expired.
func (sc *SourceCache) Get(contractID string) *SourceCode {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	path := sc.entryPath(contractID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		logger.Logger.Warn("Corrupt cache entry, ignoring", "contract_id", contractID, "error", err)
		return nil
	}

	if time.Since(entry.CachedAt) > sc.ttl {
		logger.Logger.Debug("Cache entry expired", "contract_id", contractID)
		return nil
	}

	logger.Logger.Debug("Cache hit", "contract_id", contractID)
	return entry.Source
}

// Put stores a source code entry in the cache.
func (sc *SourceCache) Put(source *SourceCode) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	entry := CacheEntry{
		Source:   source,
		CachedAt: time.Now(),
		TTL:      sc.ttl.String(),
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	path := sc.entryPath(source.ContractID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache entry: %w", err)
	}

	logger.Logger.Debug("Source cached", "contract_id", source.ContractID, "path", path)
	return nil
}

// Invalidate removes a cached entry for a contract.
func (sc *SourceCache) Invalidate(contractID string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	path := sc.entryPath(contractID)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Clear removes all cached entries.
func (sc *SourceCache) Clear() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	entries, err := os.ReadDir(sc.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(sc.cacheDir, entry.Name())
		if err := os.Remove(path); err != nil {
			logger.Logger.Warn("Failed to remove cache entry", "path", path, "error", err)
		}
	}

	return nil
}

// entryPath returns the filesystem path for a contract's cache entry.
func (sc *SourceCache) entryPath(contractID string) string {
	hash := sha256.Sum256([]byte(contractID))
	filename := fmt.Sprintf("%x.json", hash[:8])
	return filepath.Join(sc.cacheDir, filename)
}
