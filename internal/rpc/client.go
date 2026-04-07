// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	stdErrors "errors"
	"fmt"
	"net/http"
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

// HTTPClient is an interface that matches http.Client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
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
	httpClient      HTTPClient
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
	rotateCount     int
	healthCollector *HealthCollector
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
		httpClient = createHTTPClient(c.token, defaultHTTPTimeout, c.middlewares...)
	}
	c.Horizon = &horizonclient.Client{
		HorizonURL: c.HorizonURL,
		HTTP:       httpClient,
	}
	// Keep SorobanURL in sync with the newly selected node so that
	// Soroban JSON-RPC calls use the same failover endpoint as Horizon.
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

func (c *Client) getHTTPClient() HTTPClient {
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

// GetHealthReport returns a snapshot of health telemetry for all known RPC nodes.
func (c *Client) GetHealthReport() *HealthReport {
	if c.healthCollector == nil {
		return &HealthReport{
			Nodes:       []NodeHealthStats{},
			GeneratedAt: time.Now(),
			Network:     c.GetNetworkName(),
		}
	}

	stats := c.healthCollector.GetAllStats()

	// Update circuit breaker state from the client's failure tracking
	c.mu.RLock()
	for i := range stats {
		stats[i].CircuitOpen = !c.isHealthyLocked(stats[i].URL)
	}
	c.mu.RUnlock()

	return &HealthReport{
		Nodes:       stats,
		GeneratedAt: time.Now(),
		Network:     c.GetNetworkName(),
	}
}

// recordTelemetry records request telemetry if the health collector is available.
func (c *Client) recordTelemetry(url string, latency time.Duration, success bool) {
	if c.healthCollector != nil {
		c.healthCollector.RecordRequest(url, latency, success)
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
		Horizon:         horizonClient,
		Network:         "custom",
		SorobanURL:      sorobanURL,
		Config:          config,
		CacheEnabled:    true,
		httpClient:      httpClient,
		healthCollector: NewHealthCollector(),
	}, nil
}

// GetTransaction fetches the transaction details and full XDR data
func (c *Client) GetTransaction(ctx context.Context, hash string) (*TransactionResponse, error) {
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.getTransactionAttempt(ctx, hash)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			return resp, nil
		}

		c.markFailure(c.HorizonURL)

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
		c.recordTelemetry(c.HorizonURL, time.Since(startTime), false)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	tx, err := c.Horizon.TransactionDetail(hash)
	duration := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		logger.Logger.Error("Failed to fetch transaction", "hash", hash, "error", err, "url", c.HorizonURL)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(c.HorizonURL, string(c.Network), false, duration)
		c.recordTelemetry(c.HorizonURL, duration, false)

		// Check if it's a 404 (Transaction Not Found)
		if hErr, ok := err.(*horizonclient.Error); ok && hErr.Problem.Status == 404 {
			c.recordTelemetry(c.HorizonURL, duration, true)
			return nil, errors.WrapTransactionNotFound(err)
		}

		c.recordTelemetry(c.HorizonURL, duration, false)

		return nil, errors.WrapRPCConnectionFailed(err)
	}

	// Record successful remote node response
	metrics.RecordRemoteNodeResponse(c.HorizonURL, string(c.Network), true, duration)
	c.recordTelemetry(c.HorizonURL, duration, true)

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

// GetLedgerHeader fetches ledger header details for a specific sequence with automatic fallback.
func (c *Client) GetLedgerHeader(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error) {
	attempts := c.endpointAttempts()
	var failures []NodeFailure
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := c.getLedgerHeaderAttempt(ctx, sequence)
		if err == nil {
			c.markSuccess(c.HorizonURL)
			return resp, nil
		}

		c.markFailure(c.HorizonURL)

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
	resp, err := c.getHTTPClient().Do(req)
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
