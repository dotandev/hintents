// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

func loadFromEnv(cfg *Config) error {
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
	if v := os.Getenv("ERST_CRASH_ENDPOINT"); v != "" {
		cfg.CrashEndpoint = v
	}
	if v := os.Getenv("ERST_SENTRY_DSN"); v != "" {
		cfg.CrashSentryDSN = v
	}

	if v := os.Getenv("ERST_REQUEST_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return errors.WrapValidationError("ERST_REQUEST_TIMEOUT must be an integer")
		}
		cfg.RequestTimeout = n
	}

	switch strings.ToLower(os.Getenv("ERST_CRASH_REPORTING")) {
	case "":
	case "1", "true", "yes":
		cfg.CrashReporting = true
	case "0", "false", "no":
		cfg.CrashReporting = false
	default:
		return errors.WrapValidationError("ERST_CRASH_REPORTING must be a boolean")
	}

	if urlsEnv := os.Getenv("ERST_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = splitAndTrim(urlsEnv)
	} else if urlsEnv := os.Getenv("STELLAR_RPC_URLS"); urlsEnv != "" {
		cfg.RpcUrls = splitAndTrim(urlsEnv)
	}

	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
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
			rawVal = strings.Trim(rawVal, "[]")
			elems := strings.Split(rawVal, ",")
			var urls []string
			for _, p := range elems {
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
			c.RpcUrls = splitAndTrim(value)
		case "network":
			c.Network = Network(value)
		case "network_passphrase":
			c.NetworkPassphrase = value
		case "simulator_path":
			c.SimulatorPath = value
		case "log_level":
			c.LogLevel = value
		case "cache_path":
			c.CachePath = value
		case "rpc_token":
			c.RPCToken = value
		case "crash_reporting":
			switch strings.ToLower(value) {
			case "true", "1", "yes":
				c.CrashReporting = true
			case "false", "0", "no":
				c.CrashReporting = false
			default:
				return errors.WrapValidationError("crash_reporting must be a boolean")
			}
		case "crash_endpoint":
			c.CrashEndpoint = value
		case "crash_sentry_dsn":
			c.CrashSentryDSN = value
		case "request_timeout":
			n, err := strconv.Atoi(value)
			if err != nil {
				return errors.WrapValidationError("request_timeout must be an integer")
			}
			c.RequestTimeout = n
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if !strings.HasPrefix(key, "ERST_") {
		return defaultValue
	}
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
