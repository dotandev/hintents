// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
)

func makeHeader(seq uint32) *rpc.LedgerHeaderResponse {
	return &rpc.LedgerHeaderResponse{
		Sequence:          seq,
		Hash:              fmt.Sprintf("hash-%08d", seq),
		PrevHash:          fmt.Sprintf("prev-%08d", seq-1),
		CloseTime:         time.Now(),
		ProtocolVersion:   21,
		BaseFee:           100,
		BaseReserve:       10000000,
		MaxTxSetSize:      1000,
		TotalCoins:        "1000000000",
		FeePool:           "500",
		HeaderXDR:         "AAAA...",
		SuccessfulTxCount: 10,
		FailedTxCount:     1,
		OperationCount:    15,
	}
}

type mockFetcher struct {
	mu        sync.Mutex
	calls     int
	failSeq   map[uint32]bool
	customRes map[uint32]*rpc.LedgerHeaderResponse
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		failSeq:   make(map[uint32]bool),
		customRes: make(map[uint32]*rpc.LedgerHeaderResponse),
	}
}

func (m *mockFetcher) GetLedgerHeader(ctx context.Context, seq uint32) (*rpc.LedgerHeaderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	if m.failSeq[seq] {
		return nil, errors.New("ledger not found")
	}
	if res, ok := m.customRes[seq]; ok {
		return res, nil
	}
	return makeHeader(seq), nil
}

func (m *mockFetcher) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestDefaultCapacity(t *testing.T) {
	cache := NewLedgerHeaderCache()
	if cache.Capacity() != DefaultMaxLedgerHeaders {
		t.Fatalf("expected default capacity %d, got %d", DefaultMaxLedgerHeaders, cache.Capacity())
	}
	if cache.Len() != 0 {
		t.Fatalf("expected initial len 0, got %d", cache.Len())
	}

	// Test zero and negative capacity fallback
	cacheZero := NewLedgerHeaderCache(0)
	if cacheZero.Capacity() != DefaultMaxLedgerHeaders {
		t.Fatalf("expected fallback capacity %d, got %d", DefaultMaxLedgerHeaders, cacheZero.Capacity())
	}
	cacheNeg := NewLedgerHeaderCache(-5)
	if cacheNeg.Capacity() != DefaultMaxLedgerHeaders {
		t.Fatalf("expected fallback capacity %d, got %d", DefaultMaxLedgerHeaders, cacheNeg.Capacity())
	}

	aliasCache := NewLedgerCache()
	if aliasCache.Capacity() != DefaultMaxLedgerHeaders {
		t.Fatalf("expected alias cache capacity %d, got %d", DefaultMaxLedgerHeaders, aliasCache.Capacity())
	}
}

func TestStrictLRUEvictionOrder(t *testing.T) {
	// Create cache with capacity 3
	cache := NewLedgerHeaderCache(3)

	cache.Put(makeHeader(1))
	cache.Put(makeHeader(2))
	cache.Put(makeHeader(3))

	if cache.Len() != 3 {
		t.Fatalf("expected len 3, got %d", cache.Len())
	}

	// Order is now: 3 (MRU), 2, 1 (LRU)
	// Adding 4 should strictly evict 1
	evicted := cache.Put(makeHeader(4))
	if !evicted {
		t.Fatal("expected eviction to occur")
	}

	if cache.Contains(1) {
		t.Fatal("expected ledger 1 to be evicted")
	}
	if !cache.Contains(2) || !cache.Contains(3) || !cache.Contains(4) {
		t.Fatal("expected ledgers 2, 3, 4 to be in cache")
	}

	// Access 2 to make it MRU
	// Order becomes: 2 (MRU), 4, 3 (LRU)
	val, ok := cache.Get(2)
	if !ok || val.Sequence != 2 {
		t.Fatal("expected to retrieve ledger 2")
	}

	// Adding 5 should evict 3 (the least recently used)
	cache.Put(makeHeader(5))

	if cache.Contains(3) {
		t.Fatal("expected ledger 3 to be evicted after 2 was refreshed")
	}
	if !cache.Contains(2) || !cache.Contains(4) || !cache.Contains(5) {
		t.Fatal("expected ledgers 2, 4, 5 to remain")
	}
}

func TestPutUpdateExistingKey(t *testing.T) {
	cache := NewLedgerHeaderCache(3)
	cache.Put(makeHeader(1))
	cache.Put(makeHeader(2))
	cache.Put(makeHeader(3))

	// Update existing key 1 with a new object
	updatedHeader := makeHeader(1)
	updatedHeader.Hash = "updated-hash-1"
	evicted := cache.Put(updatedHeader)
	if evicted {
		t.Fatal("updating existing key should not trigger eviction")
	}

	if cache.Len() != 3 {
		t.Fatalf("expected len 3, got %d", cache.Len())
	}

	// Key 1 is now MRU: 1 (MRU), 3, 2 (LRU)
	// Adding 4 should evict 2
	cache.Put(makeHeader(4))

	if cache.Contains(2) {
		t.Fatal("expected ledger 2 to be evicted as LRU")
	}
	if !cache.Contains(1) {
		t.Fatal("expected updated ledger 1 to remain in cache")
	}

	retrieved, ok := cache.Get(1)
	if !ok || retrieved.Hash != "updated-hash-1" {
		t.Fatalf("expected updated hash, got %v", retrieved)
	}
}

func TestPeekDoesNotUpdateRecency(t *testing.T) {
	cache := NewLedgerHeaderCache(2)
	cache.Put(makeHeader(1))
	cache.Put(makeHeader(2))

	// Peek at 1: should not make it MRU
	val, ok := cache.Peek(1)
	if !ok || val.Sequence != 1 {
		t.Fatal("expected to peek ledger 1")
	}

	// Order should still be: 2 (MRU), 1 (LRU)
	// Adding 3 should evict 1
	cache.Put(makeHeader(3))

	if cache.Contains(1) {
		t.Fatal("expected ledger 1 to be evicted because Peek did not update recency")
	}
	if !cache.Contains(2) || !cache.Contains(3) {
		t.Fatal("expected ledgers 2 and 3 to be retained")
	}
}

func TestEvictionCallback(t *testing.T) {
	var evictedSeq uint32
	var evictedHeader *rpc.LedgerHeaderResponse

	cache := NewLedgerHeaderCacheWithCallback(2, func(seq uint32, header *rpc.LedgerHeaderResponse) {
		evictedSeq = seq
		evictedHeader = header
	})

	cache.Put(makeHeader(10))
	cache.Put(makeHeader(20))
	cache.Put(makeHeader(30)) // Should evict 10

	if evictedSeq != 10 {
		t.Fatalf("expected evicted seq 10, got %d", evictedSeq)
	}
	if evictedHeader == nil || evictedHeader.Sequence != 10 {
		t.Fatal("expected evicted header for sequence 10")
	}
	if cache.EvictionCount() != 1 {
		t.Fatalf("expected eviction count 1, got %d", cache.EvictionCount())
	}
}

func TestRemoveAndPurge(t *testing.T) {
	cache := NewLedgerHeaderCache(5)
	cache.Put(makeHeader(1))
	cache.Put(makeHeader(2))
	cache.Put(makeHeader(3))

	if !cache.Remove(2) {
		t.Fatal("expected Remove(2) to return true")
	}
	if cache.Remove(99) {
		t.Fatal("expected Remove(99) to return false for nonexistent key")
	}
	if cache.Len() != 2 {
		t.Fatalf("expected len 2, got %d", cache.Len())
	}
	if cache.Contains(2) {
		t.Fatal("expected ledger 2 to be removed")
	}

	cache.Purge()
	if cache.Len() != 0 {
		t.Fatalf("expected len 0 after purge, got %d", cache.Len())
	}
	if cache.Contains(1) || cache.Contains(3) {
		t.Fatal("expected all items cleared after purge")
	}
}

func TestResize(t *testing.T) {
	cache := NewLedgerHeaderCache(5)
	for i := uint32(1); i <= 5; i++ {
		cache.Put(makeHeader(i))
	}

	// Resize to 3 entries: should evict 1 and 2
	evicted := cache.Resize(3)
	if evicted != 2 {
		t.Fatalf("expected 2 evictions, got %d", evicted)
	}
	if cache.Len() != 3 {
		t.Fatalf("expected len 3, got %d", cache.Len())
	}
	if cache.Capacity() != 3 {
		t.Fatalf("expected capacity 3, got %d", cache.Capacity())
	}
	if cache.Contains(1) || cache.Contains(2) {
		t.Fatal("expected ledgers 1 and 2 to be evicted after shrinking capacity")
	}
	if !cache.Contains(3) || !cache.Contains(4) || !cache.Contains(5) {
		t.Fatal("expected ledgers 3, 4, 5 to remain")
	}
}

func TestGetOrFetch(t *testing.T) {
	cache := NewLedgerHeaderCache(10)
	fetcher := newMockFetcher()
	ctx := context.Background()

	// 1. Fetch sequence 100: should call fetcher and cache
	h1, err := cache.GetOrFetch(ctx, fetcher, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1.Sequence != 100 {
		t.Fatalf("expected sequence 100, got %d", h1.Sequence)
	}
	if fetcher.callCount() != 1 {
		t.Fatalf("expected 1 fetcher call, got %d", fetcher.callCount())
	}

	// 2. Fetch sequence 100 again: should hit cache without calling fetcher
	h2, err := cache.GetOrFetch(ctx, fetcher, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h2.Sequence != 100 {
		t.Fatalf("expected sequence 100, got %d", h2.Sequence)
	}
	if fetcher.callCount() != 1 {
		t.Fatalf("expected fetcher call count to remain 1, got %d", fetcher.callCount())
	}

	// 3. Failed fetcher call: should not cache error
	fetcher.failSeq[200] = true
	_, err = cache.GetOrFetch(ctx, fetcher, 200)
	if err == nil {
		t.Fatal("expected error from failed fetcher")
	}
	if cache.Contains(200) {
		t.Fatal("failed fetch should not be cached")
	}
}

func TestCapAt10000Entries(t *testing.T) {
	// Verify that inserting 12,000 items strictly caps at 10,000 entries
	// and performs exactly 2,000 evictions.
	cache := NewLedgerHeaderCache() // Default capacity is 10,000

	totalInserts := 12000
	for i := uint32(1); i <= uint32(totalInserts); i++ {
		cache.Put(makeHeader(i))
	}

	if cache.Len() != 10000 {
		t.Fatalf("expected cache size strictly capped at 10000, got %d", cache.Len())
	}
	if cache.EvictionCount() != uint64(totalInserts-10000) {
		t.Fatalf("expected %d evictions, got %d", totalInserts-10000, cache.EvictionCount())
	}

	// The first 2,000 entries (1 to 2000) should have been evicted
	for i := uint32(1); i <= 2000; i++ {
		if cache.Contains(i) {
			t.Fatalf("expected sequence %d to be evicted", i)
		}
	}

	// Entries 2001 to 12000 must be present
	for i := uint32(2001); i <= uint32(totalInserts); i++ {
		if !cache.Contains(i) {
			t.Fatalf("expected sequence %d to be in cache", i)
		}
	}

	stats := cache.Stats()
	if stats.Size != 10000 {
		t.Fatalf("expected stats.Size 10000, got %d", stats.Size)
	}
	if stats.Capacity != 10000 {
		t.Fatalf("expected stats.Capacity 10000, got %d", stats.Capacity)
	}
	if stats.Evictions != 2000 {
		t.Fatalf("expected stats.Evictions 2000, got %d", stats.Evictions)
	}
}

func TestNilHeaderPut(t *testing.T) {
	cache := NewLedgerHeaderCache(5)
	if cache.Put(nil) {
		t.Fatal("Put(nil) should return false")
	}
	if cache.Len() != 0 {
		t.Fatalf("expected len 0, got %d", cache.Len())
	}
}

func TestGenericLRUCache(t *testing.T) {
	lru := NewLRU[string, int](3)
	lru.Put("a", 1)
	lru.Put("b", 2)
	lru.Put("c", 3)

	if val, ok := lru.Get("a"); !ok || val != 1 {
		t.Fatalf("expected to get a=1, got %d", val)
	}

	// Recency order: a (MRU), c, b (LRU)
	lru.Put("d", 4) // should evict b

	if lru.Contains("b") {
		t.Fatal("expected 'b' to be evicted")
	}

	oldestKey, oldestVal, ok := lru.Oldest()
	if !ok || oldestKey != "c" || oldestVal != 3 {
		t.Fatalf("expected oldest 'c'=3, got %s=%d", oldestKey, oldestVal)
	}

	newestKey, newestVal, ok := lru.Newest()
	if !ok || newestKey != "d" || newestVal != 4 {
		t.Fatalf("expected newest 'd'=4, got %s=%d", newestKey, newestVal)
	}

	keys := lru.Keys()
	expectedKeys := []string{"d", "a", "c"}
	if len(keys) != len(expectedKeys) {
		t.Fatalf("expected %d keys, got %d", len(expectedKeys), len(keys))
	}
	for i, k := range keys {
		if k != expectedKeys[i] {
			t.Fatalf("expected key at %d to be %s, got %s", i, expectedKeys[i], k)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewLedgerHeaderCache(100)
	var wg sync.WaitGroup
	workers := 16
	opsPerWorker := 500

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				seq := uint32((workerID*100 + i) % 300)
				switch i % 5 {
				case 0:
					cache.Put(makeHeader(seq))
				case 1:
					cache.Get(seq)
				case 2:
					cache.Peek(seq)
				case 3:
					cache.Contains(seq)
				case 4:
					if i%20 == 0 {
						cache.Remove(seq)
					}
				}
			}
		}(w)
	}

	wg.Wait()

	if cache.Len() > 100 {
		t.Fatalf("cache size exceeded capacity under concurrency: got %d", cache.Len())
	}
}

func TestPutWithSequenceAndValues(t *testing.T) {
	cache := NewLedgerHeaderCache(3)
	h1 := makeHeader(1)
	h2 := makeHeader(2)

	cache.PutWithSequence(1, h1)
	cache.PutWithSequence(2, h2)

	if cache.Len() != 2 {
		t.Fatalf("expected len 2, got %d", cache.Len())
	}

	values := cache.lru.Values()
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if values[0].Sequence != 2 || values[1].Sequence != 1 {
		t.Fatalf("expected values in MRU order [2, 1], got [%d, %d]", values[0].Sequence, values[1].Sequence)
	}

	// Test Oldest and Newest on empty cache
	emptyLRU := NewLRU[int, int](3)
	if _, _, ok := emptyLRU.Oldest(); ok {
		t.Fatal("expected Oldest on empty cache to return false")
	}
	if _, _, ok := emptyLRU.Newest(); ok {
		t.Fatal("expected Newest on empty cache to return false")
	}

	// Test Resize with non-positive value
	cache.Resize(-10)
	if cache.Capacity() != 1 {
		t.Fatalf("expected capacity clamped to 1, got %d", cache.Capacity())
	}
	if cache.Len() != 1 {
		t.Fatalf("expected len 1 after shrinking, got %d", cache.Len())
	}
}

func BenchmarkLedgerHeaderCache_Get(b *testing.B) {
	cache := NewLedgerHeaderCache(10000)
	for i := uint32(1); i <= 10000; i++ {
		cache.Put(makeHeader(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seq := uint32((i % 10000) + 1)
		_, _ = cache.Get(seq)
	}
}

func BenchmarkLedgerHeaderCache_Put(b *testing.B) {
	cache := NewLedgerHeaderCache(10000)
	header := makeHeader(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header.Sequence = uint32(i)
		cache.Put(header)
	}
}
