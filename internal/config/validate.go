// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

const maxRequestTimeout = 300

type TimeoutValidator struct{}

func (TimeoutValidator) Validate(cfg *Config) error {
	if cfg.RequestTimeout <= 0 {
		return errors.WrapValidationError("request_timeout must be greater than 0")
	}
	if cfg.RequestTimeout > maxRequestTimeout {
		return errors.WrapValidationError(
			fmt.Sprintf("request_timeout must be at most %d seconds, got %d", maxRequestTimeout, cfg.RequestTimeout),
		)
	}
	return nil
}

type CrashReportingValidator struct{}

func (CrashReportingValidator) Validate(cfg *Config) error {
	if !cfg.CrashReporting {
		return nil
	}
	if cfg.CrashEndpoint == "" && cfg.CrashSentryDSN == "" {
		return errors.WrapValidationError(
			"crash_reporting is enabled but neither crash_endpoint nor crash_sentry_dsn is set",
		)
	}
	if cfg.CrashSentryDSN != "" && !strings.HasPrefix(cfg.CrashSentryDSN, "https://") {
		return errors.WrapValidationError(
			fmt.Sprintf("crash_sentry_dsn must use https scheme, got %q", cfg.CrashSentryDSN),
		)
	}
	return nil
}

type MaxTraceDepthValidator struct{}

func (MaxTraceDepthValidator) Validate(cfg *Config) error {
	if cfg.MaxTraceDepth <= 0 {
		return errors.WrapValidationError("max_trace_depth must be greater than 0")
	}
	if cfg.MaxTraceDepth > 1000 {
		return errors.WrapValidationError(
			fmt.Sprintf("max_trace_depth must be at most 1000, got %d", cfg.MaxTraceDepth),
		)
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

type LogLevelValidator struct{}

func (LogLevelValidator) Validate(cfg *Config) error {
	if cfg.LogLevel == "" {
		return nil
	}
	lower := strings.ToLower(cfg.LogLevel)
	if !validLogLevels[lower] {
		return errors.WrapValidationError(fmt.Sprintf("log_level must be one of trace, debug, info, warn, error; got %q", cfg.LogLevel))
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

func isValidURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (RPCValidator) Validate(cfg *Config) error {
	if cfg.RpcUrl == "" {
		return errors.WrapValidationError("rpc_url cannot be empty")
	}
	if !isValidURL(cfg.RpcUrl) {
		return errors.WrapValidationError(fmt.Sprintf("invalid rpc_url scheme: %q (must be http or https)", cfg.RpcUrl))
	}
	for _, url := range cfg.RpcUrls {
		if !isValidURL(url) {
			return errors.WrapValidationError(fmt.Sprintf("invalid rpc_urls entry scheme: %q (must be http or https)", url))
		}
	}
	return nil
}
