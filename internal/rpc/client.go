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
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/logger"

	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
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
	HorizonURL   string
	Horizon      horizonclient.ClientInterface
	Network      Network
	SorobanURL   string
	AltURLs      []string
	mu           sync.RWMutex
	currIndex    int
	token        string
	Config       NetworkConfig
	CacheEnabled bool
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

// rotateURL switches to the next available provider URL
func (c *Client) rotateURL() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.AltURLs) <= 1 {
		return false
	}

	c.currIndex = (c.currIndex + 1) % len(c.AltURLs)
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
	// Set a timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// Backwards-compatible: tests and some callers may construct a Client
	// without AltURLs configured. In that case, attempt once using the
	// configured Horizon client/URL.
	if len(c.AltURLs) == 0 {
		resp, err := c.getTransactionAttempt(ctx, hash)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	var lastErr error

	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.getTransactionAttempt(ctx, hash)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Only rotate if this isn't the last possible URL
		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}
		return nil, fmt.Errorf("all RPC endpoints failed: %w", err)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("all RPC endpoints failed: %w", lastErr)
	}
	return nil, fmt.Errorf("all RPC endpoints failed")
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
		return nil, fmt.Errorf("failed to fetch transaction from %s: %w", c.HorizonURL, err)
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

	// Set a timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
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

	const batchSize = 50
	const maxConcurrentBatches = 4

	chunks := chunkStrings(keysToFetch, batchSize)
	logger.Logger.Debug("Fetching ledger entries from RPC (batched)", "total", len(keysToFetch), "batches", len(chunks), "url", c.SorobanURL)

	sem := make(chan struct{}, maxConcurrentBatches)
	errCh := make(chan error, len(chunks))
	resCh := make(chan map[string]string, len(chunks))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, chunk := range chunks {
		chunk := chunk
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			batchEntries, err := c.getLedgerEntriesAttempt(ctx, chunk)
			if err != nil {
				errCh <- err
				cancel()
				return
			}
			resCh <- batchEntries
		}()
	}

	wg.Wait()
	close(errCh)
	close(resCh)

	if err := firstErr(errCh); err != nil {
		return nil, err
	}

	for m := range resCh {
		for k, v := range m {
			entries[k] = v
		}
	}

	return entries, nil
}

func (c *Client) getLedgerEntriesAttempt(ctx context.Context, keysToFetch []string) (map[string]string, error) {
	logger.Logger.Debug("Fetching ledger entries", "count", len(keysToFetch), "url", c.SorobanURL)
	reqBody := GetLedgerEntriesRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getLedgerEntries",
		Params:  []interface{}{keysToFetch},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	targetURL := c.SorobanURL
	if targetURL == "" {
		switch c.Network {
		case Testnet:
			targetURL = TestnetSorobanURL
		case Futurenet:
			targetURL = FuturenetSorobanURL
		default:
			targetURL = MainnetSorobanURL
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := createHTTPClient(c.token).Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request to %s: %w", targetURL, err)
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
		return nil, fmt.Errorf("rpc error from %s: %s (code %d)", targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	entries := make(map[string]string)
	for _, entry := range rpcResp.Result.Entries {
		entries[entry.Key] = entry.Xdr

		// Cache the new entry
		if c.CacheEnabled {
			if err := Set(entry.Key, entry.Xdr); err != nil {
				logger.Logger.Warn("Failed to cache entry", "key", entry.Key, "error", err)
			}
		}
	}

	return entries, nil
}

func chunkStrings(in []string, size int) [][]string {
	if size <= 0 {
		size = 1
	}
	if len(in) == 0 {
		return nil
	}

	out := make([][]string, 0, (len(in)+size-1)/size)
	for i := 0; i < len(in); i += size {
		end := i + size
		if end > len(in) {
			end = len(in)
		}
		out = append(out, in[i:end])
	}
	return out
}

func firstErr(errCh <-chan error) error {
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
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
// This is used for pre-submission "dry-run" cost estimation.
func (c *Client) SimulateTransaction(ctx context.Context, envelopeXdr string) (*SimulateTransactionResponse, error) {
	logger.Logger.Debug("Simulating transaction (preflight)", "url", c.SorobanURL)

	reqBody := SimulateTransactionRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "simulateTransaction",
		Params:  []interface{}{envelopeXdr},
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

	var rpcResp SimulateTransactionResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return &rpcResp, nil
}
