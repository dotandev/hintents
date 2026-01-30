// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"go.opentelemetry.io/otel/attribute"
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

// Soroban RPC URLs
const (
	TestnetSorobanURL   = "https://soroban-testnet.stellar.org"
	MainnetSorobanURL   = "https://mainnet.stellar.validationcloud.io/v1/soroban-rpc-demo" // Public demo endpoint
	FuturenetSorobanURL = "https://rpc-futurenet.stellar.org"
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

// Client handles interactions with the Stellar Network
type Client struct {
	Horizon    horizonclient.ClientInterface
	Network    Network
	SorobanURL string
	token      string // stored for reference, not logged
	Config     NetworkConfig
}

// TransactionResponse contains the raw XDR fields needed for simulation
type TransactionResponse struct {
	EnvelopeXdr   string
	ResultXdr     string
	ResultMetaXdr string
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
	var sorobanURL string
	httpClient := createHTTPClient(token)
	var config NetworkConfig

	switch net {
	case Testnet:
		horizonClient = &horizonclient.Client{
			HorizonURL: TestnetHorizonURL,
			HTTP:       httpClient,
		}
		sorobanURL = TestnetSorobanURL
		config = TestnetConfig
	case Futurenet:
		horizonClient = &horizonclient.Client{
			HorizonURL: FuturenetHorizonURL,
			HTTP:       httpClient,
		}
		sorobanURL = FuturenetSorobanURL
		config = FuturenetConfig
	case Mainnet:
		fallthrough
	default:
		horizonClient = &horizonclient.Client{
			HorizonURL: MainnetHorizonURL,
			HTTP:       httpClient,
		}
		sorobanURL = MainnetSorobanURL
		config = MainnetConfig
	}

	if token != "" {
		logger.Logger.Debug("RPC client initialized with authentication")
	} else {
		logger.Logger.Debug("RPC client initialized without authentication")
	}

	return &Client{
		Horizon:    horizonClient,
		Network:    net,
		SorobanURL: sorobanURL,
		token:      token,
		Config:     config,
	}
}

// NewClientWithURL creates a new RPC client with a custom Horizon URL
// Token can be provided via the token parameter or ERST_RPC_TOKEN environment variable
func NewClientWithURL(url string, net Network, token string) *Client {
	// Check environment variable if token not provided
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}

	// Re-use logic to get default Soroban URL
	defaultClient := NewClient(net, token)

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
		Horizon:    horizonClient,
		Network:    net,
		SorobanURL: defaultClient.SorobanURL,
		token:      token,
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

// NewCustomClient creates a new RPC client for a custom/private network
func NewCustomClient(config NetworkConfig) (*Client, error) {
	if config.HorizonURL == "" {
		return nil, fmt.Errorf("horizon URL is required for custom network")
	}
	if config.NetworkPassphrase == "" {
		return nil, fmt.Errorf("network passphrase is required for custom network")
	}

	horizonClient := &horizonclient.Client{
		HorizonURL: config.HorizonURL,
		HTTP:       http.DefaultClient,
	}

	sorobanURL := config.SorobanRPCURL
	if sorobanURL == "" {
		sorobanURL = config.HorizonURL // Fallback to Horizon URL if no Soroban RPC specified
	}

	return &Client{
		Horizon:    horizonClient,
		Network:    "custom",
		SorobanURL: sorobanURL,
		Config:     config,
	}, nil
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_transaction")
	span.SetAttributes(
		attribute.String("transaction.hash", hash),
		attribute.String("network", string(c.Network)),
	)
	defer span.End()

	logger.Logger.Debug("Fetching transaction details", "hash", hash)

	tx, err := c.Horizon.TransactionDetail(hash)
	if err != nil {
		span.RecordError(err)
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err)
		return nil, fmt.Errorf("failed to fetch transaction: %w", err)
	}

	span.SetAttributes(
		attribute.Int("envelope.size_bytes", len(tx.EnvelopeXdr)),
		attribute.Int("result.size_bytes", len(tx.ResultXdr)),
		attribute.Int("result_meta.size_bytes", len(tx.ResultMetaXdr)),
	)

	logger.Logger.Info("Transaction fetched successfully", "hash", hash, "envelope_size", len(tx.EnvelopeXdr))

	return ParseTransactionResponse(tx), nil

}

// GetNetworkPassphrase returns the network passphrase for this client
func (c *Client) GetNetworkPassphrase() string {
	return c.Config.NetworkPassphrase
}

// GetNetworkName returns the network name for this client
func (c *Client) GetNetworkName() string {
	if c.Config.Name != "" {
		return c.Config.Name
	}
	return "custom"
}

type GetLedgerEntriesRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type GetLedgerEntriesResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Entries []struct {
			Key                string `json:"key"`
			Xdr                string `json:"xdr"`
			LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
			LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
		} `json:"entries"`
		LatestLedger int `json:"latestLedger"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetLedgerHeader fetches ledger header details for a specific sequence.
// This includes essential metadata like sequence number, timestamp, protocol version,
// and XDR-encoded header data needed for transaction simulation.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - sequence: The ledger sequence number to fetch
//
// Returns:
//   - *LedgerHeaderResponse: Header data if successful
//   - error: Typed error indicating failure reason:
//   - LedgerNotFoundError: Ledger doesn't exist (future or invalid)
//   - LedgerArchivedError: Ledger has been archived
//   - RateLimitError: Too many requests
//
// Example:
//
//	header, err := client.GetLedgerHeader(ctx, 12345678)
//	if IsLedgerNotFound(err) {
//	    log.Printf("Ledger not found: %v", err)
//	}
func (c *Client) GetLedgerHeader(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_ledger_header")
	span.SetAttributes(
		attribute.String("network", string(c.Network)),
		attribute.Int("ledger.sequence", int(sequence)),
	)
	defer span.End()

	// Set a timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	logger.Logger.Debug("Fetching ledger header", "sequence", sequence, "network", c.Network)

	// Fetch ledger from Horizon
	ledger, err := c.Horizon.LedgerDetail(sequence)
	if err != nil {
		span.RecordError(err)
		return nil, c.handleLedgerError(err, sequence)
	}

	response := FromHorizonLedger(ledger)

	span.SetAttributes(
		attribute.String("ledger.hash", response.Hash),
		attribute.Int("ledger.protocol_version", int(response.ProtocolVersion)),
		attribute.Int("ledger.tx_count", int(response.SuccessfulTxCount+response.FailedTxCount)),
	)

	logger.Logger.Info("Ledger header fetched successfully",
		"sequence", sequence,
		"hash", response.Hash,
		"protocol_version", response.ProtocolVersion,
		"close_time", response.CloseTime,
	)

	return response, nil
}

// handleLedgerError provides detailed error messages for ledger fetch failures
func (c *Client) handleLedgerError(err error, sequence uint32) error {
	// Check if it's a Horizon error
	if hErr, ok := err.(*horizonclient.Error); ok {
		switch hErr.Problem.Status {
		case 404:
			logger.Logger.Warn("Ledger not found", "sequence", sequence, "status", 404)
			return &LedgerNotFoundError{
				Sequence: sequence,
				Message:  fmt.Sprintf("ledger %d not found (may be archived or not yet created)", sequence),
			}
		case 410:
			logger.Logger.Warn("Ledger archived", "sequence", sequence, "status", 410)
			return &LedgerArchivedError{
				Sequence: sequence,
				Message:  fmt.Sprintf("ledger %d has been archived and is no longer available", sequence),
			}
		case 429:
			logger.Logger.Warn("Rate limit exceeded", "sequence", sequence, "status", 429)
			return &RateLimitError{
				Message: "rate limit exceeded, please try again later",
			}
		default:
			logger.Logger.Error("Horizon error", "sequence", sequence, "status", hErr.Problem.Status, "detail", hErr.Problem.Detail)
			return fmt.Errorf("horizon error (status %d): %v", hErr.Problem.Status, hErr.Problem.Detail)
		}
	}

	// Generic error
	logger.Logger.Error("Failed to fetch ledger", "sequence", sequence, "error", err)
	return fmt.Errorf("failed to fetch ledger %d: %w", sequence, err)
}

// LedgerNotFoundError indicates that the requested ledger doesn't exist.
// This can happen if the ledger sequence is in the future or invalid.
type LedgerNotFoundError struct {
	Sequence uint32
	Message  string
}

func (e *LedgerNotFoundError) Error() string {
	return e.Message
}

// LedgerArchivedError indicates that the requested ledger has been archived
// and is no longer available through the Horizon API.
type LedgerArchivedError struct {
	Sequence uint32
	Message  string
}

func (e *LedgerArchivedError) Error() string {
	return e.Message
}

// RateLimitError indicates that too many requests have been made
// and the client should back off.
type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// IsLedgerNotFound checks if error is a "ledger not found" error
func IsLedgerNotFound(err error) bool {
	_, ok := err.(*LedgerNotFoundError)
	return ok
}

// IsLedgerArchived checks if error is a "ledger archived" error
func IsLedgerArchived(err error) bool {
	_, ok := err.(*LedgerArchivedError)
	return ok
}

// IsRateLimitError checks if error is a rate limit error
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// GetLedgerEntries fetches the current state of ledger entries from Soroban RPC
// keys should be a list of base64-encoded XDR LedgerKeys
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	logger.Logger.Debug("Fetching ledger entries", "count", len(keys), "url", c.SorobanURL)

	reqBody := GetLedgerEntriesRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getLedgerEntries",
		Params:  []interface{}{keys},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.SorobanURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp GetLedgerEntriesResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	entries := make(map[string]string)
	for _, entry := range rpcResp.Result.Entries {
		entries[entry.Key] = entry.Xdr
	}

	logger.Logger.Info("Ledger entries fetched successfully", "found", len(entries), "requested", len(keys))

	return entries, nil
}

type TransactionSummary struct {
	Hash      string
	Status    string
	CreatedAt string
}

func (c *Client) GetAccountTransactions(ctx context.Context, account string, limit int) ([]TransactionSummary, error) {
	logger.Logger.Debug("Fetching account transactions", "account", account)

	req := horizonclient.TransactionRequest{
		ForAccount: account,
		Limit:      uint(limit),
		Order:      horizonclient.OrderDesc,
	}

	page, err := c.Horizon.Transactions(req)
	if err != nil {
		logger.Logger.Error("Failed to fetch account transactions", "account", account, "error", err)
		return nil, fmt.Errorf("failed to fetch account transactions: %w", err)
	}

	summaries := make([]TransactionSummary, 0, len(page.Embedded.Records))
	for _, tx := range page.Embedded.Records {
		summaries = append(summaries, TransactionSummary{
			Hash:      tx.Hash,
			Status:    getTransactionStatus(tx),
			CreatedAt: tx.LedgerCloseTime.Format("2006-01-02 15:04:05"),
		})
	}

	logger.Logger.Debug("Account transactions retrieved", "count", len(summaries))
	return summaries, nil
}

func getTransactionStatus(tx hProtocol.Transaction) string {
	if tx.Successful {
		return "✓ success"
	}
	return "✗ failed"
}
