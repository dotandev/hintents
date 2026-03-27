// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package debug provides utilities for inspecting and replaying session state,
// including a centralized registry for snapshots fetched during a session.
package debug

import (
	"sync"

	"github.com/dotandev/hintents/internal/snapshot"
)

// SnapshotRegistry defines a centralized store for snapshots fetched during a
// session. Implementations must be safe for concurrent use.
type SnapshotRegistry interface {
	// Add stores a snapshot under the given id, overwriting any existing entry.
	Add(id string, snap *snapshot.Snapshot)

	// Get retrieves the snapshot associated with id.
	// Returns the snapshot and true if found, or nil and false otherwise.
	Get(id string) (*snapshot.Snapshot, bool)

	// ListIDs returns the ids of all stored snapshots in insertion order.
	ListIDs() []string

	// Clear removes all snapshots from the registry.
	Clear()
}

// snapshotRegistry is the default in-memory implementation of SnapshotRegistry.
type snapshotRegistry struct {
	mu       sync.RWMutex
	entries  map[string]*snapshot.Snapshot
	ordering []string
}

// NewSnapshotRegistry returns a new, empty SnapshotRegistry backed by an
// in-memory store.
func NewSnapshotRegistry() SnapshotRegistry {
	return &snapshotRegistry{
		entries: make(map[string]*snapshot.Snapshot),
	}
}

// Add stores snap under id. If id already exists, the snapshot is replaced and
// its position in the ordering is unchanged.
func (r *snapshotRegistry) Add(id string, snap *snapshot.Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[id]; !exists {
		r.ordering = append(r.ordering, id)
	}
	r.entries[id] = snap
}

// Get retrieves the snapshot stored under id.
func (r *snapshotRegistry) Get(id string) (*snapshot.Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap, ok := r.entries[id]
	return snap, ok
}

// ListIDs returns all stored ids in insertion order.
func (r *snapshotRegistry) ListIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, len(r.ordering))
	copy(ids, r.ordering)
	return ids
}

// Clear removes all snapshots and resets the registry to an empty state.
func (r *snapshotRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = make(map[string]*snapshot.Snapshot)
	r.ordering = r.ordering[:0]
}
