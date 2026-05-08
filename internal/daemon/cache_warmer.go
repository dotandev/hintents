// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"context"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	// defaultWarmingInterval defines how often we refresh the hot contract cache.
	defaultWarmingInterval = 5 * time.Minute
	// warmingTimeout is the maximum time allowed for a single warming cycle.
	warmingTimeout = 2 * time.Minute
)

// hotContracts maps network names to their high-traffic contract IDs.
// These are preemptively cached to eliminate cold-start latency for simulations.
var hotContracts = map[rpc.Network][]string{
	rpc.Mainnet: {
		"CAS3J7GYCCX3TP377666F6A6X6X6X6X6X6X6X6X6X6X6X6X6X6X6X6X", // Native XLM
		"CCW67ZTM6S7C3S64Z6S3SLZ6S7C3S64Z6S3SLZ6S7C3S64Z6S3SLZ6S", // USDC
		"CD4S6G64HULH7YFCTB6V7S76YF4GZTYZ6S3SLZ6S7C3S64Z6S3SLZ6S", // Soroban Router (Example)
	},
	rpc.Testnet: {
		"CDLZSTBBZUTG4F32CH7YJYSZQMHSFVP6XQ6YUKMCTM4S3YFEW6M6CSRE", // Community Test Contract
	},
}

// CacheWarmer is a background worker that preemptively fetches and caches
// ledger entries for high-traffic contracts and their WASM dependencies.
type CacheWarmer struct {
	client   *rpc.Client
	interval time.Duration
}

// NewCacheWarmer initializes a new CacheWarmer instance.
func NewCacheWarmer(client *rpc.Client) *CacheWarmer {
	return &CacheWarmer{
		client:   client,
		interval: defaultWarmingInterval,
	}
}

// Start runs the cache warmer loop. It blocks until the context is cancelled.
func (cw *CacheWarmer) Start(ctx context.Context) {
	if cw.client == nil {
		logger.Logger.Error("Cache warmer failed to start: RPC client is nil")
		return
	}

	logger.Logger.Info("Starting cache warmer daemon",
		"interval", cw.interval,
		"network", cw.client.Network)

	// Run an initial warming cycle immediately
	cw.performWarming(ctx)

	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Logger.Info("Cache warmer daemon shutting down")
			return
		case <-ticker.C:
			cw.performWarming(ctx)
		}
	}
}

// performWarming executes a single warming cycle with its own timeout context.
func (cw *CacheWarmer) performWarming(ctx context.Context) {
	warmCtx, cancel := context.WithTimeout(ctx, warmingTimeout)
	defer cancel()

	network := cw.client.Network
	contracts := hotContracts[network]
	if len(contracts) == 0 {
		logger.Logger.Debug("No hot contracts defined for network", "network", network)
		return
	}

	logger.Logger.Debug("Starting cache warming cycle",
		"network", network,
		"target_contracts", len(contracts))

	start := time.Now()

	// Stage 1: Fetch Contract Instances
	instanceKeys := make([]xdr.LedgerKey, 0, len(contracts))
	for _, id := range contracts {
		key, err := rpc.NewContractInstanceKey(id)
		if err != nil {
			logger.Logger.Warn("Skipping invalid hot contract ID", "id", id, "error", err)
			continue
		}
		instanceKeys = append(instanceKeys, key)
	}

	encodedInstances, err := rpc.EncodeLedgerKeys(instanceKeys)
	if err != nil {
		logger.Logger.Error("Failed to encode instance keys", "error", err)
		return
	}

	// GetLedgerEntries handles caching internally
	entries, err := cw.client.GetLedgerEntries(warmCtx, encodedInstances)
	if err != nil {
		logger.Logger.Error("Failed to fetch contract instances during warming", "error", err)
		return
	}

	// Stage 2: Resolve and Fetch WASM dependencies
	codeKeys := make([]xdr.LedgerKey, 0)
	for _, entryXDR := range entries {
		wasmHash, ok := rpc.ParseWasmHashFromInstance(entryXDR)
		if !ok {
			continue
		}
		key, err := rpc.NewContractCodeKey(wasmHash)
		if err != nil {
			continue
		}
		codeKeys = append(codeKeys, key)
	}

	var codeCount int
	if len(codeKeys) > 0 {
		encodedCode, err := rpc.EncodeLedgerKeys(codeKeys)
		if err != nil {
			logger.Logger.Error("Failed to encode code keys", "error", err)
		} else {
			codeEntries, err := cw.client.GetLedgerEntries(warmCtx, encodedCode)
			if err != nil {
				logger.Logger.Error("Failed to fetch WASM code during warming", "error", err)
			}
			codeCount = len(codeEntries)
		}
	}

	logger.Logger.Info("Cache warming cycle completed",
		"duration", time.Since(start).String(),
		"instances_cached", len(entries),
		"wasm_cached", codeCount)
}
