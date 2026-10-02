// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/dotandev/hintents/internal/state"
)

// LedgerFetcher fetches ledger headers from the network (e.g. *rpc.Client).
type LedgerFetcher interface {
	GetLedgerHeader(ctx context.Context, sequence uint32) (*LedgerHeaderResponse, error)
}

// LedgerHeaderCache provides an institutional-grade, thread-safe LRU cache
// dedicated to historical Stellar ledger headers. It strictly bounds memory
// consumption over long-running node sessions by evicting the least-recently-used
// ledger headers once capacity (default: 10,000) is reached.
type LedgerHeaderCache struct {
	lru *state.LRUCache[uint32, *LedgerHeaderResponse]
}

// LedgerCache is an alias for LedgerHeaderCache.
type LedgerCache = LedgerHeaderCache

// NewLedgerHeaderCache creates a new LedgerHeaderCache with the specified capacity.
// If capacity <= 0, state.DefaultMaxLedgerHeaders (10,000 entries) is enforced.
func NewLedgerHeaderCache(capacity ...int) *LedgerHeaderCache {
	capSize := state.DefaultMaxLedgerHeaders
	if len(capacity) > 0 && capacity[0] > 0 {
		capSize = capacity[0]
	}

	return &LedgerHeaderCache{
		lru: state.NewLRU[uint32, *LedgerHeaderResponse](capSize),
	}
}

// NewLedgerHeaderCacheWithCallback creates a new LedgerHeaderCache with an eviction callback.
func NewLedgerHeaderCacheWithCallback(capacity int, onEvict func(seq uint32, header *LedgerHeaderResponse)) *LedgerHeaderCache {
	if capacity <= 0 {
		capacity = state.DefaultMaxLedgerHeaders
	}
	var cb state.EvictionCallback[uint32, *LedgerHeaderResponse]
	if onEvict != nil {
		cb = state.EvictionCallback[uint32, *LedgerHeaderResponse](onEvict)
	}
	return &LedgerHeaderCache{
		lru: state.NewLRU[uint32, *LedgerHeaderResponse](capacity, cb),
	}
}

// NewLedgerCache creates a new LedgerCache, defaulting to 10,000 entries.
func NewLedgerCache(capacity ...int) *LedgerCache {
	return NewLedgerHeaderCache(capacity...)
}

// Get retrieves a cached ledger header by sequence, marking it as most recently used.
func (c *LedgerHeaderCache) Get(sequence uint32) (*LedgerHeaderResponse, bool) {
	return c.lru.Get(sequence)
}

// Put caches a ledger header using header.Sequence as key.
// If header is nil, Put is a no-op and returns false.
// Returns true if an older ledger header was evicted to make room.
func (c *LedgerHeaderCache) Put(header *LedgerHeaderResponse) bool {
	if header == nil {
		return false
	}
	return c.lru.Put(header.Sequence, header)
}

// PutWithSequence caches a ledger header with an explicit sequence number.
// Returns true if an older ledger header was evicted to make room.
func (c *LedgerHeaderCache) PutWithSequence(sequence uint32, header *LedgerHeaderResponse) bool {
	return c.lru.Put(sequence, header)
}

// Peek returns the cached ledger header without modifying recency order or stats.
func (c *LedgerHeaderCache) Peek(sequence uint32) (*LedgerHeaderResponse, bool) {
	return c.lru.Peek(sequence)
}

// Contains checks whether a ledger sequence is present in the cache without updating recency.
func (c *LedgerHeaderCache) Contains(sequence uint32) bool {
	return c.lru.Contains(sequence)
}

// Remove deletes a ledger header from the cache. Returns true if found and removed.
func (c *LedgerHeaderCache) Remove(sequence uint32) bool {
	return c.lru.Remove(sequence)
}

// Len returns the number of ledger headers currently cached.
func (c *LedgerHeaderCache) Len() int {
	return c.lru.Len()
}

// Capacity returns the maximum entry capacity (capped at 10,000 by default).
func (c *LedgerHeaderCache) Capacity() int {
	return c.lru.Capacity()
}

// Resize changes the cache capacity and evicts excess entries according to strict LRU order.
func (c *LedgerHeaderCache) Resize(newCapacity int) int {
	return c.lru.Resize(newCapacity)
}

// Purge evicts all entries from the cache.
func (c *LedgerHeaderCache) Purge() {
	c.lru.Purge()
}

// Stats returns operational metrics including hits, misses, evictions, and puts.
func (c *LedgerHeaderCache) Stats() state.CacheStats {
	return c.lru.Stats()
}

// EvictionCount returns the cumulative number of evicted historical ledger headers.
func (c *LedgerHeaderCache) EvictionCount() uint64 {
	return c.lru.EvictionCount()
}

// GetOrFetch retrieves a ledger header from the cache if available; otherwise,
// fetches it from the provided LedgerFetcher, caches it (subject to LRU eviction),
// and returns it.
func (c *LedgerHeaderCache) GetOrFetch(ctx context.Context, fetcher LedgerFetcher, sequence uint32) (*LedgerHeaderResponse, error) {
	if header, ok := c.Get(sequence); ok {
		return header, nil
	}

	header, err := fetcher.GetLedgerHeader(ctx, sequence)
	if err != nil {
		return nil, err
	}

	c.Put(header)
	return header, nil
}
