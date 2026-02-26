// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"sync"

	"github.com/dotandev/hintents/internal/logger"
)

// batchSize is the maximum number of ledger keys per getLedgerEntries request.
// Soroban RPC enforces a per-request key limit; empirically 100 is safe across
// all public network nodes and avoids HTTP 413 responses.
const batchSize = 100

// largFootprintThreshold is the minimum number of keys that triggers batching.
// Transactions touching fewer keys are sent in a single request.
const largFootprintThreshold = 100

// batchResult carries the outcome of a single batch fetch.
type batchResult struct {
	entries map[string]string
	err     error
}

// chunkKeys splits keys into slices of at most size n.
func chunkKeys(keys []string, n int) [][]string {
	if n <= 0 {
		n = batchSize
	}
	chunks := make([][]string, 0, (len(keys)+n-1)/n)
	for len(keys) > 0 {
		end := n
		if end > len(keys) {
			end = len(keys)
		}
		chunks = append(chunks, keys[:end])
		keys = keys[end:]
	}
	return chunks
}

// BatchGetLedgerEntries fetches ledger entries for an arbitrary number of keys,
// automatically batching requests when the footprint exceeds largFootprintThreshold.
// All batches are dispatched concurrently; results are merged before returning.
//
// For footprints ≤ largFootprintThreshold the call delegates directly to
// GetLedgerEntries so the fast path incurs no additional overhead.
func (c *Client) BatchGetLedgerEntries(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	if len(keys) <= largFootprintThreshold {
		return c.GetLedgerEntries(ctx, keys)
	}

	chunks := chunkKeys(keys, batchSize)
	logger.Logger.Info("Batching large footprint ledger fetch",
		"total_keys", len(keys),
		"batch_count", len(chunks),
		"batch_size", batchSize,
	)

	results := make([]batchResult, len(chunks))
	var wg sync.WaitGroup
	wg.Add(len(chunks))

	for i, chunk := range chunks {
		i, chunk := i, chunk // capture loop variables
		go func() {
			defer wg.Done()
			entries, err := c.GetLedgerEntries(ctx, chunk)
			results[i] = batchResult{entries: entries, err: err}
		}()
	}

	wg.Wait()

	merged := make(map[string]string, len(keys))
	for i, r := range results {
		if r.err != nil {
			logger.Logger.Error("Batch ledger fetch failed",
				"batch_index", i,
				"error", r.err,
			)
			return nil, r.err
		}
		for k, v := range r.entries {
			merged[k] = v
		}
	}

	logger.Logger.Info("Batch ledger fetch complete",
		"total_keys", len(keys),
		"returned_entries", len(merged),
	)

	return merged, nil
}
