// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"sync"
)

const (
	// DefaultMaxLedgerHeaders is the maximum number of historical ledger header entries
	// retained in memory before least-recently-used eviction occurs.
	DefaultMaxLedgerHeaders = 10000

	// DefaultCapacity is an alias for DefaultMaxLedgerHeaders.
	DefaultCapacity = DefaultMaxLedgerHeaders
)

// EvictionCallback is an optional callback invoked when an entry is evicted from the cache.
type EvictionCallback[K comparable, V any] func(key K, value V)

// CacheStats provides telemetry and operational metrics for an LRU cache.
type CacheStats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Puts      uint64
	Capacity  int
	Size      int
}

// lruNode is an intrusive node in the doubly-linked list.
type lruNode[K comparable, V any] struct {
	key   K
	value V
	prev  *lruNode[K, V]
	next  *lruNode[K, V]
}

// LRUCache implements a strict, thread-safe Least-Recently-Used (LRU) cache.
// All operations run in O(1) time complexity.
type LRUCache[K comparable, V any] struct {
	mu        sync.RWMutex
	capacity  int
	items     map[K]*lruNode[K, V]
	head      *lruNode[K, V] // sentinel MRU node; head.next is the most recently used
	tail      *lruNode[K, V] // sentinel LRU node; tail.prev is the least recently used
	onEvict   EvictionCallback[K, V]
	hits      uint64
	misses    uint64
	evictions uint64
	puts      uint64
}

// NewLRU creates a new strict LRUCache with the specified capacity and optional eviction callback.
// If capacity <= 0, DefaultCapacity (10,000) is used.
func NewLRU[K comparable, V any](capacity int, onEvict ...EvictionCallback[K, V]) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}

	var cb EvictionCallback[K, V]
	if len(onEvict) > 0 {
		cb = onEvict[0]
	}

	c := &LRUCache[K, V]{
		capacity: capacity,
		items:    make(map[K]*lruNode[K, V]),
		head:     &lruNode[K, V]{},
		tail:     &lruNode[K, V]{},
		onEvict:  cb,
	}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// attachMRU places a node immediately after the sentinel head (MRU position).
func (c *LRUCache[K, V]) attachMRU(n *lruNode[K, V]) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// detach unlinks a node from its current position in the list.
func (c *LRUCache[K, V]) detach(n *lruNode[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
	n.prev = nil
	n.next = nil
}

// moveToMRU moves an existing node to the MRU position.
func (c *LRUCache[K, V]) moveToMRU(n *lruNode[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev

	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// evictLRULocked removes the least recently used entry (tail.prev) from the cache.
// Caller must hold c.mu Lock.
func (c *LRUCache[K, V]) evictLRULocked() *lruNode[K, V] {
	if c.tail.prev == c.head {
		return nil
	}
	oldest := c.tail.prev
	oldest.prev.next = oldest.next
	oldest.next.prev = oldest.prev
	oldest.prev = nil
	oldest.next = nil

	delete(c.items, oldest.key)
	c.evictions++
	return oldest
}

// Put inserts or updates a key-value pair, marking it as most recently used.
// If the cache exceeds capacity, the least recently used entry is strictly evicted.
// Returns true if an existing entry was evicted to make room.
func (c *LRUCache[K, V]) Put(key K, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.puts++

	if n, ok := c.items[key]; ok {
		n.value = value
		c.moveToMRU(n)
		return false
	}

	n := &lruNode[K, V]{
		key:   key,
		value: value,
	}
	c.items[key] = n
	c.attachMRU(n)

	if len(c.items) > c.capacity {
		oldest := c.evictLRULocked()
		if c.onEvict != nil && oldest != nil {
			c.onEvict(oldest.key, oldest.value)
		}
		return true
	}
	return false
}

// Get retrieves the value associated with key and marks it as most recently used.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.hits++
		c.moveToMRU(n)
		return n.value, true
	}

	c.misses++
	var zero V
	return zero, false
}

// Peek returns the value associated with key WITHOUT updating its recency position or hit/miss stats.
func (c *LRUCache[K, V]) Peek(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if n, ok := c.items[key]; ok {
		return n.value, true
	}
	var zero V
	return zero, false
}

// Contains returns true if key exists in the cache, without updating its recency.
func (c *LRUCache[K, V]) Contains(key K) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.items[key]
	return ok
}

// Remove removes key from the cache. Returns true if key was present.
func (c *LRUCache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.detach(n)
		delete(c.items, key)
		if c.onEvict != nil {
			c.onEvict(n.key, n.value)
		}
		return true
	}
	return false
}

// Len returns the current number of cached entries.
func (c *LRUCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Capacity returns the maximum capacity of the cache.
func (c *LRUCache[K, V]) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capacity
}

// Resize changes the cache capacity and evicts excess least-recently-used entries if necessary.
func (c *LRUCache[K, V]) Resize(newCapacity int) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newCapacity <= 0 {
		newCapacity = 1
	}
	c.capacity = newCapacity

	evictedCount := 0
	for len(c.items) > c.capacity {
		oldest := c.evictLRULocked()
		if oldest != nil {
			evictedCount++
			if c.onEvict != nil {
				c.onEvict(oldest.key, oldest.value)
			}
		}
	}
	return evictedCount
}

// Purge removes all entries from the cache.
func (c *LRUCache[K, V]) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[K]*lruNode[K, V])
	c.head.next = c.tail
	c.tail.prev = c.head
}

// Keys returns all keys ordered from most-recently-used to least-recently-used.
func (c *LRUCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]K, 0, len(c.items))
	curr := c.head.next
	for curr != c.tail {
		keys = append(keys, curr.key)
		curr = curr.next
	}
	return keys
}

// Values returns all values ordered from most-recently-used to least-recently-used.
func (c *LRUCache[K, V]) Values() []V {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := make([]V, 0, len(c.items))
	curr := c.head.next
	for curr != c.tail {
		values = append(values, curr.value)
		curr = curr.next
	}
	return values
}

// Oldest returns the key and value of the least-recently-used entry without updating recency.
func (c *LRUCache[K, V]) Oldest() (key K, value V, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tail.prev == c.head {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}
	return c.tail.prev.key, c.tail.prev.value, true
}

// Newest returns the key and value of the most-recently-used entry without updating recency.
func (c *LRUCache[K, V]) Newest() (key K, value V, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.head.next == c.tail {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}
	return c.head.next.key, c.head.next.value, true
}

// Stats returns a snapshot of cache metrics.
func (c *LRUCache[K, V]) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Puts:      c.puts,
		Capacity:  c.capacity,
		Size:      len(c.items),
	}
}

// EvictionCount returns the total number of evictions performed by this cache.
func (c *LRUCache[K, V]) EvictionCount() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictions
}
