// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package debug

import "sync"

const (
	// DefaultMaxSnapshots bounds the number of snapshots kept in memory.
	DefaultMaxSnapshots = 500
)

// SnapshotRegistry stores and retrieves in-memory snapshots for active sessions.
type SnapshotRegistry interface {
	Store(sessionID string, timestamp int64, snap Entry)
	Get(sessionID string, timestamp int64) (Entry, bool)
	GetSession(sessionID string) ([]Entry, bool)
	DeleteSession(sessionID string)
	Len() int
}

// inMemorySession keeps insertion order to support deterministic purging.
type inMemorySession struct {
	entries map[int64]Entry
	order   []int64
}

// InMemoryRegistry is an in-memory implementation of SnapshotRegistry.
type InMemoryRegistry struct {
	mu           sync.RWMutex
	maxSnapshots int
	total        int
	sessions     map[string]*inMemorySession
	sessionOrder []string
}

// NewSnapshotRegistry returns a thread-safe in-memory snapshot registry.
func NewSnapshotRegistry(maxSnapshots int) *InMemoryRegistry {
	if maxSnapshots <= 0 {
		maxSnapshots = DefaultMaxSnapshots
	}

	return &InMemoryRegistry{
		maxSnapshots: maxSnapshots,
		sessions:     make(map[string]*inMemorySession),
		sessionOrder: make([]string, 0),
	}
}

// Store saves or replaces a snapshot for a session/timestamp pair.
func (r *InMemoryRegistry) Store(sessionID string, timestamp int64, snap Entry) {
	if sessionID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[sessionID]
	if !ok {
		sess = &inMemorySession{
			entries: make(map[int64]Entry),
			order:   make([]int64, 0),
		}
		r.sessions[sessionID] = sess
		r.sessionOrder = append(r.sessionOrder, sessionID)
	}

	if _, exists := sess.entries[timestamp]; !exists {
		sess.order = append(sess.order, timestamp)
		r.total++
	}

	sess.entries[timestamp] = snap
	r.purgeIfNeeded()
}

// Get retrieves a single snapshot by session/timestamp.
func (r *InMemoryRegistry) Get(sessionID string, timestamp int64) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sess, ok := r.sessions[sessionID]
	if !ok {
		return Entry{}, false
	}

	entry, found := sess.entries[timestamp]
	if !found {
		return Entry{}, false
	}

	return entry, true
}

// GetSession returns all snapshots for a session in insertion order.
func (r *InMemoryRegistry) GetSession(sessionID string) ([]Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sess, ok := r.sessions[sessionID]
	if !ok {
		return nil, false
	}

	entries := make([]Entry, 0, len(sess.order))
	for _, ts := range sess.order {
		if entry, exists := sess.entries[ts]; exists {
			entries = append(entries, entry)
		}
	}

	return entries, true
}

// DeleteSession removes all snapshots for a session.
func (r *InMemoryRegistry) DeleteSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	r.total -= len(sess.entries)
	delete(r.sessions, sessionID)

	for i, id := range r.sessionOrder {
		if id == sessionID {
			r.sessionOrder = append(r.sessionOrder[:i], r.sessionOrder[i+1:]...)
			break
		}
	}
}

// Len returns the total number of snapshots currently in memory.
func (r *InMemoryRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.total
}

func (r *InMemoryRegistry) purgeIfNeeded() {
	for r.total > r.maxSnapshots && len(r.sessionOrder) > 0 {
		sessionID := r.sessionOrder[0]
		sess, ok := r.sessions[sessionID]
		if !ok {
			r.sessionOrder = r.sessionOrder[1:]
			continue
		}

		if len(sess.order) == 0 {
			delete(r.sessions, sessionID)
			r.sessionOrder = r.sessionOrder[1:]
			continue
		}

		oldestTimestamp := sess.order[0]
		sess.order = sess.order[1:]
		if _, exists := sess.entries[oldestTimestamp]; exists {
			delete(sess.entries, oldestTimestamp)
			r.total--
		}

		if len(sess.entries) == 0 {
			delete(r.sessions, sessionID)
			r.sessionOrder = r.sessionOrder[1:]
		}
	}
}
