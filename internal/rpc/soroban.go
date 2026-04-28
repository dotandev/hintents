// Copyright 2026 Erst Users
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

	"github.com/dotandev/hintents/internal/endpoints"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/metrics"
	"github.com/dotandev/hintents/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

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

		c.markFailure(c.SorobanURL)

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
		c.recordTelemetry(targetURL, time.Since(startTime), false)

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
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp GetLedgerEntriesResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban getLedgerEntries response unmarshal failed", "url", targetURL, "error", err)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban getLedgerEntries RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		// Record failed remote node response
		metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), false, duration)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapSorobanError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Record successful remote node response
	metrics.RecordRemoteNodeResponse(targetURL, string(c.Network), true, duration)
	c.recordTelemetry(targetURL, duration, true)

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
	if err := VerifyLedgerEntries(keysToFetch, rpcResp.Result.Entries); err != nil {
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
			c.markSuccess(c.SorobanURL)
			return resp, nil
		}

		c.markFailure(c.SorobanURL)

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
	startTime := time.Now()

	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		c.recordTelemetry(targetURL, time.Since(startTime), false)
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
	duration := time.Since(startTime)
	if err != nil {
		logger.Logger.Error("Soroban simulateTransaction request failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapRPCConnectionFailed(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		logger.Logger.Error("Soroban simulateTransaction response too large", "url", targetURL, "status", resp.StatusCode)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapRPCResponseTooLarge(targetURL)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Error("Soroban simulateTransaction response read failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapUnmarshalFailed(err, "body read error")
	}

	var rpcResp SimulateTransactionResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban simulateTransaction unmarshal failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapUnmarshalFailed(err, string(respBytes))
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban simulateTransaction RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.WrapSorobanError(targetURL, rpcResp.Error.Message, rpcResp.Error.Code)
	}

	c.recordTelemetry(targetURL, duration, true)
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
			c.markSuccess(c.SorobanURL)
			return resp, nil
		}

		c.markFailure(c.SorobanURL)
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

	startTime := time.Now()
	logger.Logger.Debug("Checking Soroban RPC health", "url", targetURL)

	// Fail fast if circuit breaker is open for this Soroban endpoint.
	if !c.isHealthy(targetURL) {
		c.recordTelemetry(targetURL, time.Since(startTime), false)
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

	// Prefer SorobanURL but fall back to current HorizonURL so failovers are reflected.
	targetURL = c.SorobanURL
	if targetURL == "" {
		targetURL = c.HorizonURL
	}

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.recordTelemetry(targetURL, time.Since(startTime), false)
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.getHTTPClient().Do(req)
	duration := time.Since(startTime)
	if err != nil {
		logger.Logger.Error("Soroban getHealth request failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.NewRPCError(errors.CodeRPCConnectionFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Error("Soroban getHealth response read failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	var rpcResp GetHealthResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		logger.Logger.Error("Soroban getHealth unmarshal failed", "url", targetURL, "error", err)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.NewRPCError(errors.CodeRPCUnmarshalFailed, err)
	}

	if rpcResp.Error != nil {
		logger.Logger.Error("Soroban getHealth RPC error", "url", targetURL, "code", rpcResp.Error.Code, "message", rpcResp.Error.Message)
		c.recordTelemetry(targetURL, duration, false)
		return nil, errors.NewRPCError(errors.CodeRPCError, fmt.Errorf("rpc error from %s: %s (code %d)", targetURL, rpcResp.Error.Message, rpcResp.Error.Code))
	}

	c.recordTelemetry(targetURL, duration, true)
	logger.Logger.Debug("Soroban RPC health check successful", "url", targetURL, "status", rpcResp.Result.Status)
	return &rpcResp, nil
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
	defer func() { _ = resp.Body.Close() }()

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
		referenceURL = endpoints.SorobanTestnet
	case "mainnet", "public":
		referenceURL = endpoints.SorobanMainnet
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
