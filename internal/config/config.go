// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

// -- Interfaces --

type Parser interface {
	Parse(*Config) error
}

type DefaultAssigner interface {
	Apply(*Config)
}

type Validator interface {
	Validate(*Config) error
}

// -- Types --

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
	RpcUrl            string   `json:"rpc_url,omitempty"`
	RpcUrls           []string `json:"rpc_urls,omitempty"`
	Network           Network  `json:"network,omitempty"`
	NetworkPassphrase string   `json:"network_passphrase,omitempty"`
	SimulatorPath     string   `json:"simulator_path,omitempty"`
	LogLevel          string   `json:"log_level,omitempty"`
	CachePath         string   `json:"cache_path,omitempty"`
	RPCToken          string   `json:"rpc_token,omitempty"`
	// MaxCacheSize is the maximum size of the source map cache in bytes.
	// Set via max_cache_size in config (e.g., "500MB" or bytes).
	// Defaults to no limit (cache grows indefinitely).
	MaxCacheSize int64 `json:"max_cache_size,omitempty"`
	// CrashReporting enables opt-in anonymous crash reporting.
	// Set via crash_reporting = true in config or ERST_CRASH_REPORTING=true.
	CrashReporting bool `json:"crash_reporting,omitempty"`
	// CrashEndpoint is a custom HTTPS URL that receives JSON crash reports.
	// Set via crash_endpoint in config or ERST_CRASH_ENDPOINT.
	CrashEndpoint string `json:"crash_endpoint,omitempty"`
	// CrashSentryDSN is a Sentry Data Source Name for crash reporting.
	// Set via crash_sentry_dsn in config or ERST_SENTRY_DSN.
	CrashSentryDSN string `json:"crash_sentry_dsn,omitempty"`
	// RequestTimeout is the HTTP request timeout in seconds for all RPC calls.
	// Set via request_timeout in config or ERST_REQUEST_TIMEOUT.
	// Defaults to 15 seconds.
	RequestTimeout int `json:"request_timeout,omitempty"`
}

// CustomNetworkConfig is defined in networks.go

// -- Constants & Defaults --

const defaultRequestTimeout = 15

var validLogLevels = map[string]bool{
	"trace": true,
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

var defaultConfig = &Config{
	RpcUrl:         "https://soroban-testnet.stellar.org",
	Network:        NetworkTestnet,
	SimulatorPath:  "",
	LogLevel:       "info",
	CachePath:      filepath.Join(os.ExpandEnv("$HOME"), ".erst", "cache"),
	RequestTimeout: defaultRequestTimeout,
	MaxCacheSize:   0,
}

// -- Core Functions --

func Load() (*Config, error) {
	cfg := &Config{}
	parsers := []Parser{envParser{}, fileParser{}}
	for _, parser := range parsers {
		if err := parser.Parse(cfg); err != nil {
			return nil, err
		}
	}

	configDefaultsAssigner{}.Apply(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		RpcUrl:         defaultConfig.RpcUrl,
		Network:        defaultConfig.Network,
		SimulatorPath:  defaultConfig.SimulatorPath,
		LogLevel:       defaultConfig.LogLevel,
		CachePath:      defaultConfig.CachePath,
		RequestTimeout: defaultConfig.RequestTimeout,
		MaxCacheSize:   defaultConfig.MaxCacheSize,
	}
}

func NewConfig(rpcUrl string, network Network) *Config {
	return &Config{
		RpcUrl:         rpcUrl,
		Network:        network,
		SimulatorPath:  defaultConfig.SimulatorPath,
		LogLevel:       defaultConfig.LogLevel,
		CachePath:      defaultConfig.CachePath,
		RequestTimeout: defaultConfig.RequestTimeout,
		MaxCacheSize:   defaultConfig.MaxCacheSize,
	}
}

// -- Config Methods --

func (c *Config) MergeDefaults() {
	configDefaultsAssigner{}.Apply(c)
}

func (c *Config) Validate() error {
	validators := []Validator{
		RPCValidator{},
		NetworkValidator{},
		SimulatorValidator{},
		LogLevelValidator{},
		TimeoutValidator{},
		CrashReportingValidator{},
	}
	for _, v := range validators {
		if err := v.Validate(c); err != nil {
			return err
		}
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

func (c *Config) WithRequestTimeout(seconds int) *Config {
	c.RequestTimeout = seconds
	return c
}

// -- Path Helpers --

// GetConfigPath is defined in networks.go
func GetGeneralConfigPath() (string, error) {
	configDir, err := GetConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

// -- Load/Save Config --
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

func SaveConfig(config *Config) error {
	configPath, err := GetGeneralConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return errors.WrapConfigError("failed to create config directory", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.WrapConfigError("failed to marshal config", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return errors.WrapConfigError("failed to write config file", err)
	}

	return nil
}

// -- Parsers --

type envParser struct{}

func (envParser) Parse(cfg *Config) error {
	if v := os.Getenv("ERST_RPC_URL"); v != "" {
		cfg.RpcUrl = v
	}
	if v := os.Getenv("ERST_NETWORK"); v != "" {
		cfg.Network = Network(v)
	}
	if v := os.Getenv("ERST_SIMULATOR_PATH"); v != "" {
		cfg.SimulatorPath = v
	}
	if v := os.Getenv("ERST_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("ERST_CACHE_PATH"); v != "" {
		cfg.CachePath = v
	}
	if v := os.Getenv("ERST_RPC_TOKEN"); v != "" {
		cfg.RPCToken = v
	}
	if v := os.Getenv("ERST_MAX_CACHE_SIZE"); v != "" {
		n, err := parseSize(v)
		if err == nil && n > 0 {
			cfg.MaxCacheSize = n
		}
	}
	if v := os.Getenv("ERST_CRASH_ENDPOINT"); v != "" {
		cfg.CrashEndpoint = v
	}
	if v := os.Getenv("ERST_SENTRY_DSN"); v != "" {
		cfg.CrashSentryDSN = v
	}

	if v := os.Getenv("ERST_REQUEST_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			cfg.RequestTimeout = n
		}
	}

	switch strings.ToLower(os.Getenv("ERST_CRASH_REPORTING")) {
	case "1", "true", "yes":
		cfg.CrashReporting = true
	case "0", "false", "no":
		cfg.CrashReporting = false
	}

	if urlsEnv := os.Getenv("ERST_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = splitAndTrim(urlsEnv)
	} else if urlsEnv := os.Getenv("STELLAR_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = splitAndTrim(urlsEnv)
	}

	return nil
}

type fileParser struct{}

func (fileParser) Parse(cfg *Config) error {
	paths := []string{
		".erst.toml",
		filepath.Join(os.ExpandEnv("$HOME"), ".erst.toml"),
		"/etc/erst/config.toml",
	}

	for _, path := range paths {
		if err := cfg.loadTOML(path); err == nil {
			return nil
		}
	}

	return nil
}

// Parsers are defined in parse.go or kept here if needed by Load()

// -- Validators --

type RPCValidator struct{}

func (RPCValidator) Validate(cfg *Config) error {
	if cfg.RpcUrl == "" {
		return errors.WrapValidationError("rpc_url cannot be empty")
	}
	if !strings.HasPrefix(cfg.RpcUrl, "http://") && !strings.HasPrefix(cfg.RpcUrl, "https://") {
		return errors.WrapValidationError("rpc_url must use http or https scheme")
	}
	for i, u := range cfg.RpcUrls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return errors.WrapValidationError("rpc_urls[" + strconv.Itoa(i) + "] must use http or https scheme")
		}
	}
	return nil
}

type NetworkValidator struct{}

func (NetworkValidator) Validate(cfg *Config) error {
	if cfg.Network != "" && !validNetworks[string(cfg.Network)] {
		return errors.WrapInvalidNetwork(string(cfg.Network))
	}
	return nil
}

type SimulatorValidator struct{}

func (SimulatorValidator) Validate(cfg *Config) error {
	if cfg.SimulatorPath == "" {
		return nil
	}
	if !filepath.IsAbs(cfg.SimulatorPath) {
		return errors.WrapValidationError("simulator_path must be an absolute path")
	}
	return nil
}

type LogLevelValidator struct{}

func (LogLevelValidator) Validate(cfg *Config) error {
	if cfg.LogLevel == "" {
		return nil
	}
	if !validLogLevels[strings.ToLower(cfg.LogLevel)] {
		return errors.WrapValidationError("log_level must be one of: trace, debug, info, warn, error")
	}
	return nil
}

// Validators are defined in dedicated files

type configDefaultsAssigner struct{}

func (configDefaultsAssigner) Apply(cfg *Config) {
	if cfg.RpcUrl == "" {
		cfg.RpcUrl = defaultConfig.RpcUrl
	}
	if cfg.Network == "" {
		cfg.Network = defaultConfig.Network
	}
	if cfg.SimulatorPath == "" {
		cfg.SimulatorPath = defaultConfig.SimulatorPath
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultConfig.LogLevel
	}
	if cfg.CachePath == "" {
		cfg.CachePath = defaultConfig.CachePath
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = defaultRequestTimeout
	}
}

// -- Internal Helpers --

// End of config.go
