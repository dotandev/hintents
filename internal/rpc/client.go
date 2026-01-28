// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"net/http"
	"os"

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

// authTransport is a custom HTTP RoundTripper that adds authentication headers
type authTransport struct {
	token     string
	transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		// Add Bearer token to Authorization header
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.transport.RoundTrip(req)
}

// Client handles interactions with the Stellar Network
type Client struct {
	Horizon horizonclient.ClientInterface
	Network Network
	token   string // stored for reference, not logged
}

// NewClient creates a new RPC client with the specified network
// If network is empty, defaults to Mainnet
// Token can be provided via the token parameter or ERST_RPC_TOKEN environment variable
func NewClient(net Network, token string) *Client {
	if net == "" {
		net = Mainnet
	}

	// Check environment variable if token not provided
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}

	var horizonClient *horizonclient.Client
	httpClient := createHTTPClient(token)

	switch net {
	case Testnet:
		horizonClient = &horizonclient.Client{
			HorizonURL: TestnetHorizonURL,
			HTTP:       httpClient,
		}
	case Futurenet:
		horizonClient = &horizonclient.Client{
			HorizonURL: FuturenetHorizonURL,
			HTTP:       httpClient,
		}
	case Mainnet:
		fallthrough
	default:
		horizonClient = &horizonclient.Client{
			HorizonURL: MainnetHorizonURL,
			HTTP:       httpClient,
		}
	}

	if token != "" {
		logger.Logger.Debug("RPC client initialized with authentication")
	} else {
		logger.Logger.Debug("RPC client initialized without authentication")
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
		token:   token,
	}
}

// NewClientWithURL creates a new RPC client with a custom Horizon URL
// Token can be provided via the token parameter or ERST_RPC_TOKEN environment variable
func NewClientWithURL(url string, net Network, token string) *Client {
	// Check environment variable if token not provided
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}

	httpClient := createHTTPClient(token)

	horizonClient := &horizonclient.Client{
		HorizonURL: url,
		HTTP:       httpClient,
	}

	if token != "" {
		logger.Logger.Debug("RPC client initialized with authentication")
	} else {
		logger.Logger.Debug("RPC client initialized without authentication")
	}

	return &Client{
		Horizon: horizonClient,
		Network: net,
		token:   token,
	}
}

// createHTTPClient creates an HTTP client with optional authentication
func createHTTPClient(token string) *http.Client {
	if token == "" {
		return http.DefaultClient
	}

	return &http.Client{
		Transport: &authTransport{
			token:     token,
			transport: http.DefaultTransport,
		},
	}
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

	return parseTransactionResponse(tx), nil
}
