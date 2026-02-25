// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// HashLedgerKey generates a deterministic SHA-256 hash of a Stellar LedgerKey.
// This is used by verification scripts and potentially conflicting with internal hashing if different.
func HashLedgerKey(key xdr.LedgerKey) (string, error) {
	xdrBytes, err := key.MarshalBinary()
	if err != nil {
		return "", errors.WrapMarshalFailed(err)
	}
	hash := sha256.Sum256(xdrBytes)
	return hex.EncodeToString(hash[:]), nil
}

const (
	CacheDirName    = ".erst/cache"
	FilePerm        = 0600
	DirPerm         = 0700
	DefaultCacheTTL = 24 * time.Hour
)

type CachedEntry struct {
	Key       string        `json:"key"`
	Value     string        `json:"value"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	TTL       time.Duration `json:"ttl"`
}

// GetCachePath returns the path to the cache directory, creating it if necessary
func GetCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.WrapValidationError(fmt.Sprintf("failed to get user home directory: %v", err))
	}

	path := filepath.Join(home, CacheDirName)
	if err := os.MkdirAll(path, DirPerm); err != nil {
		return "", errors.WrapValidationError(fmt.Sprintf("failed to create cache directory: %v", err))
	}

	return path, nil
}

// getCacheKey returns the unique filename for a given ledger key
func getCacheKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func Get(key string) (string, bool, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return "", false, err
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", false, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return "", false, errors.WrapValidationError(fmt.Sprintf("failed to read cache file: %v", err))
	}

	var entry CachedEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		logger.Logger.Warn("Cache file corrupted", "key", key, "error", err)
		return "", false, nil
	}

	if time.Now().After(entry.ExpiresAt) {
		logger.Logger.Debug("Cache entry expired", "key", key, "expired_at", entry.ExpiresAt)
		_ = os.Remove(filename)
		return "", false, nil
	}

	return entry.Value, true, nil
}

func SetWithTTL(key string, value string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	now := time.Now()
	entry := CachedEntry{
		Key:       key,
		Value:     value,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		TTL:       ttl,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")
	if err := os.WriteFile(filename, data, FilePerm); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write cache file: %v", err))
	}

	return nil
}

func Set(key string, value string) error {
	return SetWithTTL(key, value, DefaultCacheTTL)
}

// Invalidate removes a specific key from the cache
func Invalidate(key string) error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	filename := filepath.Join(cachePath, getCacheKey(key)+".json")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Cleanup removes cache files older than maxAge
func Cleanup(maxAge time.Duration) (int, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	cutoff := time.Now().Add(-maxAge)

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		// Check file modification time first as a quick check
		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(cachePath, e.Name())

			// Double check content creation time if needed, but modtime is sufficient for simple TTL
			if err := os.Remove(filePath); err == nil {
				removedCount++
			}
		}
	}

	return removedCount, nil
}
