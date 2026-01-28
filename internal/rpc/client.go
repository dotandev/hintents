// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go/clients/horizonclient"
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
	TestnetHorizonURL   = "https://horizon-testnet.stellar.org/"
	MainnetHorizonURL   = "https://horizon.stellar.org/"
	FuturenetHorizonURL = "https://horizon-futurenet.stellar.org/"
)

// Client handles interactions with the Stellar Network
type Client struct {
	Horizon horizonclient.ClientInterface
	Network Network
}

// NewClient creates a new RPC client with the specified network
// If network is empty, defaults to Mainnet
func NewClient(net Network) *Client {
	if net == "" {
		net = Mainnet
	}

	var horizonClient *horizonclient.Client

	switch net {
	case Testnet:
		horizonClient = horizonclient.DefaultTestNetClient
	case Futurenet:
		// Create a futurenet client (not available as default)
		horizonClient = &horizonclient.Client{
			HorizonURL: FuturenetHorizonURL,
			HTTP:       http.DefaultClient,
		}
	case Mainnet:
		fallthrough
	default:
		horizonClient = horizonclient.DefaultPublicNetClient
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
	}
}

// NewClientWithURL creates a new RPC client with a custom Horizon URL
func NewClientWithURL(url string, net Network) *Client {
	horizonClient := &horizonclient.Client{
		HorizonURL: url,
		HTTP:       http.DefaultClient,
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
	}
}

// TransactionResponse contains the raw XDR fields needed for simulation
type TransactionResponse struct {
	EnvelopeXdr   string
	ResultXdr     string
	ResultMetaXdr string
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	logger.Logger.Debug("Fetching transaction details", "hash", hash)

	tx, err := c.Horizon.TransactionDetail(hash)
	if err != nil {
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err)
		return nil, errors.WrapTransactionNotFound(err)
	}

	logger.Logger.Info("Transaction fetched successfully", "hash", hash, "envelope_size", len(tx.EnvelopeXdr))

	return &TransactionResponse{
		EnvelopeXdr:   tx.EnvelopeXdr,
		ResultXdr:     tx.ResultXdr,
		ResultMetaXdr: tx.ResultMetaXdr,
	}, nil
}
