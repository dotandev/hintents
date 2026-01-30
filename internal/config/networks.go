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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		return "", fmt.Errorf("failed to get home directory: %w", err)
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
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config CustomNetworkConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
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
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restricted permissions (owner only)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
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
		return nil, fmt.Errorf("custom network '%s' not found", name)
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
		return fmt.Errorf("custom network '%s' not found", name)
	}

	delete(networks.Networks, name)

	return SaveCustomNetworks(networks)
}
