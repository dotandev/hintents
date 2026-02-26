// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
)

// --- NetworkValidator ---

func TestNetworkValidator_ValidNetworks(t *testing.T) {
	v := NetworkValidator{}
	for _, net := range []Network{NetworkPublic, NetworkTestnet, NetworkFuturenet, NetworkStandalone} {
		cfg := &Config{RpcUrl: "https://test.com", Network: net}
		if err := v.Validate(cfg); err != nil {
			t.Errorf("network %q should be valid: %v", net, err)
		}
	}
}

func TestNetworkValidator_EmptyAllowed(t *testing.T) {
	v := NetworkValidator{}
	cfg := &Config{RpcUrl: "https://test.com", Network: ""}
	if err := v.Validate(cfg); err != nil {
		t.Errorf("empty network should be allowed: %v", err)
	}
}

func TestNetworkValidator_InvalidNetwork(t *testing.T) {
	v := NetworkValidator{}
	cases := []string{"mainnet", "TESTNET", "Futurenet", "invalid", "prod", " testnet"}
	for _, net := range cases {
		cfg := &Config{RpcUrl: "https://test.com", Network: Network(net)}
		if err := v.Validate(cfg); err == nil {
			t.Errorf("network %q should be invalid", net)
		}
	}
}

// --- RPCValidator ---

func TestRPCValidator_ValidURL(t *testing.T) {
	v := RPCValidator{}
	cases := []string{
		"https://soroban-testnet.stellar.org",
		"http://localhost:8000",
		"https://rpc.example.com/v1",
	}
	for _, url := range cases {
		cfg := &Config{RpcUrl: url}
		if err := v.Validate(cfg); err != nil {
			t.Errorf("rpc_url %q should be valid: %v", url, err)
		}
	}
}

func TestRPCValidator_EmptyURL(t *testing.T) {
	v := RPCValidator{}
	cfg := &Config{RpcUrl: ""}
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty rpc_url")
	}
	if !strings.Contains(err.Error(), "rpc_url") {
		t.Errorf("error should mention rpc_url, got: %v", err)
	}
}

func TestRPCValidator_InvalidScheme(t *testing.T) {
	v := RPCValidator{}
	cases := []string{"ftp://rpc.example.com", "ws://rpc.example.com", "soroban-testnet.stellar.org"}
	for _, url := range cases {
		cfg := &Config{RpcUrl: url}
		if err := v.Validate(cfg); err == nil {
			t.Errorf("rpc_url %q should be rejected (bad scheme)", url)
		}
	}
}

func TestRPCValidator_ValidRpcUrls(t *testing.T) {
	v := RPCValidator{}
	cfg := &Config{
		RpcUrl:  "https://primary.example.com",
		RpcUrls: []string{"https://rpc1.com", "https://rpc2.com"},
	}
	if err := v.Validate(cfg); err != nil {
		t.Errorf("valid rpc_urls should pass: %v", err)
	}
}

func TestRPCValidator_InvalidRpcUrlsEntry(t *testing.T) {
	v := RPCValidator{}
	cfg := &Config{
		RpcUrl:  "https://primary.example.com",
		RpcUrls: []string{"https://rpc1.com", "ftp://bad.com"},
	}
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid rpc_urls entry")
	}
	if !strings.Contains(err.Error(), "rpc_urls") {
		t.Errorf("error should mention rpc_urls, got: %v", err)
	}
}

// --- SimulatorValidator ---

func TestSimulatorValidator_Empty(t *testing.T) {
	v := SimulatorValidator{}
	cfg := &Config{SimulatorPath: ""}
	if err := v.Validate(cfg); err != nil {
		t.Errorf("empty simulator_path should be valid: %v", err)
	}
}

func TestSimulatorValidator_AbsolutePath(t *testing.T) {
	v := SimulatorValidator{}
	cfg := &Config{SimulatorPath: "/usr/local/bin/soroban-sim"}
	if err := v.Validate(cfg); err != nil {
		t.Errorf("absolute path should be valid: %v", err)
	}
}

func TestSimulatorValidator_RelativePath(t *testing.T) {
	v := SimulatorValidator{}
	cfg := &Config{SimulatorPath: "relative/path/sim"}
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for relative simulator_path")
	}
	if !strings.Contains(err.Error(), "simulator_path") {
		t.Errorf("error should mention simulator_path, got: %v", err)
	}
}

// --- LogLevelValidator ---

func TestLogLevelValidator_ValidLevels(t *testing.T) {
	v := LogLevelValidator{}
	for _, lvl := range []string{"trace", "debug", "info", "warn", "error"} {
		cfg := &Config{LogLevel: lvl}
		if err := v.Validate(cfg); err != nil {
			t.Errorf("log_level %q should be valid: %v", lvl, err)
		}
	}
}

func TestLogLevelValidator_Empty(t *testing.T) {
	v := LogLevelValidator{}
	cfg := &Config{LogLevel: ""}
	if err := v.Validate(cfg); err != nil {
		t.Errorf("empty log_level should be valid: %v", err)
	}
}

func TestLogLevelValidator_InvalidLevel(t *testing.T) {
	v := LogLevelValidator{}
	cases := []string{"verbose", "fatal", "notice", "123", "WARNING"}
	for _, lvl := range cases {
		cfg := &Config{LogLevel: lvl}
		if err := v.Validate(cfg); err == nil {
			t.Errorf("log_level %q should be invalid", lvl)
		}
	}
}

// --- RunValidators ---

func TestRunValidators_StopsOnFirstError(t *testing.T) {
	cfg := &Config{RpcUrl: "", Network: Network("invalid")}
	err := RunValidators(cfg, DefaultValidators())
	if err == nil {
		t.Fatal("expected error from RunValidators")
	}
	// RPCValidator runs first, so the error should be about rpc_url.
	if !strings.Contains(err.Error(), "rpc_url") {
		t.Errorf("expected rpc_url error first, got: %v", err)
	}
}

func TestRunValidators_AllPass(t *testing.T) {
	cfg := &Config{
		RpcUrl:   "https://soroban-testnet.stellar.org",
		Network:  NetworkTestnet,
		LogLevel: "info",
	}
	if err := RunValidators(cfg, DefaultValidators()); err != nil {
		t.Errorf("all validators should pass: %v", err)
	}
}

func TestRunValidators_CustomSet(t *testing.T) {
	cfg := &Config{RpcUrl: "https://test.com", Network: Network("bogus")}
	// Only run NetworkValidator.
	err := RunValidators(cfg, []Validator{NetworkValidator{}})
	if err == nil {
		t.Fatal("expected NetworkValidator error")
	}
}

// --- MergeDefaults ---

func TestMergeDefaults_FillsEmptyFields(t *testing.T) {
	cfg := &Config{}
	cfg.MergeDefaults()

	if cfg.RpcUrl == "" {
		t.Error("expected RpcUrl to be filled by MergeDefaults")
	}
	if cfg.Network == "" {
		t.Error("expected Network to be filled by MergeDefaults")
	}
	if cfg.LogLevel == "" {
		t.Error("expected LogLevel to be filled by MergeDefaults")
	}
	if cfg.CachePath == "" {
		t.Error("expected CachePath to be filled by MergeDefaults")
	}
}

func TestMergeDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		RpcUrl:   "https://custom.example.com",
		Network:  NetworkPublic,
		LogLevel: "debug",
	}
	cfg.MergeDefaults()

	if cfg.RpcUrl != "https://custom.example.com" {
		t.Errorf("MergeDefaults should not overwrite RpcUrl, got %s", cfg.RpcUrl)
	}
	if cfg.Network != NetworkPublic {
		t.Errorf("MergeDefaults should not overwrite Network, got %s", cfg.Network)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("MergeDefaults should not overwrite LogLevel, got %s", cfg.LogLevel)
	}
}

// --- Validate integration (delegates to validators) ---

func TestValidate_Delegates(t *testing.T) {
	cfg := &Config{RpcUrl: "https://test.com", Network: NetworkTestnet, LogLevel: "info"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should pass Validate: %v", err)
	}

	cfg2 := &Config{RpcUrl: "", Network: NetworkTestnet}
	if err := cfg2.Validate(); err == nil {
		t.Error("expected Validate to reject empty rpc_url")
	}
}
