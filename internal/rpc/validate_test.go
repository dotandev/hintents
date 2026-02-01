// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"
)

func TestIsValidURL_ValidURLs(t *testing.T) {
	validURLs := []string{
		"https://horizon.stellar.org",
		"https://horizon-testnet.stellar.org/",
		"http://localhost:8000",
		"https://soroban-testnet.stellar.org:443",
		"http://192.168.1.1:8080",
	}

	for _, urlStr := range validURLs {
		t.Run(urlStr, func(t *testing.T) {
			err := isValidURL(urlStr)
			if err != nil {
				t.Errorf("expected no error for valid URL %q, got %v", urlStr, err)
			}
		})
	}
}

func TestIsValidURL_InvalidURLs(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		hasErr bool
	}{
		{"empty URL", "", true},
		{"no scheme", "horizon.stellar.org", true},
		{"invalid scheme", "ftp://example.com", true},
		{"no host", "https://", true},
		{"malformed", "ht!ps://example.com", true},
		{"only path", "/path/to/resource", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidURL(tt.url)
			if (err != nil) != tt.hasErr {
				t.Errorf("expected error=%v, got error=%v", tt.hasErr, err != nil)
			}
		})
	}
}

func TestValidateNetworkConfig_ValidWithOnlyHorizonURL(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "https://horizon.example.com",
		NetworkPassphrase: "Custom Network",
	}

	err := ValidateNetworkConfig(config)
	if err != nil {
		t.Errorf("expected no error with only HorizonURL, got %v", err)
	}
}

func TestValidateNetworkConfig_ValidWithOnlySorobanURL(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		SorobanRPCURL:     "https://soroban.example.com",
		NetworkPassphrase: "Custom Network",
	}

	err := ValidateNetworkConfig(config)
	if err != nil {
		t.Errorf("expected no error with only SorobanRPCURL, got %v", err)
	}
}

func TestValidateNetworkConfig_MissingPassphrase(t *testing.T) {
	config := NetworkConfig{
		Name:       "custom",
		HorizonURL: "https://horizon.example.com",
	}

	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error for missing passphrase")
	}
}

func TestValidateNetworkConfig_NoURLs(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		NetworkPassphrase: "Custom Network",
	}

	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error when both URLs are empty")
	}
}

func TestValidateNetworkConfig_InvalidHorizonURL(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "not-a-url",
		NetworkPassphrase: "Custom Network",
	}

	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error for invalid HorizonURL")
	}
}

func TestValidateNetworkConfig_InvalidSorobanURL(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "https://horizon.example.com",
		SorobanRPCURL:     "ftp://soroban.example.com",
		NetworkPassphrase: "Custom Network",
	}

	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error for invalid SorobanRPCURL")
	}
}

func TestValidateNetworkConfig_Testnet(t *testing.T) {
	err := ValidateNetworkConfig(TestnetConfig)
	if err != nil {
		t.Errorf("expected no error for Testnet config, got %v", err)
	}
}

func TestValidateNetworkConfig_Mainnet(t *testing.T) {
	err := ValidateNetworkConfig(MainnetConfig)
	if err != nil {
		t.Errorf("expected no error for Mainnet config, got %v", err)
	}
}

func TestValidateNetworkConfig_Futurenet(t *testing.T) {
	err := ValidateNetworkConfig(FuturenetConfig)
	if err != nil {
		t.Errorf("expected no error for Futurenet config, got %v", err)
	}
}

func BenchmarkValidateNetworkConfig(b *testing.B) {
	config := NetworkConfig{
		Name:              "testnet",
		HorizonURL:        "https://horizon-testnet.stellar.org",
		NetworkPassphrase: "Test SDF Network ; September 2015",
		SorobanRPCURL:     "https://soroban-testnet.stellar.org",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateNetworkConfig(config)
	}
}

func BenchmarkIsValidURL(b *testing.B) {
	url := "https://horizon-testnet.stellar.org"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isValidURL(url)
	}
}
