// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"testing"
)

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
