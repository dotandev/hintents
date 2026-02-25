// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

type Network string

const (
	NetworkPublic     Network = "public"
	NetworkTestnet    Network = "testnet"
	NetworkFuturenet  Network = "futurenet"
	NetworkStandalone Network = "standalone"
)

var validNetworks = map[string]bool{
	string(NetworkPublic):     true,
	string(NetworkTestnet):    true,
	string(NetworkFuturenet):  true,
	string(NetworkStandalone): true,
}

// Config represents the general configuration for erst
type Config struct {
	RpcUrl        string   `json:"rpc_url,omitempty"`
	RpcUrls       []string `json:"rpc_urls,omitempty"`
	Network       Network  `json:"network,omitempty"`
	SimulatorPath string   `json:"simulator_path,omitempty"`
	LogLevel      string   `json:"log_level,omitempty"`
	CachePath     string   `json:"cache_path,omitempty"`
	RPCToken      string   `json:"rpc_token,omitempty"`
}

var defaultConfig = &Config{
	RpcUrl:        "https://soroban-testnet.stellar.org",
	Network:       NetworkTestnet,
	SimulatorPath: "",
	LogLevel:      "info",
	CachePath:     filepath.Join(os.ExpandEnv("$HOME"), ".erst", "cache"),
}

// GetGeneralConfigPath returns the path to the general configuration file
func GetGeneralConfigPath() (string, error) {
	configDir, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// LoadConfig loads the general configuration from disk (JSON format)
func LoadConfig() (*Config, error) {
	configPath, err := GetGeneralConfigPath()
	if err != nil {
		return nil, err
	}

	// If file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.WrapConfigError("failed to read config file", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, errors.WrapConfigError("failed to parse config file", err)
	}

	return config, nil
}

// Load loads the configuration from environment variables and TOML files
func Load() (*Config, error) {
	cfg := &Config{
		RpcUrl:        getEnv("ERST_RPC_URL", defaultConfig.RpcUrl),
		Network:       Network(getEnv("ERST_NETWORK", string(defaultConfig.Network))),
		SimulatorPath: getEnv("ERST_SIMULATOR_PATH", defaultConfig.SimulatorPath),
		LogLevel:      getEnv("ERST_LOG_LEVEL", defaultConfig.LogLevel),
		CachePath:     getEnv("ERST_CACHE_PATH", defaultConfig.CachePath),
		RPCToken:      getEnv("ERST_RPC_TOKEN", ""),
	}

	if urlsEnv := os.Getenv("ERST_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = strings.Split(urlsEnv, ",")
		for i := range cfg.RpcUrls {
			cfg.RpcUrls[i] = strings.TrimSpace(cfg.RpcUrls[i])
		}
	} else if urlsEnv := os.Getenv("STELLAR_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = strings.Split(urlsEnv, ",")
		for i := range cfg.RpcUrls {
			cfg.RpcUrls[i] = strings.TrimSpace(cfg.RpcUrls[i])
		}
	}

	if err := cfg.loadFromFile(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) loadFromFile() error {
	paths := []string{
		".erst.toml",
		filepath.Join(os.ExpandEnv("$HOME"), ".erst.toml"),
		"/etc/erst/config.toml",
	}

	for _, path := range paths {
		if err := c.loadTOML(path); err == nil {
			return nil
		}
	}

	return nil
}

func (c *Config) loadTOML(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return c.parseTOML(string(data))
}

func (c *Config) parseTOML(content string) error {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		rawVal := strings.TrimSpace(parts[1])

		if key == "rpc_urls" && strings.HasPrefix(rawVal, "[") && strings.HasSuffix(rawVal, "]") {
			// Basic array parsing for TOML-like lists: ["a", "b"]
			rawVal = strings.Trim(rawVal, "[]")
			parts := strings.Split(rawVal, ",")
			var urls []string
			for _, p := range parts {
				urls = append(urls, strings.Trim(strings.TrimSpace(p), "\"'"))
			}
			c.RpcUrls = urls
			continue
		}

		value := strings.Trim(rawVal, "\"'")

		switch key {
		case "rpc_url":
			c.RpcUrl = value
		case "rpc_urls":
			// Fallback if not an array but comma-separated string
			c.RpcUrls = strings.Split(value, ",")
			for i := range c.RpcUrls {
				c.RpcUrls[i] = strings.TrimSpace(c.RpcUrls[i])
			}
		case "network":
			c.Network = Network(value)
		case "simulator_path":
			c.SimulatorPath = value
		case "log_level":
			c.LogLevel = value
		case "cache_path":
			c.CachePath = value
		case "rpc_token":
			c.RPCToken = value
		}
	}

	return nil
}

// SaveConfig saves the configuration to disk (JSON format)
func SaveConfig(config *Config) error {
	configPath, err := GetGeneralConfigPath()
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

func (c *Config) Validate() error {
	if c.RpcUrl == "" {
		return errors.WrapValidationError("rpc_url cannot be empty")
	}

	if c.Network != "" && !validNetworks[string(c.Network)] {
		return errors.WrapInvalidNetwork(string(c.Network))
	}

	return nil
}

func (c *Config) NetworkURL() string {
	switch c.Network {
	case NetworkPublic:
		return "https://soroban.stellar.org"
	case NetworkTestnet:
		return "https://soroban-testnet.stellar.org"
	case NetworkFuturenet:
		return "https://soroban-futurenet.stellar.org"
	case NetworkStandalone:
		return "http://localhost:8000"
	default:
		return c.RpcUrl
	}
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{RPC: %s, Network: %s, LogLevel: %s, CachePath: %s}",
		c.RpcUrl, c.Network, c.LogLevel, c.CachePath,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func DefaultConfig() *Config {
	return &Config{
		RpcUrl:        defaultConfig.RpcUrl,
		Network:       defaultConfig.Network,
		SimulatorPath: defaultConfig.SimulatorPath,
		LogLevel:      defaultConfig.LogLevel,
		CachePath:     defaultConfig.CachePath,
	}
}

func NewConfig(rpcUrl string, network Network) *Config {
	return &Config{
		RpcUrl:        rpcUrl,
		Network:       network,
		SimulatorPath: defaultConfig.SimulatorPath,
		LogLevel:      defaultConfig.LogLevel,
		CachePath:     defaultConfig.CachePath,
	}
}

func (c *Config) WithSimulatorPath(path string) *Config {
	c.SimulatorPath = path
	return c
}

func (c *Config) WithLogLevel(level string) *Config {
	c.LogLevel = level
	return c
}

func (c *Config) WithCachePath(path string) *Config {
	c.CachePath = path
	return c
}
