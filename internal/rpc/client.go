// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/logger"

	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"go.opentelemetry.io/otel/attribute"

	"github.com/dotandev/hintents/internal/errors"
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
	Horizon      horizonclient.ClientInterface
	HorizonURL   string
	Network      Network
	SorobanURL   string
	AltURLs      []string
	currIndex    int
	mu           sync.RWMutex
	token        string // stored for reference, not logged
	Config       NetworkConfig
	CacheEnabled bool
	failures     map[string]int
	lastFailure  map[string]time.Time
}

// NodeFailure records a failure for a specific RPC URL
type NodeFailure struct {
	URL    string
	Reason error
}

// AllNodesFailedError represents a failure after exhausting all RPC endpoints
type AllNodesFailedError struct {
	Failures []NodeFailure
}

func (e *AllNodesFailedError) Error() string {
	var reasons []string
	for _, f := range e.Failures {
		reasons = append(reasons, fmt.Sprintf("%s: %v", f.URL, f.Reason))
	}
	return fmt.Sprintf("all RPC endpoints failed: [%s]", strings.Join(reasons, ", "))
}

// isHealthy checks if an endpoint is currently healthy or if circuit is open
func (c *Client) isHealthy(url string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isHealthyLocked(url)
}

func (c *Client) isHealthyLocked(url string) bool {
	fails := c.failures[url]
	if fails < 5 {
		return true
	}
	last := c.lastFailure[url]
	// Circuit opens for 60 seconds
	if time.Since(last) > 60*time.Second {
		return true
	}
	return false
}

func (c *Client) markFailure(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	if c.lastFailure == nil {
		c.lastFailure = make(map[string]time.Time)
	}
	c.failures[url]++
	c.lastFailure[url] = time.Now()
}

func (c *Client) markSuccess(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	c.failures[url] = 0
}

// NewClientDefault creates a new RPC client with sensible defaults
// Uses the Mainnet by default and accepts optional environment token
// Deprecated: Use NewClient with functional options instead
func NewClientDefault(net Network, token string) *Client {
	client, err := NewClient(WithNetwork(net), WithToken(token))
	if err != nil {
		logger.Logger.Error("Failed to create client with default options", "error", err)
		return nil
	}
	return client
}

// NewClientWithURLOption creates a new RPC client with a custom Horizon URL
// Deprecated: Use NewClient with WithHorizonURL instead
func NewClientWithURLOption(url string, net Network, token string) *Client {
	client, err := NewClient(WithNetwork(net), WithToken(token), WithHorizonURL(url))
	if err != nil {
		logger.Logger.Error("Failed to create client with URL", "error", err)
		return nil
	}
	return client
}

// NewClientWithURLsOption creates a new RPC client with multiple Horizon URLs for failover
// Deprecated: Use NewClient with WithAltURLs instead
func NewClientWithURLsOption(urls []string, net Network, token string) *Client {
	client, err := NewClient(WithNetwork(net), WithToken(token), WithAltURLs(urls))
	if err != nil {
		logger.Logger.Error("Failed to create client with URLs", "error", err)
		return nil
	}
	return client
}

// rotateURL switches to the next available provider URL, skipping unhealthy ones if possible
func (c *Client) rotateURL() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.AltURLs) <= 1 {
		return false
	}

	// Try to find a healthy URL
	for i := 0; i < len(c.AltURLs); i++ {
		c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
		url := c.AltURLs[c.currIndex]
		if c.isHealthyLocked(url) {
			break
		}
		// If we've circled back to where we started, just take it
		if i == len(c.AltURLs)-1 {
			break
		}
	}

	c.HorizonURL = c.AltURLs[c.currIndex]
	c.Horizon = &horizonclient.Client{
		HorizonURL: c.HorizonURL,
		HTTP:       createHTTPClient(c.token),
	}

	logger.Logger.Warn("RPC failover triggered", "new_url", c.HorizonURL)
	return true
}

// createHTTPClient creates an HTTP client with optional authentication
func createHTTPClient(token string) *http.Client {
	cfg := DefaultRetryConfig()

	var baseTransport http.RoundTripper = http.DefaultTransport

	var transport http.RoundTripper = baseTransport
	if token != "" {
		transport = &authTransport{
			token:     token,
			transport: baseTransport,
		}
	}

	transport = NewRetryTransport(cfg, transport)

	return &http.Client{
		Transport: transport,
	}
}

// NewCustomClient creates a new RPC client for a custom/private network
// Deprecated: Use NewClient with WithNetworkConfig instead
func NewCustomClient(config NetworkConfig) (*Client, error) {
	if err := ValidateNetworkConfig(config); err != nil {
		return nil, err
	}

	horizonClient := &horizonclient.Client{
		HorizonURL: config.HorizonURL,
		HTTP:       http.DefaultClient,
	}

	sorobanURL := config.SorobanRPCURL
	if sorobanURL == "" {
		sorobanURL = config.HorizonURL
	}

	return &Client{
		Horizon:      horizonClient,
		Network:      "custom",
		SorobanURL:   sorobanURL,
		Config:       config,
		CacheEnabled: true,
	}, nil
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.getTransactionAttempt(ctx, hash)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			return resp, nil
		}

		c.markFailure(c.HorizonURL)

		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		// Only rotate if this isn't the last possible URL
		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) getTransactionAttempt(ctx context.Context, hash string) (*TransactionResponse, error) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_transaction")
	span.SetAttributes(
		attribute.String("transaction.hash", hash),
		attribute.String("network", string(c.Network)),
		attribute.String("rpc.url", c.HorizonURL),
	)
	defer span.End()

	logger.Logger.Debug("Fetching transaction details", "hash", hash, "url", c.HorizonURL)

	tx, err := c.Horizon.TransactionDetail(hash)
	if err != nil {
		span.RecordError(err)
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err, "url", c.HorizonURL)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	span.SetAttributes(
		attribute.Int("envelope.size_bytes", len(tx.EnvelopeXdr)),
		attribute.Int("result.size_bytes", len(tx.ResultXdr)),
		attribute.Int("result_meta.size_bytes", len(tx.ResultMetaXdr)),
	)

	logger.Logger.Info("Transaction fetched", "hash", hash, "envelope_size", len(tx.EnvelopeXdr), "url", c.HorizonURL)

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

// GetLedgerEntries fetches the current state of ledger entries from Soroban RPC with automatic failover
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
//
// GetLedgerHeader fetches ledger header details for a specific sequence with automatic fallback.
func (c *Client) GetLedgerHeader(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error) {
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.getLedgerHeaderAttempt(ctx, sequence)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			return resp, nil
		}

		c.markFailure(c.HorizonURL)

		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying ledger header fetch with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) getLedgerHeaderAttempt(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_ledger_header")
	span.SetAttributes(
		attribute.String("network", string(c.Network)),
		attribute.Int("ledger.sequence", int(sequence)),
		attribute.String("rpc.url", c.HorizonURL),
	)
	defer span.End()

	logger.Logger.Debug("Fetching ledger header", "sequence", sequence, "network", c.Network, "url", c.HorizonURL)

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
		"url", c.HorizonURL,
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
			return errors.WrapLedgerNotFound(sequence)
		case 410:
			logger.Logger.Warn("Ledger archived", "sequence", sequence, "status", 410)
			return errors.WrapLedgerArchived(sequence)
		case 429:
			logger.Logger.Warn("Rate limit exceeded", "sequence", sequence, "status", 429)
			return errors.WrapRateLimitExceeded()
		default:
			logger.Logger.Error("Horizon error", "sequence", sequence, "status", hErr.Problem.Status, "detail", hErr.Problem.Detail)
			return errors.WrapRPCError(c.HorizonURL, hErr.Problem.Detail, hErr.Problem.Status)
		}
	}

	// Generic error
	logger.Logger.Error("Failed to fetch ledger", "sequence", sequence, "error", err)
	return errors.WrapRPCConnectionFailed(err)
}

// IsLedgerNotFound checks if error is a "ledger not found" error
func IsLedgerNotFound(err error) bool {
	return errors.Is(err, errors.ErrLedgerNotFound)
}

// IsLedgerArchived checks if error is a "ledger archived" error
func IsLedgerArchived(err error) bool {
	return errors.Is(err, errors.ErrLedgerArchived)
}

// IsRateLimitError checks if error is a rate limit error
func IsRateLimitError(err error) bool {
	return errors.Is(err, errors.ErrRateLimitExceeded)
}

// GetLedgerEntries fetches the current state of ledger entries from Soroban RPC
// keys should be a list of base64-encoded XDR LedgerKeys
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	entries := make(map[string]string)
	var keysToFetch []string

	// Check cache if enabled
	if c.CacheEnabled {
		for _, key := range keys {
			val, hit, err := Get(key)
			if err != nil {
				logger.Logger.Warn("Cache read failed", "error", err)
			}
			if hit {
				entries[key] = val
				logger.Logger.Debug("Cache hit", "key", key)
			} else {
				keysToFetch = append(keysToFetch, key)
			}
		}
	} else {
		keysToFetch = keys
	}

	// If all keys found in cache, return immediately
	if len(keysToFetch) == 0 {
		logger.Logger.Info("All ledger entries found in cache", "count", len(keys))
		return entries, nil
	}

	logger.Logger.Debug("Fetching ledger entries from RPC", "count", len(keysToFetch), "url", c.SorobanURL)
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		res, err := c.getLedgerEntriesAttempt(ctx, keysToFetch)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			// Merge with cached results
			for k, v := range res {
				entries[k] = v
			}
			return entries, nil
		}

		c.markFailure(c.HorizonURL)
		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying with fallback Soroban RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) getLedgerEntriesAttempt(ctx context.Context, keysToFetch []string) (map[string]string, error) {
	logger.Logger.Debug("Fetching ledger entries", "count", len(keysToFetch), "url", c.HorizonURL)
	reqBody := GetLedgerEntriesRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getLedgerEntries",
		Params:  []interface{}{keysToFetch},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.WrapMarshalFailed(err)
	}

	targetURL := c.HorizonURL
	if c.Network == Testnet && targetURL == "" {
		targetURL = TestnetSorobanURL
	} else if c.Network == Mainnet && targetURL == "" {
		targetURL = MainnetSorobanURL
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp GetLedgerEntriesResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		return nil, errors.WrapRPCError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	entries := make(map[string]string)
	fetchedCount := 0
	for _, entry := range rpcResp.Result.Entries {
		entries[entry.Key] = entry.Xdr
		fetchedCount++

		// Cache the new entry
		if c.CacheEnabled {
			if err := Set(entry.Key, entry.Xdr); err != nil {
				logger.Logger.Warn("Failed to cache entry", "key", entry.Key, "error", err)
			}
		}
	}

	logger.Logger.Info("Ledger entries fetched",
		// 		"total_requested", len(keys),
		"total_requested", len(keysToFetch),
		"from_cache", len(keysToFetch)-fetchedCount,
		"from_rpc", fetchedCount,
		"url", targetURL,
	)

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
		return nil, errors.WrapRPCConnectionFailed(err)
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
		return "success"
	}
	return "failed"
}

type SimulateTransactionRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type SimulateTransactionResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		// Soroban RPC returns these in various versions. Keep fields optional.
		// We only need minimal pieces for fee/budget estimation.
		MinResourceFee  string `json:"minResourceFee,omitempty"`
		TransactionData string `json:"transactionData,omitempty"`
		Cost            struct {
			CpuInsns  int64 `json:"cpuInsns,omitempty"`
			MemBytes  int64 `json:"memBytes,omitempty"`
			CpuInsns_ int64 `json:"cpu_insns,omitempty"`
			MemBytes_ int64 `json:"mem_bytes,omitempty"`
		} `json:"cost,omitempty"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// SimulateTransaction calls Soroban RPC simulateTransaction using a base64 TransactionEnvelope XDR.
func (c *Client) SimulateTransaction(ctx context.Context, envelopeXdr string) (*SimulateTransactionResponse, error) {
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.simulateTransactionAttempt(ctx, envelopeXdr)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			return resp, nil
		}

		c.markFailure(c.HorizonURL)

		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying transaction simulation with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) simulateTransactionAttempt(ctx context.Context, envelopeXdr string) (*SimulateTransactionResponse, error) {
	logger.Logger.Debug("Simulating transaction (preflight)", "url", c.HorizonURL)

	reqBody := SimulateTransactionRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "simulateTransaction",
		Params:  []interface{}{envelopeXdr},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.WrapMarshalFailed(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.HorizonURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp SimulateTransactionResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		return nil, errors.WrapRPCError(c.HorizonURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return &rpcResp, nil
}
