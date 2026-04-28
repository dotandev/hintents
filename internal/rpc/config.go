// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"github.com/dotandev/hintents/internal/endpoints"
)

// Network types for Stellar
type Network string

const (
	Testnet   Network = "testnet"
	Mainnet   Network = "mainnet"
	Futurenet Network = "futurenet"
)

// Horizon URLs for each network
const (
	TestnetHorizonURL   = endpoints.HorizonTestnet
	MainnetHorizonURL   = endpoints.HorizonMainnet
	FuturenetHorizonURL = endpoints.HorizonFuturenet
)

// Soroban RPC URLs
const (
	TestnetSorobanURL   = endpoints.SorobanTestnet
	MainnetSorobanURL   = endpoints.ValidationCloudMainnet // Public demo endpoint
	FuturenetSorobanURL = endpoints.FuturenetAlt
)

// NetworkConfig represents a Stellar network configuration
type NetworkConfig struct {
	Name              string
	HorizonURL        string
	NetworkPassphrase string
	SorobanRPCURL     string
}

// Predefined network configurations
var (
	TestnetConfig = NetworkConfig{
		Name:              "testnet",
		HorizonURL:        TestnetHorizonURL,
		NetworkPassphrase: "Test SDF Network ; September 2015",
		SorobanRPCURL:     TestnetSorobanURL,
	}

	MainnetConfig = NetworkConfig{
		Name:              "mainnet",
		HorizonURL:        MainnetHorizonURL,
		NetworkPassphrase: "Public Global Stellar Network ; September 2015",
		SorobanRPCURL:     MainnetSorobanURL,
	}

	FuturenetConfig = NetworkConfig{
		Name:              "futurenet",
		HorizonURL:        FuturenetHorizonURL,
		NetworkPassphrase: "Test SDF Future Network ; October 2022",
		SorobanRPCURL:     FuturenetSorobanURL,
	}
)
