// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"testing"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

func TestWithNetwork(t *testing.T) {
	client, err := NewClient(WithNetwork(Testnet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Testnet {
		t.Errorf("expected network Testnet, got %v", client.Network)
	}
}

func TestWithToken(t *testing.T) {
	token := "test-token"
	client, err := NewClient(WithToken(token))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.token != token {
		t.Errorf("expected token %s, got %s", token, client.token)
	}
}

func TestWithHorizonURL(t *testing.T) {
	url := "https://horizon-testnet.stellar.org/"
	client, err := NewClient(WithHorizonURL(url))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != url {
		t.Errorf("expected HorizonURL %s, got %s", url, client.HorizonURL)
	}
}

func TestWithInvalidHorizonURL(t *testing.T) {
	_, err := NewClient(WithHorizonURL("invalid-url"))
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestWithAltURLs(t *testing.T) {
	urls := []string{"https://horizon-testnet.stellar.org/", "https://horizon-futurenet.stellar.org/"}
	client, err := NewClient(WithAltURLs(urls))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(client.AltURLs) != len(urls) {
		t.Errorf("expected %d URLs, got %d", len(urls), len(client.AltURLs))
	}
	if client.HorizonURL != urls[0] {
		t.Errorf("expected first URL as HorizonURL, got %s", client.HorizonURL)
	}
}

func TestWithInvalidAltURL(t *testing.T) {
	_, err := NewClient(WithAltURLs([]string{"https://valid.org/", "invalid-url"}))
	if err == nil {
		t.Fatal("expected error for invalid URL in list")
	}
}

func TestWithSorobanURL(t *testing.T) {
	url := "https://soroban-testnet.stellar.org"
	client, err := NewClient(WithSorobanURL(url))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.SorobanURL != url {
		t.Errorf("expected SorobanURL %s, got %s", url, client.SorobanURL)
	}
}

func TestWithNetworkConfig(t *testing.T) {
	config := TestnetConfig
	client, err := NewClient(WithNetworkConfig(config))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Config.Name != config.Name {
		t.Errorf("expected network name %s, got %s", config.Name, client.Config.Name)
	}
	if client.SorobanURL != config.SorobanRPCURL {
		t.Errorf("expected SorobanURL %s, got %s", config.SorobanRPCURL, client.SorobanURL)
	}
}

func TestWithInvalidNetworkConfig(t *testing.T) {
	_, err := NewClient(WithNetworkConfig(NetworkConfig{Name: ""}))
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestWithCacheEnabled(t *testing.T) {
	client, err := NewClient(WithCacheEnabled(false))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.CacheEnabled {
		t.Errorf("expected CacheEnabled to be false")
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	client, err := NewClient(WithHTTPClient(customClient))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	hc, ok := client.Horizon.(*horizonclient.Client)
	if !ok {
		t.Fatal("expected *horizonclient.Client")
	}
	if hc.HTTP != customClient {
		t.Errorf("expected custom HTTP client to be used")
	}
}

func TestMultipleOptions(t *testing.T) {
	token := "test-token"
	client, err := NewClient(
		WithNetwork(Testnet),
		WithToken(token),
		WithHorizonURL(TestnetHorizonURL),
		WithSorobanURL(TestnetSorobanURL),
		WithCacheEnabled(false),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Testnet {
		t.Errorf("expected network Testnet")
	}
	if client.token != token {
		t.Errorf("expected token to be set")
	}
	if client.HorizonURL != TestnetHorizonURL {
		t.Errorf("expected HorizonURL to be set")
	}
	if client.SorobanURL != TestnetSorobanURL {
		t.Errorf("expected SorobanURL to be set")
	}
	if client.CacheEnabled {
		t.Errorf("expected CacheEnabled to be false")
	}
}

func TestDefaultNetwork(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Mainnet {
		t.Errorf("expected default network Mainnet, got %v", client.Network)
	}
}

func TestDefaultHorizonURL(t *testing.T) {
	client, err := NewClient(WithNetwork(Testnet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != TestnetHorizonURL {
		t.Errorf("expected default Testnet HorizonURL")
	}
}

func TestDefaultSorobanURL(t *testing.T) {
	client, err := NewClient(WithNetwork(Futurenet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.SorobanURL != FuturenetSorobanURL {
		t.Errorf("expected default Futurenet SorobanURL")
	}
}

func TestWithNetworkThenConfig(t *testing.T) {
	client, err := NewClient(
		WithNetwork(Testnet),
		WithNetworkConfig(MainnetConfig),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.Network != Mainnet {
		t.Errorf("expected config to override network")
	}
}

func TestCacheEnabledByDefault(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !client.CacheEnabled {
		t.Errorf("expected cache to be enabled by default")
	}
}

func TestMainnetDefaults(t *testing.T) {
	client, err := NewClient(WithNetwork(Mainnet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != MainnetHorizonURL {
		t.Errorf("expected Mainnet HorizonURL")
	}
	if client.SorobanURL != MainnetSorobanURL {
		t.Errorf("expected Mainnet SorobanURL")
	}
}

func TestTestnetDefaults(t *testing.T) {
	client, err := NewClient(WithNetwork(Testnet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != TestnetHorizonURL {
		t.Errorf("expected Testnet HorizonURL")
	}
	if client.SorobanURL != TestnetSorobanURL {
		t.Errorf("expected Testnet SorobanURL")
	}
}

func TestFuturenetDefaults(t *testing.T) {
	client, err := NewClient(WithNetwork(Futurenet))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.HorizonURL != FuturenetHorizonURL {
		t.Errorf("expected Futurenet HorizonURL")
	}
	if client.SorobanURL != FuturenetSorobanURL {
		t.Errorf("expected Futurenet SorobanURL")
	}
}

func TestAltURLsAsFailover(t *testing.T) {
	urls := []string{
		"https://horizon-testnet.stellar.org/",
		"https://horizon-futurenet.stellar.org/",
	}
	client, err := NewClient(WithAltURLs(urls))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	for i, url := range urls {
		if client.AltURLs[i] != url {
			t.Errorf("expected URL %d to be %s, got %s", i, url, client.AltURLs[i])
		}
	}
}

func TestDeprecatedNewClient(t *testing.T) {
	client := NewClientDefault(Testnet, "token")
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.Network != Testnet {
		t.Errorf("expected Testnet network")
	}
}

func TestDeprecatedNewClientWithURL(t *testing.T) {
	client := NewClientWithURLOption(TestnetHorizonURL, Testnet, "token")
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.HorizonURL != TestnetHorizonURL {
		t.Errorf("expected HorizonURL to match")
	}
}

func TestDeprecatedNewClientWithURLs(t *testing.T) {
	urls := []string{"https://horizon-testnet.stellar.org/", "https://horizon-futurenet.stellar.org/"}
	client := NewClientWithURLsOption(urls, Testnet, "token")
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if len(client.AltURLs) != len(urls) {
		t.Errorf("expected %d URLs", len(urls))
	}
}

func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewClient(WithNetwork(Testnet), WithToken("token"))
	}
}

func BenchmarkNewClientWithMultipleOptions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewClient(
			WithNetwork(Testnet),
			WithToken("token"),
			WithHorizonURL(TestnetHorizonURL),
			WithSorobanURL(TestnetSorobanURL),
			WithCacheEnabled(true),
		)
	}
}

func BenchmarkNewCustomClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewCustomClient(TestnetConfig)
	}
}
