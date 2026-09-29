// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestIsWindowsAbsolutePath(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"C:\\Users\\User\\contract.wasm", true},
		{"C:/Users/User/contract.wasm", true},
		{"c:\\contracts\\token.wasm", true},
		{"d:/builds/contract.wasm", true},
		{"Z:\\test.wasm", true},
		{"C:relative.wasm", true},
		{"\\\\server\\share\\contract.wasm", true},
		{"//server/share/contract.wasm", true},
		{"/usr/local/contract.wasm", false},
		{"./contracts/token.wasm", false},
		{"../parent/token.wasm", false},
		{"contract.wasm", false},
		{"https://example.com/contract.wasm", false},
		{"http://localhost:8080/token.wasm", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := IsWindowsAbsolutePath(tt.input)
			assert.Equal(t, tt.expected, actual, "path: %s", tt.input)
		})
	}
}

func TestIsRemoteURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com/contract.wasm", true},
		{"https://soroban.stellar.org/contract.wasm", true},
		{"HTTP://EXAMPLE.COM/CONTRACT.WASM", true},
		{"HTTPS://TEST.COM/CONTRACT.WASM", true},
		{"C:\\contracts\\my_contract.wasm", false},
		{"C:/contracts/my_contract.wasm", false},
		{"c:\\Users\\test.wasm", false},
		{"d:/test.wasm", false},
		{"\\\\network\\share\\contract.wasm", false},
		{"/home/user/contract.wasm", false},
		{"./contract.wasm", false},
		{"file:///C:/path/to/contract.wasm", false},
		{"file:///home/user/contract.wasm", false},
		{"ftp://example.com/contract.wasm", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := IsRemoteURL(tt.input)
			assert.Equal(t, tt.expected, actual, "target: %s", tt.input)
		})
	}
}

func TestNormalizeContractPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"C:\\Users\\User\\contract.wasm", "C:/Users/User/contract.wasm"},
		{"D:\\build\\output\\token.wasm", "D:/build/output/token.wasm"},
		{"folder\\subfolder\\contract.wasm", "folder/subfolder/contract.wasm"},
		{"/already/slashed/path.wasm", "/already/slashed/path.wasm"},
		{"C:/mixed/slashes\\contract.wasm", "C:/mixed/slashes/contract.wasm"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := NormalizeContractPath(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestResolveContractPath_WindowsPaths(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantResolved string
		wantRemote   bool
		wantWindows  bool
	}{
		{
			name:         "Windows drive with backslashes",
			input:        `C:\Users\Alice\contracts\token.wasm`,
			wantResolved: "C:/Users/Alice/contracts/token.wasm",
			wantRemote:   false,
			wantWindows:  true,
		},
		{
			name:         "Windows drive with forward slashes",
			input:        "C:/Users/Alice/contracts/token.wasm",
			wantResolved: "C:/Users/Alice/contracts/token.wasm",
			wantRemote:   false,
			wantWindows:  true,
		},
		{
			name:         "Lowercase Windows drive letter",
			input:        `d:\stellar\contracts\pool.wasm`,
			wantResolved: "d:/stellar/contracts/pool.wasm",
			wantRemote:   false,
			wantWindows:  true,
		},
		{
			name:         "UNC network share path",
			input:        `\\storage\shares\contracts\vault.wasm`,
			wantResolved: "//storage/shares/contracts/vault.wasm",
			wantRemote:   false,
			wantWindows:  true,
		},
		{
			name:         "file URI pointing to Windows drive",
			input:        "file:///C:/contracts/vault.wasm",
			wantResolved: "C:/contracts/vault.wasm",
			wantRemote:   false,
			wantWindows:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := ResolveContractPath(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantResolved, loc.ResolvedPath)
			assert.Equal(t, tt.wantRemote, loc.IsRemote)
			assert.Equal(t, tt.wantWindows, loc.IsWindows)
		})
	}
}

func TestResolveContractPath_RemoteAndUnix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantResolved string
		wantRemote   bool
		wantWindows  bool
	}{
		{
			name:         "HTTP remote URL",
			input:        "http://example.com/soroban/contract.wasm",
			wantResolved: "http://example.com/soroban/contract.wasm",
			wantRemote:   true,
			wantWindows:  false,
		},
		{
			name:         "HTTPS remote URL",
			input:        "https://raw.githubusercontent.com/stellar/contracts/main/soroban.wasm",
			wantResolved: "https://raw.githubusercontent.com/stellar/contracts/main/soroban.wasm",
			wantRemote:   true,
			wantWindows:  false,
		},
		{
			name:         "Unix absolute path",
			input:        "/home/developer/contracts/token.wasm",
			wantResolved: "/home/developer/contracts/token.wasm",
			wantRemote:   false,
			wantWindows:  false,
		},
		{
			name:         "Relative path",
			input:        "./build/contracts/token.wasm",
			wantResolved: "./build/contracts/token.wasm",
			wantRemote:   false,
			wantWindows:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := ResolveContractPath(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantResolved, loc.ResolvedPath)
			assert.Equal(t, tt.wantRemote, loc.IsRemote)
			assert.Equal(t, tt.wantWindows, loc.IsWindows)
		})
	}
}

func TestResolveContractPath_Empty(t *testing.T) {
	_, err := ResolveContractPath("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	_, err = ResolveContractPath("   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestLoadContract_LocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test_contract.wasm")
	expectedContent := []byte("\x00asm\x01\x00\x00\x00mock-wasm-bytecode")
	err := os.WriteFile(wasmPath, expectedContent, 0644)
	require.NoError(t, err)

	// Load via normal path
	loaded, err := LoadContract(wasmPath)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, loaded)

	// Load via path normalized with ToSlash
	slashPath := filepath.ToSlash(wasmPath)
	loaded2, err := LoadContract(slashPath)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, loaded2)
}

func TestLoadContract_RemoteURL(t *testing.T) {
	expectedBytes := []byte("\x00asm\x01\x00\x00\x00remote-contract-code")
	origTransport := contractHTTPClient.Transport
	defer func() { contractHTTPClient.Transport = origTransport }()

	contractHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "GET", req.Method)
		assert.Equal(t, "erst-contract-loader", req.Header.Get("User-Agent"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(expectedBytes)),
			Header:     make(http.Header),
		}, nil
	})

	loaded, err := LoadContractWithContext(context.Background(), "https://example.com/contract.wasm")
	require.NoError(t, err)
	assert.Equal(t, expectedBytes, loaded)
}

func TestLoadContract_RemoteErrors(t *testing.T) {
	origTransport := contractHTTPClient.Transport
	defer func() { contractHTTPClient.Transport = origTransport }()

	// 404 error
	contractHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
			Header:     make(http.Header),
		}, nil
	})

	_, err := LoadContract("https://example.com/missing.wasm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")

	// Empty remote content
	contractHTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
			Header:     make(http.Header),
		}, nil
	})

	_, err = LoadContract("https://example.com/empty.wasm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoadContract_NonExistentFile(t *testing.T) {
	_, err := LoadContract("/non/existent/path/to/contract.wasm")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contract file not found or unreadable")
}

func TestRunCommand_Registration(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "run" {
			found = true
			assert.Equal(t, "testing", cmd.GroupID)
			assert.NotEmpty(t, cmd.Short)
			break
		}
	}
	assert.True(t, found, "run command should be registered on rootCmd")
}
