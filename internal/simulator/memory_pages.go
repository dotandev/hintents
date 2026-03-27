// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"os"
	"strconv"
)

const (
	// WasmPageSize is the standard WASM memory page size (64 KiB).
	WasmPageSize uint64 = 64 * 1024

	// DefaultInitialPages is the number of pages allocated on first request (1 MiB).
	DefaultInitialPages uint64 = 16

	// DefaultMaxPages is the hard ceiling (64 MiB = 1024 pages).
	DefaultMaxPages uint64 = 1024

	// DefaultInitialMemoryBytes is the lazy starting point instead of full 64 MiB.
	DefaultInitialMemoryBytes = DefaultInitialPages * WasmPageSize

	// DefaultMaxMemoryBytes is the absolute ceiling (64 MiB).
	DefaultMaxMemoryBytes = DefaultMaxPages * WasmPageSize
)

// PagedMemoryConfig controls how memory pages are allocated per simulation.
type PagedMemoryConfig struct {
	// InitialBytes is the memory allocated upfront for a new simulation.
	// Defaults to DefaultInitialMemoryBytes (1 MiB).
	InitialBytes uint64

	// MaxBytes is the hard ceiling. Pages are grown lazily up to this limit.
	// Defaults to DefaultMaxMemoryBytes (64 MiB).
	MaxBytes uint64

	// LazyGrowth enables on-demand page expansion instead of pre-allocating
	// the full MaxBytes upfront.
	LazyGrowth bool
}

// DefaultPagedMemoryConfig returns the recommended lazy-growth configuration.
func DefaultPagedMemoryConfig() PagedMemoryConfig {
	cfg := PagedMemoryConfig{
		InitialBytes: DefaultInitialMemoryBytes,
		MaxBytes:     DefaultMaxMemoryBytes,
		LazyGrowth:   true,
	}

	// Allow env overrides for tuning without recompilation.
	if v := os.Getenv("ERST_SIM_MEMORY_INITIAL_BYTES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			cfg.InitialBytes = alignToPage(n)
		}
	}
	if v := os.Getenv("ERST_SIM_MEMORY_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			cfg.MaxBytes = alignToPage(n)
		}
	}

	return cfg
}

// alignToPage rounds n up to the nearest WASM page boundary (64 KiB).
func alignToPage(n uint64) uint64 {
	if n == 0 {
		return WasmPageSize
	}
	pages := (n + WasmPageSize - 1) / WasmPageSize
	return pages * WasmPageSize
}

// effectiveMemoryLimit returns the memory limit to set on a SimulationRequest
// based on the paged config. When LazyGrowth is enabled, we start with
// InitialBytes instead of eagerly allocating MaxBytes.
func effectiveMemoryLimit(cfg PagedMemoryConfig, req *SimulationRequest) uint64 {
	// Explicit per-request override always wins.
	if req != nil && req.MemoryLimit != nil {
		return *req.MemoryLimit
	}
	if cfg.LazyGrowth {
		return cfg.InitialBytes
	}
	return cfg.MaxBytes
}

// applyPagedMemoryLimit sets the MemoryLimit on req using the paged config,
// unless the request already has an explicit limit set.
func applyPagedMemoryLimit(cfg PagedMemoryConfig, req *SimulationRequest) {
	if req == nil || req.MemoryLimit != nil {
		return
	}
	limit := effectiveMemoryLimit(cfg, req)
	req.MemoryLimit = &limit
}
