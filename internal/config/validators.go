// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

// Validator validates a specific aspect of the configuration.
type Validator interface {
	Validate(cfg *Config) error
}

// NetworkValidator checks that the configured network is recognized.
type NetworkValidator struct{}

func (v NetworkValidator) Validate(cfg *Config) error {
	if cfg.Network != "" && !validNetworks[string(cfg.Network)] {
		return errors.WrapInvalidNetwork(string(cfg.Network))
	}
	return nil
}

// RPCValidator checks that RPC connection fields are properly set.
type RPCValidator struct{}

func (v RPCValidator) Validate(cfg *Config) error {
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
			return errors.WrapValidationError(
				"rpc_urls[" + intToStr(i) + "] must use http or https scheme",
			)
		}
	}
	return nil
}

// SimulatorValidator checks that the simulator path, when set, looks valid.
type SimulatorValidator struct{}

func (v SimulatorValidator) Validate(cfg *Config) error {
	if cfg.SimulatorPath == "" {
		return nil
	}
	if !filepath.IsAbs(cfg.SimulatorPath) {
		return errors.WrapValidationError("simulator_path must be an absolute path")
	}
	return nil
}

// LogLevelValidator checks that the log level is a known value.
type LogLevelValidator struct{}

var validLogLevels = map[string]bool{
	"trace": true,
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

func (v LogLevelValidator) Validate(cfg *Config) error {
	if cfg.LogLevel == "" {
		return nil
	}
	if !validLogLevels[strings.ToLower(cfg.LogLevel)] {
		return errors.WrapValidationError("log_level must be one of: trace, debug, info, warn, error")
	}
	return nil
}

// DefaultValidators returns the standard set of validators.
func DefaultValidators() []Validator {
	return []Validator{
		RPCValidator{},
		NetworkValidator{},
		SimulatorValidator{},
		LogLevelValidator{},
	}
}

// RunValidators executes each validator against the config, returning the
// first error encountered.
func RunValidators(cfg *Config, validators []Validator) error {
	for _, v := range validators {
		if err := v.Validate(cfg); err != nil {
			return err
		}
	}
	return nil
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
