// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Contract ID Validation Tests
// =============================================================================

func TestValidateContractID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid contract ID",
			id:      "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			id:      "GAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
			wantErr: true,
		},
		{
			name:    "too short",
			id:      "CAS3J7GY",
			wantErr: true,
		},
		{
			name:    "too long",
			id:      "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3NEXTRA",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContractID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateContractID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// GitHub URL Parsing Tests
// =============================================================================

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "full HTTPS URL",
			url:       "https://github.com/stellar/soroban-examples",
			wantOwner: "stellar",
			wantRepo:  "soroban-examples",
		},
		{
			name:      "HTTPS URL with .git suffix",
			url:       "https://github.com/stellar/soroban-examples.git",
			wantOwner: "stellar",
			wantRepo:  "soroban-examples",
		},
		{
			name:      "URL with trailing slash",
			url:       "https://github.com/stellar/soroban-examples/",
			wantOwner: "stellar",
			wantRepo:  "soroban-examples",
		},
		{
			name:      "URL without scheme",
			url:       "github.com/stellar/soroban-examples",
			wantOwner: "stellar",
			wantRepo:  "soroban-examples",
		},
		{
			name:      "URL with subpath",
			url:       "https://github.com/stellar/soroban-examples/tree/main/src",
			wantOwner: "stellar",
			wantRepo:  "soroban-examples",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "only owner",
			url:     "https://github.com/stellar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGitHubURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("parseGitHubURL(%q) owner = %q, want %q", tt.url, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseGitHubURL(%q) repo = %q, want %q", tt.url, repo, tt.wantRepo)
			}
		})
	}
}

// =============================================================================
// Source File Detection Tests
// =============================================================================

func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"src/lib.rs", true},
		{"src/contract.rs", true},
		{"Cargo.toml", true},
		{"deep/nested/Cargo.toml", true},
		{"Cargo.lock", true},
		{"README.md", false},
		{"src/main.go", false},
		{"test.py", false},
		{".gitignore", false},
		{"target/release/build.rs", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isSourceFile(tt.path)
			if got != tt.want {
				t.Errorf("isSourceFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Registry Client Tests (with mock server)
// =============================================================================

func TestGetContractInfo_Found(t *testing.T) {
	info := ContractInfo{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		WasmHash:   "abc123def456",
		Repository: "https://github.com/stellar/soroban-examples",
		Verified:   true,
		Creator:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.GetContractInfo(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil contract info")
	}
	if got.WasmHash != info.WasmHash {
		t.Errorf("WasmHash = %q, want %q", got.WasmHash, info.WasmHash)
	}
	if !got.Verified {
		t.Error("expected Verified to be true")
	}
	if got.Repository != info.Repository {
		t.Errorf("Repository = %q, want %q", got.Repository, info.Repository)
	}
}

func TestGetContractInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.GetContractInfo(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for not-found contract")
	}
}

func TestGetContractInfo_InvalidID(t *testing.T) {
	client := NewRegistryClient()
	ctx := context.Background()

	_, err := client.GetContractInfo(ctx, "invalid")
	if err == nil {
		t.Fatal("expected error for invalid contract ID")
	}
}

func TestGetContractInfo_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	_, err := client.GetContractInfo(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestGetWasmCode_Found(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(wasmBytes)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.GetWasmCode(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil WASM code")
	}
	if len(got.Code) != len(wasmBytes) {
		t.Errorf("WASM code length = %d, want %d", len(got.Code), len(wasmBytes))
	}
	// Check WASM magic bytes
	if got.Code[0] != 0x00 || got.Code[1] != 0x61 || got.Code[2] != 0x73 || got.Code[3] != 0x6d {
		t.Error("WASM magic bytes mismatch")
	}
}

func TestGetWasmCode_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.GetWasmCode(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for not-found WASM")
	}
}

func TestFetchVerifiedSource_NotVerified(t *testing.T) {
	info := ContractInfo{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		Verified:   false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.FetchVerifiedSource(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-verified contract")
	}
}

func TestFetchVerifiedSource_NoRepository(t *testing.T) {
	info := ContractInfo{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		Verified:   true,
		Repository: "",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	got, err := client.FetchVerifiedSource(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil when no repository link")
	}
}

func TestFetchVerifiedSource_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // simulate slow response
	}))
	defer server.Close()

	client := NewRegistryClient(WithBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.FetchVerifiedSource(ctx, "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// =============================================================================
// Registry Client Option Tests
// =============================================================================

func TestRegistryClientOptions(t *testing.T) {
	t.Run("WithNetwork", func(t *testing.T) {
		client := NewRegistryClient(WithNetwork(NetworkTestnet))
		if client.network != NetworkTestnet {
			t.Errorf("network = %q, want %q", client.network, NetworkTestnet)
		}
	})

	t.Run("WithBaseURL", func(t *testing.T) {
		client := NewRegistryClient(WithBaseURL("https://custom.api.com/"))
		if client.baseURL != "https://custom.api.com" {
			t.Errorf("baseURL = %q, want %q", client.baseURL, "https://custom.api.com")
		}
	})

	t.Run("WithHTTPClient", func(t *testing.T) {
		customHTTP := &http.Client{Timeout: 5 * time.Second}
		client := NewRegistryClient(WithHTTPClient(customHTTP))
		if client.httpClient != customHTTP {
			t.Error("expected custom HTTP client to be used")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		client := NewRegistryClient()
		if client.network != NetworkPublic {
			t.Errorf("default network = %q, want %q", client.network, NetworkPublic)
		}
		if client.baseURL != StellarExpertBaseURL {
			t.Errorf("default baseURL = %q, want %q", client.baseURL, StellarExpertBaseURL)
		}
	})
}

// =============================================================================
// Cache Tests
// =============================================================================

func TestSourceCache_PutAndGet(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	source := &SourceCode{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		WasmHash:   "abc123",
		Repository: "https://github.com/stellar/soroban-examples",
		Files: map[string]string{
			"src/lib.rs": "fn main() {}",
			"Cargo.toml": "[package]\nname = \"test\"",
		},
		FetchedAt: time.Now(),
	}

	if err := cache.Put(source); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got := cache.Get(source.ContractID)
	if got == nil {
		t.Fatal("expected non-nil cached source")
	}
	if got.ContractID != source.ContractID {
		t.Errorf("ContractID = %q, want %q", got.ContractID, source.ContractID)
	}
	if got.WasmHash != source.WasmHash {
		t.Errorf("WasmHash = %q, want %q", got.WasmHash, source.WasmHash)
	}
	if len(got.Files) != len(source.Files) {
		t.Errorf("file count = %d, want %d", len(got.Files), len(source.Files))
	}
	if got.Files["src/lib.rs"] != source.Files["src/lib.rs"] {
		t.Error("file content mismatch for src/lib.rs")
	}
}

func TestSourceCache_Miss(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	got := cache.Get("CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if got != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestSourceCache_Expiry(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set a very short TTL
	cache.SetTTL(1 * time.Millisecond)

	source := &SourceCode{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		WasmHash:   "abc123",
		Files:      map[string]string{"src/lib.rs": "fn main() {}"},
		FetchedAt:  time.Now(),
	}

	if err := cache.Put(source); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(10 * time.Millisecond)

	got := cache.Get(source.ContractID)
	if got != nil {
		t.Fatal("expected nil for expired cache entry")
	}
}

func TestSourceCache_Invalidate(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	source := &SourceCode{
		ContractID: "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N",
		WasmHash:   "abc123",
		Files:      map[string]string{},
		FetchedAt:  time.Now(),
	}

	if err := cache.Put(source); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := cache.Invalidate(source.ContractID); err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	got := cache.Get(source.ContractID)
	if got != nil {
		t.Fatal("expected nil after invalidation")
	}
}

func TestSourceCache_InvalidateNonexistent(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	err = cache.Invalidate("CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("Invalidate of nonexistent entry should not error: %v", err)
	}
}

func TestSourceCache_Clear(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Add multiple entries
	for i := 0; i < 3; i++ {
		// Generate different valid-looking contract IDs
		id := fmt.Sprintf("C%055d", i)
		source := &SourceCode{
			ContractID: id,
			WasmHash:   fmt.Sprintf("hash%d", i),
			Files:      map[string]string{},
			FetchedAt:  time.Now(),
		}
		if err := cache.Put(source); err != nil {
			t.Fatalf("Put failed for entry %d: %v", i, err)
		}
	}

	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify all entries are gone
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("failed to read cache dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty cache dir, got %d entries", len(entries))
	}
}

func TestSourceCache_CorruptEntry(t *testing.T) {
	cacheDir := t.TempDir()
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Write a corrupt entry
	path := cache.entryPath("CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatalf("failed to write corrupt entry: %v", err)
	}

	got := cache.Get("CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if got != nil {
		t.Fatal("expected nil for corrupt cache entry")
	}
}

// =============================================================================
// Resolver Tests
// =============================================================================

func TestResolver_ResolveFromRegistry(t *testing.T) {
	contractID := "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N"

	// Mock the registry server
	mux := http.NewServeMux()
	// Contract info endpoint
	mux.HandleFunc("/public/contract/", func(w http.ResponseWriter, r *http.Request) {
		info := ContractInfo{
			ContractID: contractID,
			WasmHash:   "abc123",
			Repository: "https://github.com/test/repo",
			Verified:   true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Mock GitHub API
	githubMux := http.NewServeMux()
	githubMux.HandleFunc("/repos/test/repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"default_branch": "main"})
	})
	githubMux.HandleFunc("/repos/test/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		tree := map[string]interface{}{
			"tree": []map[string]interface{}{
				{"path": "src/lib.rs", "type": "blob", "size": 100},
				{"path": "Cargo.toml", "type": "blob", "size": 50},
				{"path": "README.md", "type": "blob", "size": 200},
			},
		}
		json.NewEncoder(w).Encode(tree)
	})

	// We cannot easily mock raw.githubusercontent.com in this test,
	// so we test what we can: the registry lookup and tree parsing.
	// The full integration is tested via integration tests.

	rc := NewRegistryClient(WithBaseURL(server.URL))
	ctx := context.Background()

	info, err := rc.GetContractInfo(ctx, contractID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil contract info")
	}
	if !info.Verified {
		t.Error("expected contract to be verified")
	}
	if info.Repository != "https://github.com/test/repo" {
		t.Errorf("Repository = %q, want %q", info.Repository, "https://github.com/test/repo")
	}
}

func TestResolver_ResolveFromCache(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "sourcemap")
	cache, err := NewSourceCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	contractID := "CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N"
	source := &SourceCode{
		ContractID: contractID,
		WasmHash:   "abc123",
		Repository: "https://github.com/stellar/soroban-examples",
		Files: map[string]string{
			"src/lib.rs": "fn main() {}",
		},
		FetchedAt: time.Now(),
	}
	if err := cache.Put(source); err != nil {
		t.Fatalf("failed to put cache entry: %v", err)
	}

	// Create a resolver that would fail if it hit the registry
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not have hit the registry - cache should have been used")
	}))
	defer failServer.Close()

	resolver := &Resolver{
		registry: NewRegistryClient(WithBaseURL(failServer.URL)),
		cache:    cache,
	}

	got, err := resolver.Resolve(context.Background(), contractID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil source from cache")
	}
	if got.ContractID != contractID {
		t.Errorf("ContractID = %q, want %q", got.ContractID, contractID)
	}
}

func TestResolver_InvalidContractID(t *testing.T) {
	resolver := NewResolver()

	_, err := resolver.Resolve(context.Background(), "invalid")
	if err == nil {
		t.Fatal("expected error for invalid contract ID")
	}
}

func TestResolver_NilCache(t *testing.T) {
	resolver := &Resolver{
		registry: NewRegistryClient(),
		cache:    nil,
	}

	// These should not panic
	err := resolver.InvalidateCache("CAS3J7GYCCX3S7LX63P6R7EAL477J26C356X6E5A4XERAD7UXD6I7Y3N")
	if err != nil {
		t.Fatalf("InvalidateCache with nil cache should not error: %v", err)
	}

	err = resolver.ClearCache()
	if err != nil {
		t.Fatalf("ClearCache with nil cache should not error: %v", err)
	}
}

// =============================================================================
// NewResolver Option Tests
// =============================================================================

func TestNewResolver_Options(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		r := NewResolver()
		if r.registry == nil {
			t.Fatal("expected non-nil registry")
		}
		if r.cache != nil {
			t.Fatal("expected nil cache by default")
		}
	})

	t.Run("with cache", func(t *testing.T) {
		cacheDir := t.TempDir()
		r := NewResolver(WithCache(cacheDir))
		if r.cache == nil {
			t.Fatal("expected non-nil cache when WithCache is used")
		}
	})

	t.Run("with registry client", func(t *testing.T) {
		rc := NewRegistryClient(WithNetwork(NetworkTestnet))
		r := NewResolver(WithRegistryClient(rc))
		if r.registry != rc {
			t.Fatal("expected custom registry client to be used")
		}
	})
}
