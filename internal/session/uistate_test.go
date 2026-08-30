// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUIStore(t *testing.T) *UIStateStore {
	t.Helper()
	s, err := newUIStateStoreAt(filepath.Join(t.TempDir(), "ui_state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSaveSectionState(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.SaveSectionState(ctx, "abc123", []string{"events", "logs", "budget"}))

	got, err := s.LoadSectionState(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, []string{"events", "logs", "budget"}, got)
}

func TestLoadSectionState_NoEntry(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	got, err := s.LoadSectionState(ctx, "notfound")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSaveSectionState_OverwritesPrevious(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.SaveSectionState(ctx, "abc123", []string{"events"}))
	require.NoError(t, s.SaveSectionState(ctx, "abc123", []string{"events", "security"}))

	got, err := s.LoadSectionState(ctx, "abc123")
	require.NoError(t, err)
	assert.Equal(t, []string{"events", "security"}, got)
}

func TestSaveSectionState_IndependentPerTxHash(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.SaveSectionState(ctx, "tx-aaa", []string{"budget"}))
	require.NoError(t, s.SaveSectionState(ctx, "tx-bbb", []string{"logs", "tokenflow"}))

	a, err := s.LoadSectionState(ctx, "tx-aaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"budget"}, a)

	b, err := s.LoadSectionState(ctx, "tx-bbb")
	require.NoError(t, err)
	assert.Equal(t, []string{"logs", "tokenflow"}, b)
}

func TestAppendRecentSearch(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.AppendRecentSearch(ctx, "transfer"))
	require.NoError(t, s.AppendRecentSearch(ctx, "mint"))
	require.NoError(t, s.AppendRecentSearch(ctx, "burn"))

	queries, err := s.RecentSearches(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"burn", "mint", "transfer"}, queries)
}

func TestAppendRecentSearch_Deduplication(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.AppendRecentSearch(ctx, "transfer"))
	require.NoError(t, s.AppendRecentSearch(ctx, "mint"))
	require.NoError(t, s.AppendRecentSearch(ctx, "transfer"))

	queries, err := s.RecentSearches(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"transfer", "mint"}, queries)
}

func TestAppendRecentSearch_TrimToMax(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	for i := 0; i < maxRecentSearches+5; i++ {
		require.NoError(t, s.AppendRecentSearch(ctx, fmt.Sprintf("query-%02d", i)))
	}

	queries, err := s.RecentSearches(ctx, 100)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(queries), maxRecentSearches)
}

func TestRecentSearches_Empty(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	queries, err := s.RecentSearches(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, queries)
}

func TestAppendRecentSearch_EmptyQueryIgnored(t *testing.T) {
	ctx := context.Background()
	s := newTestUIStore(t)

	require.NoError(t, s.AppendRecentSearch(ctx, ""))

	queries, err := s.RecentSearches(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, queries)
}
