// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/xdr"
	_ "modernc.org/sqlite"
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
	CacheDBName     = "cache.db"
	CacheDirName    = ".erst"
	FilePerm        = 0600
	DirPerm         = 0700
	DefaultCacheTTL = 24 * time.Hour
)

// CachedEntry represents a single cached value.
type CachedEntry struct {
	Key       string        `json:"key"`
	Value     string        `json:"value"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	TTL       time.Duration `json:"ttl"`
}

var (
	cacheDB   *sql.DB
	cacheOnce sync.Once
	cacheMu   sync.Mutex
)

// cacheSchema creates the rpc_cache table and indexes.
const cacheSchema = `
CREATE TABLE IF NOT EXISTS rpc_cache (
	key_hash   TEXT PRIMARY KEY,
	cache_key  TEXT NOT NULL,
	value      TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rpc_cache_expires ON rpc_cache(expires_at);
`

// GetCachePath returns the path to the cache directory, creating it if necessary.
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

// getCacheDBPath returns the full path to cache.db inside ~/.erst/
func getCacheDBPath() (string, error) {
	dir, err := GetCachePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, CacheDBName), nil
}

// ensureDB lazily opens the SQLite database and creates the schema.
func ensureDB() (*sql.DB, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cacheDB != nil {
		return cacheDB, nil
	}

	dbPath, err := getCacheDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec(cacheSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize cache schema: %w", err)
	}

	cacheDB = db
	return cacheDB, nil
}

// InitCacheWithDB injects an already-open *sql.DB (e.g. an in-memory database
// for testing). The caller is responsible for closing it.
func InitCacheWithDB(db *sql.DB) error {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if _, err := db.Exec(cacheSchema); err != nil {
		return fmt.Errorf("failed to initialize cache schema: %w", err)
	}
	cacheDB = db
	return nil
}

// CloseCache closes the underlying SQLite connection, if open.
func CloseCache() error {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cacheDB != nil {
		err := cacheDB.Close()
		cacheDB = nil
		return err
	}
	return nil
}

// getCacheKey returns the SHA-256 hex digest used as the primary key.
func getCacheKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// Get retrieves a value from the SQLite cache.
// Returns (value, found, error).
func Get(key string) (string, bool, error) {
	db, err := ensureDB()
	if err != nil {
		return "", false, err
	}

	keyHash := getCacheKey(key)
	now := time.Now().UnixNano()

	var value string
	err = db.QueryRow(
		"SELECT value FROM rpc_cache WHERE key_hash = ? AND expires_at > ?",
		keyHash, now,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cache read failed: %w", err)
	}

	return value, true, nil
}

// SetWithTTL stores a value in the cache with a specific TTL.
func SetWithTTL(key string, value string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	db, err := ensureDB()
	if err != nil {
		return err
	}

	keyHash := getCacheKey(key)
	now := time.Now()

	_, err = db.Exec(
		`INSERT INTO rpc_cache (key_hash, cache_key, value, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(key_hash) DO UPDATE SET
		   value = excluded.value,
		   created_at = excluded.created_at,
		   expires_at = excluded.expires_at`,
		keyHash, key, value, now.UnixNano(), now.Add(ttl).UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("cache write failed: %w", err)
	}
	return nil
}

// Set stores a value using the default TTL.
func Set(key string, value string) error {
	return SetWithTTL(key, value, DefaultCacheTTL)
}

// Invalidate removes a specific key from the cache.
func Invalidate(key string) error {
	db, err := ensureDB()
	if err != nil {
		return err
	}

	keyHash := getCacheKey(key)
	_, err = db.Exec("DELETE FROM rpc_cache WHERE key_hash = ?", keyHash)
	if err != nil {
		return fmt.Errorf("cache invalidate failed: %w", err)
	}
	return nil
}

// Cleanup removes expired cache entries older than maxAge.
// Returns the number of rows removed.
func Cleanup(maxAge time.Duration) (int, error) {
	db, err := ensureDB()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge).UnixNano()

	result, err := db.Exec("DELETE FROM rpc_cache WHERE expires_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("cache cleanup failed: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if removed > 0 {
		logger.Logger.Info("Cache cleanup completed", "entries_removed", removed)
	}

	return int(removed), nil
}
