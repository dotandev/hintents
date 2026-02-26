// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig("https://test.com", NetworkTestnet)

	if cfg.RpcUrl != "https://test.com" {
		t.Errorf("expected RpcUrl 'https://test.com', got %s", cfg.RpcUrl)
	}

	if cfg.Network != NetworkTestnet {
		t.Errorf("expected Network testnet, got %s", cfg.Network)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RpcUrl == "" {
		t.Error("expected non-empty RpcUrl")
	}

	if cfg.Network == "" {
		t.Error("expected non-empty Network")
	}

	if cfg.CachePath == "" {
		t.Error("expected non-empty CachePath")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			"valid public network",
			&Config{RpcUrl: "https://test.com", Network: NetworkPublic},
			false,
		},
		{
			"valid testnet",
			&Config{RpcUrl: "https://test.com", Network: NetworkTestnet},
			false,
		},
		{
			"valid futurenet",
			&Config{RpcUrl: "https://test.com", Network: NetworkFuturenet},
			false,
		},
		{
			"valid standalone",
			&Config{RpcUrl: "https://test.com", Network: NetworkStandalone},
			false,
		},
		{
			"empty RpcUrl",
			&Config{RpcUrl: "", Network: NetworkTestnet},
			true,
		},
		{
			"invalid network",
			&Config{RpcUrl: "https://test.com", Network: Network("invalid")},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNetworkURL(t *testing.T) {
	tests := []struct {
		network Network
		want    string
	}{
		{NetworkPublic, "https://soroban.stellar.org"},
		{NetworkTestnet, "https://soroban-testnet.stellar.org"},
		{NetworkFuturenet, "https://soroban-futurenet.stellar.org"},
		{NetworkStandalone, "http://localhost:8000"},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			cfg := NewConfig("", tt.network)
			got := cfg.NetworkURL()
			if got != tt.want {
				t.Errorf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestConfigBuilder(t *testing.T) {
	cfg := NewConfig("https://test.com", NetworkTestnet).
		WithSimulatorPath("/path/to/sim").
		WithLogLevel("debug").
		WithCachePath("/custom/cache")

	if cfg.SimulatorPath != "/path/to/sim" {
		t.Errorf("expected simulator path /path/to/sim, got %s", cfg.SimulatorPath)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}

	if cfg.CachePath != "/custom/cache" {
		t.Errorf("expected cache path /custom/cache, got %s", cfg.CachePath)
	}
}

func TestConfigString(t *testing.T) {
	cfg := NewConfig("https://test.com", NetworkTestnet)
	str := cfg.String()

	if !strings.Contains(str, "https://test.com") {
		t.Error("expected RpcUrl in string representation")
	}

	if !strings.Contains(str, "testnet") {
		t.Error("expected Network in string representation")
	}
}

func TestParseTOML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Config
	}{
		{
			"simple TOML",
			`rpc_url = "https://custom.com"
network = "public"`,
			&Config{RpcUrl: "https://custom.com", Network: NetworkPublic},
		},
		{
			"TOML with comments",
			`# Configuration
rpc_url = "https://custom.com"
# Network selection
network = "testnet"`,
			&Config{RpcUrl: "https://custom.com", Network: NetworkTestnet},
		},
		{
			"TOML with all fields",
			`rpc_url = "https://custom.com"
network = "futurenet"
network_passphrase = "Test SDF Future Network ; October 2022"
simulator_path = "/path/to/sim"
log_level = "debug"
cache_path = "/custom/cache"`,
			&Config{
				RpcUrl:            "https://custom.com",
				Network:           NetworkFuturenet,
				NetworkPassphrase: "Test SDF Future Network ; October 2022",
				SimulatorPath:     "/path/to/sim",
				LogLevel:          "debug",
				CachePath:         "/custom/cache",
			},
		},
		{
			"TOML with rpc_urls array",
			`rpc_urls = ["https://rpc1.com", "https://rpc2.com"]
network = "testnet"`,
			&Config{
				RpcUrls: []string{"https://rpc1.com", "https://rpc2.com"},
				Network: NetworkTestnet,
			},
		},
		{
			"TOML with rpc_urls comma string",
			`rpc_urls = "https://rpc1.com,https://rpc2.com"`,
			&Config{
				RpcUrls: []string{"https://rpc1.com", "https://rpc2.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			err := cfg.parseTOML(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.RpcUrl != tt.want.RpcUrl {
				t.Errorf("RpcUrl: expected %s, got %s", tt.want.RpcUrl, cfg.RpcUrl)
			}

			if len(cfg.RpcUrls) != len(tt.want.RpcUrls) {
				t.Errorf("RpcUrls count: expected %d, got %d", len(tt.want.RpcUrls), len(cfg.RpcUrls))
			} else {
				for i := range cfg.RpcUrls {
					if cfg.RpcUrls[i] != tt.want.RpcUrls[i] {
						t.Errorf("RpcUrls[%d]: expected %s, got %s", i, tt.want.RpcUrls[i], cfg.RpcUrls[i])
					}
				}
			}

			if cfg.Network != tt.want.Network {
				t.Errorf("Network: expected %s, got %s", tt.want.Network, cfg.Network)
			}

			if cfg.NetworkPassphrase != tt.want.NetworkPassphrase {
				t.Errorf("NetworkPassphrase: expected %s, got %s", tt.want.NetworkPassphrase, cfg.NetworkPassphrase)
			}

			if cfg.SimulatorPath != tt.want.SimulatorPath {
				t.Errorf("SimulatorPath: expected %s, got %s", tt.want.SimulatorPath, cfg.SimulatorPath)
			}

			if cfg.LogLevel != tt.want.LogLevel {
				t.Errorf("LogLevel: expected %s, got %s", tt.want.LogLevel, cfg.LogLevel)
			}

			if cfg.CachePath != tt.want.CachePath {
				t.Errorf("CachePath: expected %s, got %s", tt.want.CachePath, cfg.CachePath)
			}
		})
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	// Save original env vars
	origRpc := os.Getenv("ERST_RPC_URL")
	origNet := os.Getenv("ERST_NETWORK")
	origLog := os.Getenv("ERST_LOG_LEVEL")

	defer func() {
		os.Setenv("ERST_RPC_URL", origRpc)
		os.Setenv("ERST_NETWORK", origNet)
		os.Setenv("ERST_LOG_LEVEL", origLog)
	}()

	os.Setenv("ERST_RPC_URL", "https://env.test.com")
	os.Setenv("ERST_NETWORK", "public")
	os.Setenv("ERST_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RpcUrl != "https://env.test.com" {
		t.Errorf("expected RpcUrl from env, got %s", cfg.RpcUrl)
	}

	if cfg.Network != NetworkPublic {
		t.Errorf("expected Network from env, got %s", cfg.Network)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel from env, got %s", cfg.LogLevel)
	}
}

func TestLoadTOMLFile(t *testing.T) {
	tmpdir := t.TempDir()
	configPath := filepath.Join(tmpdir, "test.toml")

	content := `rpc_url = "https://file.test.com"
network = "testnet"
log_level = "trace"`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg := &Config{}
	err := cfg.loadTOML(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RpcUrl != "https://file.test.com" {
		t.Errorf("expected RpcUrl from file, got %s", cfg.RpcUrl)
	}

	if cfg.Network != NetworkTestnet {
		t.Errorf("expected Network from file, got %s", cfg.Network)
	}
}

func TestValidNetworks(t *testing.T) {
	networks := []Network{NetworkPublic, NetworkTestnet, NetworkFuturenet, NetworkStandalone}

	for _, net := range networks {
		cfg := NewConfig("https://test.com", net)
		if err := cfg.Validate(); err != nil {
			t.Errorf("network %s should be valid: %v", net, err)
		}
	}
}

func TestConfigCopy(t *testing.T) {
	original := NewConfig("https://test.com", NetworkTestnet).
		WithLogLevel("debug").
		WithCachePath("/cache")

	copy := &Config{
		RpcUrl:        original.RpcUrl,
		Network:       original.Network,
		LogLevel:      original.LogLevel,
		CachePath:     original.CachePath,
		SimulatorPath: original.SimulatorPath,
	}

	if original.RpcUrl != copy.RpcUrl {
		t.Error("RpcUrl mismatch in copy")
	}

	if original.Network != copy.Network {
		t.Error("Network mismatch in copy")
	}

	copy.LogLevel = "info"
	if original.LogLevel == copy.LogLevel {
		t.Error("copy should not affect original")
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	cfg := NewConfig("https://test.com", NetworkTestnet)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Validate()
	}
}

func BenchmarkParseTOML(b *testing.B) {
	content := `rpc_url = "https://test.com"
network = "testnet"
log_level = "info"
simulator_path = "/path/to/sim"
cache_path = "/cache"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := &Config{}
		_ = cfg.parseTOML(content)
	}
}

// ---- Crash reporting config -------------------------------------------------

func TestParseTOML_CrashReportingFields(t *testing.T) {
	content := `rpc_url = "https://test.com"
network = "testnet"
crash_reporting = true
crash_endpoint = "https://custom.example.com/crash"
crash_sentry_dsn = "https://key@o0.ingest.sentry.io/1"`

	cfg := &Config{}
	if err := cfg.parseTOML(content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.CrashReporting {
		t.Error("expected CrashReporting=true")
	}
	if cfg.CrashEndpoint != "https://custom.example.com/crash" {
		t.Errorf("expected CrashEndpoint from TOML, got %q", cfg.CrashEndpoint)
	}
	if cfg.CrashSentryDSN != "https://key@o0.ingest.sentry.io/1" {
		t.Errorf("expected CrashSentryDSN from TOML, got %q", cfg.CrashSentryDSN)
	}
}

func TestParseTOML_CrashReportingDisabledByDefault(t *testing.T) {
	cfg := &Config{}
	if err := cfg.parseTOML(`rpc_url = "https://test.com"`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CrashReporting {
		t.Error("CrashReporting should default to false")
	}
	if cfg.CrashEndpoint != "" {
		t.Errorf("CrashEndpoint should default to empty, got %q", cfg.CrashEndpoint)
	}
	if cfg.CrashSentryDSN != "" {
		t.Errorf("CrashSentryDSN should default to empty, got %q", cfg.CrashSentryDSN)
	}
}

func TestLoad_CrashReportingEnvVars(t *testing.T) {
	keys := []string{
		"ERST_CRASH_REPORTING",
		"ERST_CRASH_ENDPOINT",
		"ERST_SENTRY_DSN",
	}
	orig := make(map[string]string, len(keys))
	for _, k := range keys {
		orig[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range orig {
			os.Setenv(k, v)
		}
	}()

	os.Setenv("ERST_CRASH_REPORTING", "true")
	os.Setenv("ERST_CRASH_ENDPOINT", "https://custom.example.com/crash")
	os.Setenv("ERST_SENTRY_DSN", "https://key@o0.ingest.sentry.io/2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.CrashReporting {
		t.Error("expected CrashReporting=true from ERST_CRASH_REPORTING")
	}
	if cfg.CrashEndpoint != "https://custom.example.com/crash" {
		t.Errorf("expected CrashEndpoint from env, got %q", cfg.CrashEndpoint)
	}
	if cfg.CrashSentryDSN != "https://key@o0.ingest.sentry.io/2" {
		t.Errorf("expected CrashSentryDSN from env, got %q", cfg.CrashSentryDSN)
	}
}

func TestLoad_CrashReportingOffByDefault(t *testing.T) {
	for _, k := range []string{"ERST_CRASH_REPORTING", "ERST_CRASH_ENDPOINT", "ERST_SENTRY_DSN"} {
		os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.CrashReporting {
		t.Error("CrashReporting should be off by default")
	}
}

// ---- RequestTimeout config --------------------------------------------------

func TestDefaultConfig_RequestTimeout(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RequestTimeout != 15 {
		t.Errorf("expected default RequestTimeout=15, got %d", cfg.RequestTimeout)
	}
}

func TestLoad_RequestTimeoutFromEnv(t *testing.T) {
	orig := os.Getenv("ERST_REQUEST_TIMEOUT")
	defer os.Setenv("ERST_REQUEST_TIMEOUT", orig)

	os.Setenv("ERST_REQUEST_TIMEOUT", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestTimeout != 30 {
		t.Errorf("expected RequestTimeout=30 from env, got %d", cfg.RequestTimeout)
	}
}

func TestLoad_RequestTimeoutInvalidEnvIgnored(t *testing.T) {
	orig := os.Getenv("ERST_REQUEST_TIMEOUT")
	defer os.Setenv("ERST_REQUEST_TIMEOUT", orig)

	os.Setenv("ERST_REQUEST_TIMEOUT", "notanumber")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestTimeout != 15 {
		t.Errorf("expected default RequestTimeout=15 for invalid env value, got %d", cfg.RequestTimeout)
	}
}

func TestLoad_RequestTimeoutZeroEnvIgnored(t *testing.T) {
	orig := os.Getenv("ERST_REQUEST_TIMEOUT")
	defer os.Setenv("ERST_REQUEST_TIMEOUT", orig)

	os.Setenv("ERST_REQUEST_TIMEOUT", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestTimeout != 15 {
		t.Errorf("expected default RequestTimeout=15 for zero env value, got %d", cfg.RequestTimeout)
	}
}

func TestParseTOML_RequestTimeout(t *testing.T) {
	content := `rpc_url = "https://test.com"
network = "testnet"
request_timeout = 60`

	cfg := &Config{}
	if err := cfg.parseTOML(content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestTimeout != 60 {
		t.Errorf("expected RequestTimeout=60 from TOML, got %d", cfg.RequestTimeout)
	}
}

func TestParseTOML_RequestTimeoutInvalidIgnored(t *testing.T) {
	content := `rpc_url = "https://test.com"
request_timeout = -5`

	cfg := &Config{}
	if err := cfg.parseTOML(content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequestTimeout != 0 {
		t.Errorf("expected RequestTimeout unchanged for negative value, got %d", cfg.RequestTimeout)
	}
}

func TestWithRequestTimeout(t *testing.T) {
	cfg := NewConfig("https://test.com", NetworkTestnet).WithRequestTimeout(45)
	if cfg.RequestTimeout != 45 {
		t.Errorf("expected RequestTimeout=45, got %d", cfg.RequestTimeout)
	}
}
