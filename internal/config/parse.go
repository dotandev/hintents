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

func loadFromEnv(cfg *Config) error { //nolint:unused // Reserved for future config loading from environment
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
		if err != nil {
			return errors.WrapValidationError("ERST_MAX_CACHE_SIZE must be a valid size (e.g., 500MB)")
		}
		cfg.MaxCacheSize = n
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

	if v := os.Getenv("ERST_MAX_TRACE_DEPTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return errors.WrapValidationError("ERST_MAX_TRACE_DEPTH must be an integer")
		}
		cfg.MaxTraceDepth = n
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
		case "max_trace_depth":
			n, err := strconv.Atoi(value)
			if err != nil {
				return errors.WrapValidationError("max_trace_depth must be an integer")
			}
			c.MaxTraceDepth = n
		case "max_cache_size":
			n, err := parseSize(value)
			if err != nil {
				return errors.WrapValidationError("max_cache_size must be a valid size (e.g., 500MB)")
			}
			c.MaxCacheSize = n
		case "failure_threshold":
			n, err := strconv.Atoi(value)
			if err != nil {
				return errors.WrapValidationError("failure_threshold must be an integer")
			}
			c.FailureThreshold = n
		case "retry_timeout":
			n, err := strconv.Atoi(value)
			if err != nil {
				return errors.WrapValidationError("retry_timeout must be an integer")
			}
			c.RetryTimeout = n
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

func parseSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	var multiplier int64 = 1
	lowerValue := strings.ToLower(value)

	if strings.HasSuffix(lowerValue, "kb") {
		multiplier = 1024
		value = strings.TrimSuffix(value, "kb")
	} else if strings.HasSuffix(lowerValue, "mb") {
		multiplier = 1024 * 1024
		value = strings.TrimSuffix(value, "mb")
	} else if strings.HasSuffix(lowerValue, "gb") {
		multiplier = 1024 * 1024 * 1024
		value = strings.TrimSuffix(value, "gb")
	} else if strings.HasSuffix(lowerValue, "k") {
		multiplier = 1000
		value = strings.TrimSuffix(value, "k")
	} else if strings.HasSuffix(lowerValue, "m") {
		multiplier = 1000 * 1000
		value = strings.TrimSuffix(value, "m")
	} else if strings.HasSuffix(lowerValue, "g") {
		multiplier = 1000 * 1000 * 1000
		value = strings.TrimSuffix(value, "g")
	}

	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}

	return n * multiplier, nil
}
