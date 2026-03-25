// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
)

// setTestHomeDir sets both HOME (Unix) and USERPROFILE (Windows) to the given directory
// and returns a cleanup function to restore the original values
func setTestHomeDir(t *testing.T, tmpDir string) func() {
	t.Helper()
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)
	return func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
	}
}

func TestAddAndGetCustomNetwork(t *testing.T) {
	// Use a temporary directory for testing
	tmpDir := t.TempDir()
	cleanup := setTestHomeDir(t, tmpDir)
	defer cleanup()

	testConfig := rpc.NetworkConfig{
		Name:              "local-dev",
		HorizonURL:        "http://localhost:8000",
		NetworkPassphrase: "Local Development Network",
		SorobanRPCURL:     "http://localhost:8001",
	}

	// Add custom network
	err := AddCustomNetwork("local-dev", testConfig)
	if err != nil {
		t.Fatalf("Failed to add custom network: %v", err)
	}

	// Retrieve custom network
	retrieved, err := GetCustomNetwork("local-dev")
	if err != nil {
		t.Fatalf("Failed to get custom network: %v", err)
	}

	if retrieved.HorizonURL != testConfig.HorizonURL {
		t.Errorf("Expected HorizonURL %s, got %s", testConfig.HorizonURL, retrieved.HorizonURL)
	}
	if retrieved.NetworkPassphrase != testConfig.NetworkPassphrase {
		t.Errorf("Expected NetworkPassphrase %s, got %s", testConfig.NetworkPassphrase, retrieved.NetworkPassphrase)
	}
}

func TestListCustomNetworks(t *testing.T) {
	tmpDir := t.TempDir()
	cleanup := setTestHomeDir(t, tmpDir)
	defer cleanup()

	// Add multiple networks
	networks := []string{"local-dev", "staging", "private-net"}
	for _, name := range networks {
		config := rpc.NetworkConfig{
			Name:              name,
			HorizonURL:        "http://localhost:8000",
			NetworkPassphrase: "Test Network",
		}
		if err := AddCustomNetwork(name, config); err != nil {
			t.Fatalf("Failed to add network %s: %v", name, err)
		}
	}

	// List networks
	list, err := ListCustomNetworks()
	if err != nil {
		t.Fatalf("Failed to list networks: %v", err)
	}

	if len(list) != len(networks) {
		t.Errorf("Expected %d networks, got %d", len(networks), len(list))
	}
}

func TestRemoveCustomNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	cleanup := setTestHomeDir(t, tmpDir)
	defer cleanup()

	testConfig := rpc.NetworkConfig{
		Name:              "temp-network",
		HorizonURL:        "http://localhost:8000",
		NetworkPassphrase: "Temp Network",
	}

	// Add network
	if err := AddCustomNetwork("temp-network", testConfig); err != nil {
		t.Fatalf("Failed to add network: %v", err)
	}

	// Remove network
	if err := RemoveCustomNetwork("temp-network"); err != nil {
		t.Fatalf("Failed to remove network: %v", err)
	}

	// Verify it's gone
	_, err := GetCustomNetwork("temp-network")
	if err == nil {
		t.Error("Expected error when getting removed network, got nil")
	}
}

func TestConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	cleanup := setTestHomeDir(t, tmpDir)
	defer cleanup()

	testConfig := rpc.NetworkConfig{
		Name:              "secure-net",
		HorizonURL:        "http://localhost:8000",
		NetworkPassphrase: "Secure Network",
	}

	if err := AddCustomNetwork("secure-net", testConfig); err != nil {
		t.Fatalf("Failed to add network: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".erst", "networks.json")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	// Check that file has restrictive permissions (0600)
	// Note: On Windows, file permissions work differently and this check may not be meaningful
	// Windows uses ACLs instead of Unix-style permissions
	mode := info.Mode().Perm()
	expected := os.FileMode(0600)
	if os.PathSeparator != '\\' && mode != expected {
		// Only check permissions on Unix systems
		t.Errorf("Expected file permissions %o, got %o", expected, mode)
	}
}
