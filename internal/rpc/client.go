// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	stdErrors "errors"
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
	effects "github.com/stellar/go-stellar-sdk/protocols/horizon/effects"
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
	Horizon         horizonclient.ClientInterface
	HorizonURL      string
	Network         Network
	SorobanURL      string
	AltURLs         []string
	currIndex       int
	mu              sync.RWMutex
	httpClient      *http.Client
	token           string // stored for reference, not logged
	Config          NetworkConfig
	CacheEnabled    bool
	methodTelemetry MethodTelemetry
	failures        map[string]int
	lastFailure     map[string]time.Time
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

// isHealthy checks if an endpoint is currently healthy or if circuit is open.
// This is a best-effort check — there is an intentional TOCTOU window between
// this call and the subsequent http.Do; no lock is held across both operations
// because doing so would risk deadlocks with rotateURL. The circuit breaker is
// an optimistic fast-path, not a hard guarantee.
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
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = createHTTPClient(c.token, defaultHTTPTimeout)
	}
	c.Horizon = &horizonclient.Client{
		HorizonURL: c.HorizonURL,
		HTTP:       httpClient,
	}
	c.SorobanURL = c.AltURLs[c.currIndex]

	logger.Logger.Warn("RPC failover triggered", "new_url", c.HorizonURL)
	return true
}

func (c *Client) getHTTPClient() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *Client) startMethodTimer(ctx context.Context, method string, attributes map[string]string) MethodTimer {
	if c == nil || c.methodTelemetry == nil {
		return noopMethodTimer{}
	}
	return c.methodTelemetry.StartMethodTimer(ctx, method, attributes)
}

// createHTTPClient creates an HTTP client with optional authentication and a configurable timeout.
func createHTTPClient(token string, timeout time.Duration) *http.Client {
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
		Timeout:   timeout,
	}
}

// NewCustomClient creates a new RPC client for a custom/private network
// Deprecated: Use NewClient with WithNetworkConfig instead
func NewCustomClient(config NetworkConfig) (*Client, error) {
	if err := ValidateNetworkConfig(config); err != nil {
		return nil, err
	}

	httpClient := createHTTPClient("", defaultHTTPTimeout)
	horizonClient := &horizonclient.Client{
		HorizonURL: config.HorizonURL,
		HTTP:       httpClient,
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
		httpClient:   httpClient,
	}, nil
}

type GetHealthRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
}

type GetHealthResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Status                string `json:"status"`
		LatestLedger          uint32 `json:"latestLedger"`
		OldestLedger          uint32 `json:"oldestLedger"`
		LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	if len(c.AltURLs) == 0 {
		return nil, &AllNodesFailedError{}
	}
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

func (c *Client) getTransactionAttempt(ctx context.Context, hash string) (txResp *TransactionResponse, err error) {
	timer := c.startMethodTimer(ctx, "rpc.get_transaction", map[string]string{
		"network": c.GetNetworkName(),
		"rpc_url": c.HorizonURL,
	})
	defer func() {
		timer.Stop(err)
	}()

	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_transaction")
	span.SetAttributes(
		attribute.String("transaction.hash", hash),
		attribute.String("network", string(c.Network)),
		attribute.String("rpc.url", c.HorizonURL),
	)
	defer span.End()

	logger.Logger.Debug("Fetching transaction details", "hash", hash, "url", c.HorizonURL)

	// Fail fast if circuit breaker is open for this Horizon endpoint.
	if !c.isHealthy(c.HorizonURL) {
		err := fmt.Errorf("circuit breaker open for %s", c.HorizonURL)
		span.RecordError(err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

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
		Entries      []LedgerEntryResult `json:"entries"`
		LatestLedger int                 `json:"latestLedger"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type LedgerEntryResult struct {
	Key                string `json:"key"`
	Xdr                string `json:"xdr"`
	LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
	LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
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
//
// GetLedgerHeader fetches ledger header details for a specific sequence with automatic fallback.
func (c *Client) GetLedgerHeader(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error) {
	if len(c.AltURLs) == 0 {
		resp, err := c.getLedgerHeaderAttempt(ctx, sequence)
		if err != nil {
			c.markFailure(c.HorizonURL)
			return nil, err
		}
		c.markSuccess(c.HorizonURL)
		return resp, nil
	}
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

func (c *Client) getLedgerHeaderAttempt(ctx context.Context, sequence uint32) (ledgerResp *LedgerHeaderResponse, err error) {
	timer := c.startMethodTimer(ctx, "rpc.get_ledger_header", map[string]string{
		"network": c.GetNetworkName(),
		"rpc_url": c.HorizonURL,
	})
	defer func() {
		timer.Stop(err)
	}()

	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_ledger_header")
	span.SetAttributes(
		attribute.String("network", string(c.Network)),
		attribute.Int("ledger.sequence", int(sequence)),
		attribute.String("rpc.url", c.HorizonURL),
	)
	defer span.End()

	logger.Logger.Debug("Fetching ledger header", "sequence", sequence, "network", c.Network, "url", c.HorizonURL)

	// Fail fast if circuit breaker is open for this Horizon endpoint.
	if !c.isHealthy(c.HorizonURL) {
		err := fmt.Errorf("circuit breaker open for %s", c.HorizonURL)
		span.RecordError(err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

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
		case 413:
			logger.Logger.Warn("Response too large", "sequence", sequence, "status", 413)
			return errors.WrapRPCResponseTooLarge(c.HorizonURL)
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
	if err == nil {
		return false
	}
	if errors.Is(err, errors.ErrLedgerNotFound) {
		return true
	}
	return ledgerFailureContains(err, IsLedgerNotFound)
}

func ledgerFailureContains(err error, checker func(error) bool) bool {
	var allErr *AllNodesFailedError
	if !stdErrors.As(err, &allErr) {
		return false
	}
	for _, failure := range allErr.Failures {
		if checker(failure.Reason) {
			return true
		}
	}
	return false
}

// IsLedgerArchived checks if error is a "ledger archived" error
func IsLedgerArchived(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errors.ErrLedgerArchived) {
		return true
	}
	return ledgerFailureContains(err, IsLedgerArchived)
}

// IsRateLimitError checks if error is a rate limit error
func IsRateLimitError(err error) bool {
	return errors.Is(err, errors.ErrRateLimitExceeded)
}

// IsResponseTooLarge checks if error indicates the RPC response exceeded size limits
func IsResponseTooLarge(err error) bool {
	return errors.Is(err, errors.ErrRPCResponseTooLarge)
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

	if len(c.AltURLs) == 0 {
		return nil, &AllNodesFailedError{}
	}

	logger.Logger.Debug("Fetching ledger entries from RPC", "count", len(keysToFetch), "url", c.SorobanURL)
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		res, err := c.getLedgerEntriesAttempt(ctx, keysToFetch)
		if err == nil {
			c.markSuccess(c.SorobanURL)
			// Merge with cached results
			for k, v := range res {
				entries[k] = v
			}
			return entries, nil
		}

		c.markFailure(c.SorobanURL)
		failures = append(failures, NodeFailure{URL: c.SorobanURL, Reason: err})

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

func (c *Client) getLedgerEntriesAttempt(ctx context.Context, keysToFetch []string) (entries map[string]string, err error) {
	// Always use the dedicated Soroban RPC URL for getLedgerEntries; this is a
	// Soroban JSON-RPC method and is not served by the Horizon REST API.
	targetURL := c.SorobanURL
	if targetURL == "" {
		switch c.Network {
		case Testnet:
			targetURL = TestnetSorobanURL
		case Mainnet:
			targetURL = MainnetSorobanURL
		case Futurenet:
			targetURL = FuturenetSorobanURL
		}
	}

	timer := c.startMethodTimer(ctx, "rpc.get_ledger_entries", map[string]string{
		"network": c.GetNetworkName(),
		"rpc_url": targetURL,
	})
	defer func() {
		timer.Stop(err)
	}()

	logger.Logger.Debug("Fetching ledger entries", "count", len(keysToFetch), "url", targetURL)

	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		return nil, errors.WrapRPCConnectionFailed(
			fmt.Errorf("circuit breaker open for %s", targetURL),
		)
	}

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

	// Validate payload size before attempting to send to network
	if err := ValidatePayloadSize(int64(len(bodyBytes))); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

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

	entries = make(map[string]string)
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

	// Cryptographically verify all returned ledger entries
	if err := VerifyLedgerEntries(keysToFetch, entries); err != nil {
		return nil, fmt.Errorf("ledger entry verification failed: %w", err)
	}

	logger.Logger.Info("Ledger entries fetched",
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

type AccountSummary struct {
	ID            string
	Sequence      int64
	SubentryCount int32
}

type EventSummary struct {
	ID   string
	Type string
}

func (c *Client) GetAccountTransactions(ctx context.Context, account string, limit int) ([]TransactionSummary, error) {
	logger.Logger.Debug("Fetching account transactions", "account", account)

	pageSize := normalizePageSize(limit)
	req := horizonclient.TransactionRequest{
		ForAccount: account,
		Limit:      uint(pageSize),
		Order:      horizonclient.OrderDesc,
	}

	transactions, err := pageIterator[hProtocol.TransactionsPage, hProtocol.Transaction]{
		first: func() (hProtocol.TransactionsPage, error) {
			return c.Horizon.Transactions(req)
		},
		next: func(page hProtocol.TransactionsPage) (hProtocol.TransactionsPage, error) {
			return c.Horizon.NextTransactionsPage(page)
		},
		records: func(page hProtocol.TransactionsPage) []hProtocol.Transaction {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch account transactions", "account", account, "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	summaries := make([]TransactionSummary, 0, len(transactions))
	for _, tx := range transactions {
		summaries = append(summaries, TransactionSummary{
			Hash:      tx.Hash,
			Status:    getTransactionStatus(tx),
			CreatedAt: tx.LedgerCloseTime.Format("2006-01-02 15:04:05"),
		})
	}

	logger.Logger.Debug("Account transactions retrieved", "count", len(summaries))
	return summaries, nil
}

// GetEventsForAccount fetches effects (treated as events) for an account using shared page iteration.
func (c *Client) GetEventsForAccount(ctx context.Context, account string, limit int) ([]EventSummary, error) {
	logger.Logger.Debug("Fetching account events", "account", account)

	pageSize := normalizePageSize(limit)
	req := horizonclient.EffectRequest{
		ForAccount: account,
		Limit:      uint(pageSize),
		Order:      horizonclient.OrderDesc,
	}

	eventRecords, err := pageIterator[effects.EffectsPage, effects.Effect]{
		first: func() (effects.EffectsPage, error) {
			return c.Horizon.Effects(req)
		},
		next: func(page effects.EffectsPage) (effects.EffectsPage, error) {
			return c.Horizon.NextEffectsPage(page)
		},
		records: func(page effects.EffectsPage) []effects.Effect {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch account events", "account", account, "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	out := make([]EventSummary, 0, len(eventRecords))
	for _, evt := range eventRecords {
		out = append(out, EventSummary{
			ID:   evt.GetID(),
			Type: evt.GetType(),
		})
	}

	logger.Logger.Debug("Account events retrieved", "count", len(out))
	return out, nil
}

// GetAccounts fetches account records using shared page iteration.
func (c *Client) GetAccounts(ctx context.Context, limit int) ([]AccountSummary, error) {
	logger.Logger.Debug("Fetching accounts")

	pageSize := normalizePageSize(limit)
	req := horizonclient.AccountsRequest{
		Limit: uint(pageSize),
		Order: horizonclient.OrderDesc,
	}

	accountRecords, err := pageIterator[hProtocol.AccountsPage, hProtocol.Account]{
		first: func() (hProtocol.AccountsPage, error) {
			return c.Horizon.Accounts(req)
		},
		next: func(page hProtocol.AccountsPage) (hProtocol.AccountsPage, error) {
			return c.Horizon.NextAccountsPage(page)
		},
		records: func(page hProtocol.AccountsPage) []hProtocol.Account {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch accounts", "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	out := make([]AccountSummary, 0, len(accountRecords))
	for _, acc := range accountRecords {
		out = append(out, AccountSummary{
			ID:            acc.AccountID,
			Sequence:      acc.Sequence,
			SubentryCount: acc.SubentryCount,
		})
	}

	logger.Logger.Debug("Accounts retrieved", "count", len(out))
	return out, nil
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
	if len(c.AltURLs) == 0 {
		return nil, &AllNodesFailedError{}
	}
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.simulateTransactionAttempt(ctx, envelopeXdr)
		if err == nil {
			c.markSuccess(c.SorobanURL)
			return resp, nil
		}

		c.markFailure(c.SorobanURL)

		failures = append(failures, NodeFailure{URL: c.SorobanURL, Reason: err})

		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying transaction simulation with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) simulateTransactionAttempt(ctx context.Context, envelopeXdr string) (simResp *SimulateTransactionResponse, err error) {
	// Always use the dedicated Soroban RPC URL for simulateTransaction; this is a
	// Soroban JSON-RPC method and is not served by the Horizon REST API.
	targetURL := c.SorobanURL
	if targetURL == "" {
		switch c.Network {
		case Testnet:
			targetURL = TestnetSorobanURL
		case Mainnet:
			targetURL = MainnetSorobanURL
		case Futurenet:
			targetURL = FuturenetSorobanURL
		}
	}

	timer := c.startMethodTimer(ctx, "rpc.simulate_transaction", map[string]string{
		"network": c.GetNetworkName(),
		"rpc_url": targetURL,
	})
	defer func() {
		timer.Stop(err)
	}()

	logger.Logger.Debug("Simulating transaction (preflight)", "url", targetURL)

	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		return nil, errors.WrapRPCConnectionFailed(
			fmt.Errorf("circuit breaker open for %s", targetURL),
		)
	}

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

	// Validate payload size before attempting to send to network
	if err := ValidatePayloadSize(int64(len(bodyBytes))); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp SimulateTransactionResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		return nil, errors.WrapRPCError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return &rpcResp, nil
}

// GetHealth checks the health of the Soroban RPC endpoint.
func (c *Client) GetHealth(ctx context.Context) (*GetHealthResponse, error) {
	if len(c.AltURLs) == 0 {
		return nil, &AllNodesFailedError{}
	}
	var failures []NodeFailure
	for attempt := 0; attempt < len(c.AltURLs); attempt++ {
		resp, err := c.getHealthAttempt(ctx)
		if err == nil {
			c.markSuccess(c.SorobanURL)
			return resp, nil
		}

		c.markFailure(c.SorobanURL)
		failures = append(failures, NodeFailure{URL: c.SorobanURL, Reason: err})

		if attempt < len(c.AltURLs)-1 {
			logger.Logger.Warn("Retrying GetHealth with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}
	}
	return nil, &AllNodesFailedError{Failures: failures}
}

func (c *Client) getHealthAttempt(ctx context.Context) (healthResp *GetHealthResponse, err error) {
	targetURL := c.SorobanURL
	timer := c.startMethodTimer(ctx, "rpc.get_health", map[string]string{
		"network": c.GetNetworkName(),
		"rpc_url": targetURL,
	})
	defer func() {
		timer.Stop(err)
	}()

	logger.Logger.Debug("Checking Soroban RPC health", "url", targetURL)

	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed,
			fmt.Errorf("circuit breaker open for %s", targetURL),
		)
	}

	reqBody := GetHealthRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getHealth",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCMarshalFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	if err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	var rpcResp GetHealthResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	if rpcResp.Error != nil {
		return nil, errors.NewRPCError(errors.CodeRPCError, fmt.Errorf("rpc error from %s: %s (code %d)", targetURL, rpcResp.Error.Message, rpcResp.Error.Code))
	}

	logger.Logger.Info("Soroban RPC health check successful", "url", targetURL, "status", rpcResp.Result.Status)
	return &rpcResp, nil
}
