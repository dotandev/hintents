// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/snapshot"
)

// ImportConfig holds configuration for importing remote network state into a
// local snapshot file.
type ImportConfig struct {
	// ContractIDs lists contract IDs (strkey "C..." or 32-byte hex) whose
	// instance + code entries are fetched into the snapshot.
	ContractIDs []string
	// LedgerKeys lists raw base64 XDR LedgerKeys to fetch verbatim.
	LedgerKeys []string
	// OutputPath is the target snapshot JSON file. Required.
	OutputPath string
	// Network is the Stellar network to fetch from (testnet, mainnet, futurenet).
	Network string
	// RPCURL is an optional custom Soroban RPC URL overriding the network default.
	RPCURL string
	// Token is an optional RPC authentication token.
	Token string
	// Timeout bounds the whole import operation (default: 60s).
	Timeout time.Duration
}

// ImportResult summarizes what was written into the snapshot.
type ImportResult struct {
	// OutputPath is the snapshot file that was written.
	OutputPath string
	// Entries is the number of ledger entries in the final snapshot.
	Entries int
	// FetchedKeys counts keys fetched from RPC (cache misses).
	FetchedKeys int
	// Contracts lists the contract IDs that were resolved.
	Contracts []string
	// Fingerprint is the deterministic snapshot fingerprint.
	Fingerprint string
}

// ImportNetworkState fetches contract data for the given contracts/keys from a
// Soroban RPC endpoint and writes it into a soroban-cli compatible snapshot
// JSON file that the local simulator can load.
//
// For each contract ID the contract instance entry and its referenced
// contract code (WASM) entry are fetched, so the snapshot is self-contained
// for local replay. Additional raw ledger keys can be supplied verbatim.
//
// If OutputPath already exists, the fetched entries are merged into the
// existing snapshot (fetched values win), enabling incremental imports.
func ImportNetworkState(ctx context.Context, cfg ImportConfig) (*ImportResult, error) {
	if cfg.OutputPath == "" {
		return nil, fmt.Errorf("import: OutputPath is required")
	}
	if len(cfg.ContractIDs) == 0 && len(cfg.LedgerKeys) == 0 {
		return nil, fmt.Errorf("import: at least one contract ID or ledger key is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// Build the RPC client against the requested network. The local RPC cache
	// is deliberately disabled: an import must fetch fresh state from the
	// remote endpoint, never serve stale cached ledger entries.
	opts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(cfg.Network)),
		rpc.WithToken(cfg.Token),
		rpc.WithCacheEnabled(false),
	}
	if cfg.RPCURL != "" {
		opts = append(opts, rpc.WithSorobanURL(cfg.RPCURL))
	}
	client, err := rpc.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("import: failed to create RPC client: %w", err)
	}

	entries := make(map[string]string)

	// Merge with an existing snapshot so imports are incremental.
	if existing, err := snapshot.Load(cfg.OutputPath); err == nil {
		entries = existing.ToMap()
		logger.Logger.Info("Merging import into existing snapshot",
			"path", cfg.OutputPath, "existing_entries", len(entries))
	}

	fetched := 0

	// Fetch contract instance + code entries for each contract.
	for _, contractID := range cfg.ContractIDs {
		contractEntries, err := rpc.FetchContractBytecode(ctx, client, contractID)
		if err != nil {
			return nil, fmt.Errorf("import: contract %s: %w", contractID, err)
		}
		for k, v := range contractEntries {
			if _, existed := entries[k]; !existed {
				fetched++
			}
			entries[k] = v
		}
	}

	// Fetch any raw ledger keys verbatim.
	if len(cfg.LedgerKeys) > 0 {
		rawEntries, err := client.GetLedgerEntries(ctx, cfg.LedgerKeys)
		if err != nil {
			return nil, fmt.Errorf("import: ledger keys: %w", err)
		}
		for k, v := range rawEntries {
			if _, existed := entries[k]; !existed {
				fetched++
			}
			entries[k] = v
		}
	}

	if dir := filepath.Dir(cfg.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("import: failed to create output directory: %w", err)
		}
	}

	snap := snapshot.FromMap(entries)
	if err := snapshot.Save(cfg.OutputPath, snap); err != nil {
		return nil, fmt.Errorf("import: failed to write snapshot: %w", err)
	}

	return &ImportResult{
		OutputPath:  cfg.OutputPath,
		Entries:     len(snap.LedgerEntries),
		FetchedKeys: fetched,
		Contracts:   append([]string(nil), cfg.ContractIDs...),
		Fingerprint: snap.Fingerprint,
	}, nil
}
