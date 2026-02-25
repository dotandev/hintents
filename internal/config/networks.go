// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
)

// CustomNetworkConfig represents a saved custom network configuration
type CustomNetworkConfig struct {
	Networks map[string]rpc.NetworkConfig `json:"networks"`
}

// GetConfigPath returns the path to the erst configuration directory
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.WrapConfigError("failed to get home directory", err)
	}
	return filepath.Join(home, ".erst"), nil
}

// GetNetworkConfigPath returns the path to the network configuration file
func GetNetworkConfigPath() (string, error) {
	configDir, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "networks.json"), nil
}

// LoadCustomNetworks loads custom network configurations from disk
func LoadCustomNetworks() (*CustomNetworkConfig, error) {
	configPath, err := GetNetworkConfigPath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return empty config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &CustomNetworkConfig{
			Networks: make(map[string]rpc.NetworkConfig),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.WrapConfigError("failed to read config file", err)
	}

	var config CustomNetworkConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.WrapConfigError("failed to parse config file", err)
	}

	if config.Networks == nil {
		config.Networks = make(map[string]rpc.NetworkConfig)
	}

	return &config, nil
}

// SaveCustomNetworks saves custom network configurations to disk
func SaveCustomNetworks(config *CustomNetworkConfig) error {
	configPath, err := GetNetworkConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return errors.WrapConfigError("failed to create config directory", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.WrapConfigError("failed to marshal config", err)
	}

	// Write with restricted permissions (owner only)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return errors.WrapConfigError("failed to write config file", err)
	}

	return nil
}

// AddCustomNetwork adds or updates a custom network configuration
func AddCustomNetwork(name string, config rpc.NetworkConfig) error {
	networks, err := LoadCustomNetworks()
	if err != nil {
		return err
	}

	config.Name = name
	networks.Networks[name] = config

	return SaveCustomNetworks(networks)
}

// GetCustomNetwork retrieves a custom network configuration by name
func GetCustomNetwork(name string) (*rpc.NetworkConfig, error) {
	networks, err := LoadCustomNetworks()
	if err != nil {
		return nil, err
	}

	config, exists := networks.Networks[name]
	if !exists {
		return nil, errors.WrapNetworkNotFound(name)
	}

	return &config, nil
}

// ListCustomNetworks returns all saved custom network names
func ListCustomNetworks() ([]string, error) {
	networks, err := LoadCustomNetworks()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(networks.Networks))
	for name := range networks.Networks {
		names = append(names, name)
	}

	return names, nil
}

// RemoveCustomNetwork removes a custom network configuration
func RemoveCustomNetwork(name string) error {
	networks, err := LoadCustomNetworks()
	if err != nil {
		return err
	}

	if _, exists := networks.Networks[name]; !exists {
		return errors.WrapNetworkNotFound(name)
	}

	delete(networks.Networks, name)

	return SaveCustomNetworks(networks)
}
