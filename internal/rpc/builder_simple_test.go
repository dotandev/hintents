// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"
)

func TestBuilderCreateDefaultClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
		return
	}
	if client.Network != Mainnet {
		t.Errorf("expected default network Mainnet, got %v", client.Network)
	}
	if client.HorizonURL == "" {
		t.Error("expected HorizonURL to be set")
	}
}

func TestBuilderWithNetworkTestnet(t *testing.T) {
	client, err := NewClient(WithNetwork(Testnet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Testnet {
		t.Errorf("expected network Testnet, got %v", client.Network)
	}
	if client.HorizonURL != TestnetHorizonURL {
		t.Errorf("expected HorizonURL %s, got %s", TestnetHorizonURL, client.HorizonURL)
	}
}

func TestBuilderWithNetworkFuturenet(t *testing.T) {
	client, err := NewClient(WithNetwork(Futurenet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Futurenet {
		t.Errorf("expected network Futurenet, got %v", client.Network)
	}
	if client.HorizonURL != FuturenetHorizonURL {
		t.Errorf("expected HorizonURL %s, got %s", FuturenetHorizonURL, client.HorizonURL)
	}
}

func TestBuilderWithToken(t *testing.T) {
	token := "test-token-123"
	client, err := NewClient(WithToken(token))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.token != token {
		t.Errorf("expected token %s, got %s", token, client.token)
	}
}

func TestBuilderWithHorizonURL(t *testing.T) {
	url := "https://custom.horizon.org"
	client, err := NewClient(WithHorizonURL(url))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != url {
		t.Errorf("expected HorizonURL %s, got %s", url, client.HorizonURL)
	}
}

func TestBuilderWithInvalidURL(t *testing.T) {
	_, err := NewClient(WithHorizonURL("not a valid url"))
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestBuilderWithAltURLs(t *testing.T) {
	urls := []string{"https://url1.org", "https://url2.org"}
	client, err := NewClient(WithAltURLs(urls))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(client.AltURLs) != len(urls) {
		t.Errorf("expected %d AltURLs, got %d", len(urls), len(client.AltURLs))
	}
	if client.HorizonURL != urls[0] {
		t.Errorf("expected HorizonURL to be first URL, got %s", client.HorizonURL)
	}
}

func TestBuilderWithNetworkConfig(t *testing.T) {
	config := TestnetConfig
	client, err := NewClient(WithNetworkConfig(config))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Config.Name != config.Name {
		t.Errorf("expected config name %s, got %s", config.Name, client.Config.Name)
	}
	if client.HorizonURL != config.HorizonURL {
		t.Errorf("expected HorizonURL %s, got %s", config.HorizonURL, client.HorizonURL)
	}
}

func TestBuilderWithCacheEnabled(t *testing.T) {
	client, err := NewClient(WithCacheEnabled(false))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.CacheEnabled {
		t.Error("expected CacheEnabled to be false")
	}

	client, err = NewClient(WithCacheEnabled(true))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !client.CacheEnabled {
		t.Error("expected CacheEnabled to be true")
	}
}

func TestBuilderMultipleOptions(t *testing.T) {
	token := "my-token"
	client, err := NewClient(
		WithNetwork(Testnet),
		WithToken(token),
		WithCacheEnabled(false),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Testnet {
		t.Errorf("expected network Testnet, got %v", client.Network)
	}
	if client.token != token {
		t.Errorf("expected token %s, got %s", token, client.token)
	}
	if client.CacheEnabled {
		t.Error("expected CacheEnabled to be false")
	}
}

func TestDeprecatedNewClientDefault(t *testing.T) {
	client := NewClientDefault(Testnet, "")
	if client == nil {
		t.Fatal("expected non-nil client")
		return
	}
	if client.Network != Testnet {
		t.Errorf("expected network Testnet, got %v", client.Network)
	}
}

func TestDeprecatedNewClientWithURLOption(t *testing.T) {
	url := "https://custom.org"
	client := NewClientWithURLOption(url, Testnet, "")
	if client == nil {
		t.Fatal("expected non-nil client")
		return
	}
	if client.HorizonURL != url {
		t.Errorf("expected HorizonURL %s, got %s", url, client.HorizonURL)
	}
}

func TestDeprecatedNewClientWithURLsOption(t *testing.T) {
	urls := []string{"https://url1.org", "https://url2.org"}
	client := NewClientWithURLsOption(urls, Testnet, "")
	if client == nil {
		t.Fatal("expected non-nil client")
		return
	}
	if len(client.AltURLs) != len(urls) {
		t.Errorf("expected %d AltURLs, got %d", len(urls), len(client.AltURLs))
	}
}

func TestDeprecatedNewCustomClient(t *testing.T) {
	config := MainnetConfig
	client, err := NewCustomClient(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
		return
	}
	if client.Config.Name != config.Name {
		t.Errorf("expected config name %s, got %s", config.Name, client.Config.Name)
	}
}

func TestValidateNetworkConfig_Valid(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "https://custom.org",
		NetworkPassphrase: "custom network",
	}
	err := ValidateNetworkConfig(config)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateNetworkConfig_MissingName(t *testing.T) {
	config := NetworkConfig{
		HorizonURL:        "https://custom.org",
		NetworkPassphrase: "custom network",
	}
	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateNetworkConfig_InvalidURL(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "not a url",
		NetworkPassphrase: "custom network",
	}
	err := ValidateNetworkConfig(config)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
