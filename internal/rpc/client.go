// Copyright 2026 Erst Users
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
	"github.com/dotandev/hintents/internal/metrics"

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

// Middleware defines a function that wraps an http.RoundTripper
type Middleware func(http.RoundTripper) http.RoundTripper

// RoundTripperFunc is a helper to implement http.RoundTripper with a function
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper
func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
	middlewares     []Middleware
	// rotateCount tracks how many times rotateURL has successfully switched
	// the active provider.  This is useful for metrics/observability when the
	// client is operating in a multi‑URL failover configuration.
	rotateCount int
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

type GetLatestLedgerResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		ID          string `json:"id"`
		Sequence    int    `json:"sequence"`
		CloseTime   string `json:"closeTime"`
		HeaderXdr   string `json:"headerXdr"`
		MetadataXdr string `json:"metadataXdr"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (e *AllNodesFailedError) Error() string {
	var reasons []string
	for _, f := range e.Failures {
		reasons = append(reasons, fmt.Sprintf("%s: %v", f.URL, f.Reason))
	}
	return fmt.Sprintf("all RPC endpoints failed: [%s]", strings.Join(reasons, ", "))
}

// Unwrap returns all per-node errors so errors.Is and errors.As can traverse them.
func (e *AllNodesFailedError) Unwrap() []error {
	errs := make([]error, len(e.Failures))
	for i, f := range e.Failures {
		errs[i] = f.Reason
	}
	return errs
}

// isHealthy checks if an endpoint is currently healthy or if circuit is open
// endpointAttempts returns how many attempts should be made across endpoint lists.
func (c *Client) endpointAttempts() int {
	if len(c.AltURLs) == 0 {
		return 1
	}
	return len(c.AltURLs)
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

// markHorizonFailure atomically reads HorizonURL and records a failure.
// This prevents a data race between the bare field read and rotateURL's write.
func (c *Client) markHorizonFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	url := c.HorizonURL
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	if c.lastFailure == nil {
		c.lastFailure = make(map[string]time.Time)
	}
	c.failures[url]++
	c.lastFailure[url] = time.Now()
}

// markHorizonSuccess atomically reads HorizonURL and resets its failure count.
func (c *Client) markHorizonSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	url := c.HorizonURL
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	c.failures[url] = 0
}

// markSorobanFailure atomically reads SorobanURL and records a failure.
func (c *Client) markSorobanFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	url := c.SorobanURL
	if c.failures == nil {
		c.failures = make(map[string]int)
	}
	if c.lastFailure == nil {
		c.lastFailure = make(map[string]time.Time)
	}
	c.failures[url]++
	c.lastFailure[url] = time.Now()
}

// markSorobanSuccess atomically reads SorobanURL and resets its failure count.
func (c *Client) markSorobanSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	url := c.SorobanURL
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
	// Keep SorobanURL in sync with HorizonURL when they were previously identical
	// so that health checks and Soroban RPC calls reflect the failover.
	if c.SorobanURL != "" {
		c.SorobanURL = c.HorizonURL
	}
	c.SorobanURL = c.HorizonURL
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = createHTTPClient(c.token, defaultHTTPTimeout, c.middlewares...)
	}
	c.Horizon = &horizonclient.Client{
		HorizonURL: c.HorizonURL,
		HTTP:       httpClient,
	}
	c.SorobanURL = c.AltURLs[c.currIndex]

	logger.Logger.Warn("RPC failover triggered", "new_url", c.HorizonURL)
	// increment counter under the same lock so readers get a consistent view
	c.rotateCount++
	return true
}

// RotateCount returns the number of times the client has switched
// to a different Horizon URL via rotateURL.  It is safe for concurrent
// use.
func (c *Client) RotateCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rotateCount
}

// attempts returns the number of retry attempts for failover loops (at least 1)
func (c *Client) attempts() int {
	if len(c.AltURLs) == 0 {
		return 1
	}
	return len(c.AltURLs)
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

// createHTTPClient creates an HTTP client with optional authentication, a configurable timeout, and custom middlewares.
//
// The transport chain is constructed innermost-first:
//
//	[user middlewares (outermost)] → RetryTransport → authTransport → http.DefaultTransport
//
// User middlewares therefore observe one call per logical request (inclusive of all
// retry attempts), which is suitable for per-request logging and header injection.
func createHTTPClient(token string, timeout time.Duration, middlewares ...Middleware) *http.Client {
	cfg := DefaultRetryConfig()

	// Innermost: base network transport.
	var transport http.RoundTripper = http.DefaultTransport

	// Auth sits just above the base so it runs on every retry attempt.
	if token != "" {
		transport = &authTransport{
			token:     token,
			transport: transport,
		}
	}

	// Retry wraps auth so that transient errors trigger a new authenticated attempt.
	transport = NewRetryTransport(cfg, transport)

	// Apply middlewares in reverse so that middlewares[0] becomes the outermost
	// wrapper and therefore the first to intercept an outbound request.
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			transport = middlewares[i](transport)
		}
	}

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

	httpClient := createHTTPClient("", defaultHTTPTimeout, nil)
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

type StellarbeatResponse struct {
	LatestLedger int `json:"latestLedger"`
	// You can add other fields if needed, like 'updatedAt'
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.getTransactionAttempt(ctx, hash)
		if err == nil {
			c.markHorizonSuccess()
			return resp, nil
		}

		c.markHorizonFailure()

		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		// Only rotate if this isn't the last possible URL
		if attempt < attempts-1 && len(c.AltURLs) > 1 {
			logger.Logger.Warn("Retrying with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}

		if len(c.AltURLs) <= 1 {
			return nil, err
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

	startTime := time.Now()

	// Fail fast if circuit breaker is open for this Horizon endpoint.
	if !c.isHealthy(c.HorizonURL) {
		err := fmt.Errorf("circuit breaker open for %s", c.HorizonURL)
		span.RecordError(err)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(c.HorizonURL, string(c.Network), false, time.Since(startTime))
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	tx, err := c.Horizon.TransactionDetail(hash)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err, "url", c.HorizonURL)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(c.HorizonURL, string(c.Network), false, duration)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	// Record successful remote node response
	metrics.RecordRemoteNodeResponse(c.HorizonURL, string(c.Network), true, duration)

	span.SetAttributes(
		attribute.Int("envelope.size_bytes", len(tx.EnvelopeXdr)),
		attribute.Int("result.size_bytes", len(tx.ResultXdr)),
		attribute.Int("result_meta.size_bytes", len(tx.ResultMetaXdr)),
	)

	logger.Logger.Debug("Transaction fetched", "hash", hash, "envelope_size", len(tx.EnvelopeXdr), "url", c.HorizonURL)

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

type LedgerEntryResult struct {
	Key                string `json:"key"`
	Xdr                string `json:"xdr"`
	LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
	LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
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
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.getLedgerHeaderAttempt(ctx, sequence)
		if err == nil {
			c.markHorizonSuccess()
			return resp, nil
		}

		c.markHorizonFailure()

		failures = append(failures, NodeFailure{URL: c.HorizonURL, Reason: err})

		if attempt < attempts-1 && len(c.AltURLs) > 1 {
			logger.Logger.Warn("Retrying ledger header fetch with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}

		if len(c.AltURLs) <= 1 {
			return nil, err
		}
	}
	// Single-node path: return the typed error directly so callers can use Is/As.
	if len(failures) == 1 {
		return nil, failures[0].Reason
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

	logger.Logger.Debug("Ledger header fetched successfully",
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
// This method implements batching and concurrent requests for large key sets
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

	// Batch keys into chunks for concurrent processing
	const batchSize = 50
	batches := chunkKeys(keysToFetch, batchSize)

	// Use concurrent requests for large batches
	if len(batches) > 1 {
		fetchedEntries, err := c.getLedgerEntriesConcurrent(ctx, batches)
		if err != nil {
			return nil, err
		}
		// Merge cached entries with fetched entries
		for k, v := range fetchedEntries {
			entries[k] = v
		}
		return entries, nil
	}

	// Single batch - use existing failover logic
	attempts := c.endpointAttempts()
	for attempt := 0; attempt < attempts; attempt++ {

		fetchedEntries, err := c.getLedgerEntriesAttempt(ctx, keysToFetch)
		if err == nil {
			// Merge cached entries with fetched entries
			for k, v := range fetchedEntries {
				entries[k] = v
			}
			return entries, nil
		}

		c.markSorobanFailure()

		if attempt < attempts-1 && len(c.AltURLs) > 1 {
			logger.Logger.Warn("Retrying with fallback Soroban RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}

		if len(c.AltURLs) <= 1 {
			return nil, err
		}
	}
	return nil, &AllNodesFailedError{Failures: []NodeFailure{}}
}

// getLedgerEntriesConcurrent fetches multiple batches concurrently with timeout handling
func (c *Client) getLedgerEntriesConcurrent(ctx context.Context, batches [][]string) (map[string]string, error) {
	tracer := telemetry.GetTracer()
	_, span := tracer.Start(ctx, "rpc_get_ledger_entries_concurrent")
	span.SetAttributes(
		attribute.Int("batch.count", len(batches)),
		attribute.String("network", string(c.Network)),
	)
	defer span.End()

	type batchResult struct {
		entries map[string]string
		err     error
	}

	results := make(chan batchResult, len(batches))
	var wg sync.WaitGroup

	// Create a context with timeout for all concurrent requests
	batchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.Logger.Debug("Fetching ledger entries concurrently",
		"batch_count", len(batches),
		"total_keys", sumBatchSizes(batches))

	for _, batch := range batches {
		wg.Add(1)
		go func(keys []string) {
			defer wg.Done()

			// Attempt with failover for each batch
			var entries map[string]string
			var err error
			for attempt := 0; attempt < len(c.AltURLs); attempt++ {
				entries, err = c.getLedgerEntriesAttempt(batchCtx, keys)
				if err == nil {
					break
				}
				if attempt < len(c.AltURLs)-1 {
					logger.Logger.Warn("Batch request failed, trying next URL", "error", err)
					c.rotateURL()
				}
			}

			results <- batchResult{entries: entries, err: err}
		}(batch)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	allEntries := make(map[string]string)
	var errs []error

	for result := range results {
		if result.err != nil {
			errs = append(errs, result.err)
			span.RecordError(result.err)
		} else {
			for k, v := range result.entries {
				allEntries[k] = v
			}
		}
	}

	// If any batch failed, return error
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to fetch %d/%d batches: %v", len(errs), len(batches), errs[0])
	}

	logger.Logger.Debug("Concurrent ledger entry fetch completed",
		"total_entries", len(allEntries),
		"batches", len(batches))

	return allEntries, nil
}

// sumBatchSizes calculates total number of keys across all batches
func sumBatchSizes(batches [][]string) int {
	total := 0
	for _, batch := range batches {
		total += len(batch)
	}
	return total
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

	startTime := time.Now()
	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		err := errors.WrapRPCConnectionFailed(
			fmt.Errorf("circuit breaker open for %s", targetURL),
		)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, time.Since(startTime))

		return nil, err
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
	duration := time.Since(startTime)
	if err != nil {
		logger.Logger.Error("Soroban getLedgerEntries request failed", "url", targetURL, "error", err)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp GetLedgerEntriesResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban getLedgerEntries response unmarshal failed", "url", targetURL, "error", err)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban getLedgerEntries RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		return nil, errors.WrapSorobanError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Record successful remote node response
	metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), true, duration)

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

	logger.Logger.Debug("Ledger entries fetched",
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
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.simulateTransactionAttempt(ctx, envelopeXdr)
		if err == nil {
			c.markSorobanSuccess()
			return resp, nil
		}

		c.markSorobanFailure()

		failures = append(failures, NodeFailure{URL: c.SorobanURL, Reason: err})

		if attempt < attempts-1 && len(c.AltURLs) > 1 {
			logger.Logger.Warn("Retrying transaction simulation with fallback RPC...", "error", err)
			if !c.rotateURL() {
				break
			}
			continue
		}

		if len(c.AltURLs) <= 1 {
			return nil, err
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
		logger.Logger.Error("Soroban simulateTransaction request failed", "url", targetURL, "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		logger.Logger.Error("Soroban simulateTransaction response too large", "url", targetURL, "status", resp.StatusCode)
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Error("Soroban simulateTransaction response read failed", "url", targetURL, "error", err)
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp SimulateTransactionResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban simulateTransaction unmarshal failed", "url", targetURL, "error", err)
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban simulateTransaction RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		return nil, errors.WrapSorobanError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	logger.Logger.Debug("Soroban simulateTransaction succeeded", "url", targetURL)
	return &rpcResp, nil
}

// GetHealth checks the health of the Soroban RPC endpoint.
func (c *Client) GetHealth(ctx context.Context) (*GetHealthResponse, error) {
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.getHealthAttempt(ctx)
		if err == nil {
			c.markSorobanSuccess()
			return resp, nil
		}

		c.markSorobanFailure()
		failures = append(failures, NodeFailure{URL: c.SorobanURL, Reason: err})

		if attempt < attempts-1 && len(c.AltURLs) > 1 {
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
	if targetURL == "" {
		targetURL = c.HorizonURL
	}
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

	// Prefer SorobanURL but fall back to the currently active HorizonURL so that
	// rotateURL-triggered failovers are reflected in health checks.
	targetURL = c.SorobanURL
	if targetURL == "" {
		targetURL = c.HorizonURL
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	if err != nil {
		logger.Logger.Error("Soroban getHealth request failed", "url", targetURL, "error", err)
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Error("Soroban getHealth response read failed", "url", targetURL, "error", err)
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	var rpcResp GetHealthResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban getHealth unmarshal failed", "url", targetURL, "error", err)
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban getHealth RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		return nil, errors.NewRPCError(errors.CodeRPCError, fmt.Errorf("rpc error from %s: %s (code %d)", targetURL, rpcResp.Error.Message, rpcResp.Error.Code))
	}

	logger.Logger.Debug("Soroban RPC health check successful", "url", targetURL, "status", rpcResp.Result.Status)
	return &rpcResp, nil
}

//  Warn if RPC node is lagging behind current ledge

func (c *Client) postRequest(ctx context.Context, payload interface{}, result interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use c.SorobanURL as the endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", c.SorobanURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use the client's internal httpClient
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// GetLatestLedgerSequence fetches the latest ledger from the node this client is configured for.
func (c *Client) GetLatestLedgerSequence(ctx context.Context) (int, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getLatestLedger",
	}

	var resp GetLatestLedgerResponse
	err := c.postRequest(ctx, payload, &resp)
	if err != nil {
		return 0, err
	}

	return resp.Result.Sequence, nil
}

func fetchLatestFromSDF(ctx context.Context, url string) (int, error) {
	// 1. Prepare the JSON-RPC payload
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getLatestLedger",
	}
	body, _ := json.Marshal(payload)

	// 2. Create the request with a strict timeout
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// 3. Decode using the struct you found earlier
	var rpcResp GetLatestLedgerResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return 0, err
	}

	if rpcResp.Error != nil {
		return 0, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	return rpcResp.Result.Sequence, nil
}

func (c *Client) CheckStaleness(ctx context.Context, network string) error {
	// 1. Get the ledger sequence from the user's configured RPC (the local node)
	localLedger, err := c.GetLatestLedgerSequence(ctx)
	if err != nil {
		return fmt.Errorf("failed to get local ledger: %w", err)
	}

	// 2. Determine the official reference URL based on the network
	var referenceURL string
	switch strings.ToLower(network) {
	case "testnet":
		referenceURL = "https://soroban-testnet.stellar.org"
	case "public":
		referenceURL = "https://soroban.stellar.org"
	default:
		// Skip check for 'standalone' or unknown networks
		return nil
	}

	// 3. Fetch the latest ledger from the official SDF reference node
	refLedger, err := fetchLatestFromSDF(ctx, referenceURL)
	if err != nil {
		// We don't want to block the tool if the internet is down,
		// just log it and move on.
		return nil
	}

	// 4. Compare
	const threshold = 15 // ~1.5 minutes of lag
	if refLedger > localLedger+threshold {
		fmt.Printf("\033[33m[WARN]\033[0m Local node is lagging! (Local: %d, Network: %d). \n", localLedger, refLedger)
		fmt.Println("       Traces and replays might use outdated contract state.")
	}

	return nil
}
