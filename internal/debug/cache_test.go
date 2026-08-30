// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package debug

import (
	"fmt"
	"sync"
	"testing"

	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRegistry_StoreAndGet(t *testing.T) {
	r := NewSnapshotRegistry(10)

	ts := int64(123)
	expected := Entry{
		Timestamp: ts,
		Snapshot: snapshot.FromMap(map[string]string{
			"key-a": "val-a",
		}),
	}

	r.Store("session-1", ts, expected)

	actual, ok := r.Get("session-1", ts)
	require.True(t, ok)
	require.Equal(t, expected.Timestamp, actual.Timestamp)
	require.Equal(t, expected.Snapshot.ToMap(), actual.Snapshot.ToMap())

	_, missing := r.Get("session-1", 999)
	require.False(t, missing)
}

func TestInMemoryRegistry_GetSessionReturnsInsertionOrder(t *testing.T) {
	r := NewSnapshotRegistry(10)

	r.Store("session-1", 300, Entry{Timestamp: 300, Snapshot: snapshot.FromMap(map[string]string{"k3": "v3"})})
	r.Store("session-1", 100, Entry{Timestamp: 100, Snapshot: snapshot.FromMap(map[string]string{"k1": "v1"})})
	r.Store("session-1", 200, Entry{Timestamp: 200, Snapshot: snapshot.FromMap(map[string]string{"k2": "v2"})})

	entries, ok := r.GetSession("session-1")
	require.True(t, ok)
	require.Len(t, entries, 3)
	require.Equal(t, int64(300), entries[0].Timestamp)
	require.Equal(t, int64(100), entries[1].Timestamp)
	require.Equal(t, int64(200), entries[2].Timestamp)
}

func TestInMemoryRegistry_ReplacingSameTimestampDoesNotGrow(t *testing.T) {
	r := NewSnapshotRegistry(10)

	r.Store("session-1", 42, Entry{Timestamp: 42, Snapshot: snapshot.FromMap(map[string]string{"k": "v1"})})
	r.Store("session-1", 42, Entry{Timestamp: 42, Snapshot: snapshot.FromMap(map[string]string{"k": "v2"})})

	require.Equal(t, 1, r.Len())

	entry, ok := r.Get("session-1", 42)
	require.True(t, ok)
	require.Equal(t, "v2", entry.Snapshot.ToMap()["k"])
}

func TestInMemoryRegistry_SizeLimitPurgesOldestSnapshots(t *testing.T) {
	r := NewSnapshotRegistry(5)

	for i := 0; i < 7; i++ {
		ts := int64(i)
		r.Store("session-1", ts, Entry{
			Timestamp: ts,
			Snapshot: snapshot.FromMap(map[string]string{
				fmt.Sprintf("k-%d", i): fmt.Sprintf("v-%d", i),
			}),
		})
	}

	require.Equal(t, 5, r.Len())

	_, ok0 := r.Get("session-1", 0)
	_, ok1 := r.Get("session-1", 1)
	require.False(t, ok0)
	require.False(t, ok1)

	for i := 2; i < 7; i++ {
		_, ok := r.Get("session-1", int64(i))
		require.True(t, ok)
	}
}

func TestInMemoryRegistry_MemoryBoundWithMockData(t *testing.T) {
	maxSnapshots := 500
	r := NewSnapshotRegistry(maxSnapshots)

	for i := 0; i < 5000; i++ {
		ts := int64(i)
		r.Store("session-a", ts, Entry{
			Timestamp: ts,
			Snapshot: snapshot.FromMap(map[string]string{
				fmt.Sprintf("ledger-key-%d", i): fmt.Sprintf("ledger-value-%d", i),
			}),
		})
	}

	require.Equal(t, maxSnapshots, r.Len())
	entries, ok := r.GetSession("session-a")
	require.True(t, ok)
	require.Len(t, entries, maxSnapshots)
}

func TestInMemoryRegistry_ConcurrentStoreAndGet(t *testing.T) {
	r := NewSnapshotRegistry(DefaultMaxSnapshots)

	const workers = 8
	const perWorker = 100

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		worker := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", worker%2)
			for i := 0; i < perWorker; i++ {
				ts := int64(worker*perWorker + i)
				entry := Entry{
					Timestamp: ts,
					Snapshot: snapshot.FromMap(map[string]string{
						fmt.Sprintf("k-%d", ts): "v",
					}),
				}
				r.Store(sessionID, ts, entry)
				if got, ok := r.Get(sessionID, ts); ok {
					require.Equal(t, ts, got.Timestamp)
				}
			}
		}()
	}
	wg.Wait()

	require.LessOrEqual(t, r.Len(), DefaultMaxSnapshots)
}
