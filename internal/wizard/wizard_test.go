// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wizard

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAccountAddress = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF"

type stubRenderer struct {
	*terminal.MockRenderer
	choice  int
	scanErr error
}

func newStubRenderer(choice int) *stubRenderer {
	return &stubRenderer{
		MockRenderer: terminal.NewMockRenderer(),
		choice:       choice,
	}
}

func (s *stubRenderer) Scanln(a ...any) (int, error) {
	if s.scanErr != nil {
		return 0, s.scanErr
	}
	if len(a) > 0 {
		if ptr, ok := a[0].(*int); ok {
			*ptr = s.choice
		}
	}
	return 1, nil
}

type mockHorizonTx struct {
	ID         string `json:"id"`
	Hash       string `json:"hash"`
	Successful bool   `json:"successful"`
	CreatedAt  string `json:"created_at"`
}

func newMockHorizonTransactionsServer(t *testing.T, records []mockHorizonTx) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/accounts/"+testAccountAddress+"/transactions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		pageRecords := records
		if r.URL.Query().Get("cursor") == "next" {
			pageRecords = []mockHorizonTx{}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"_links": map[string]any{
				"self": map[string]any{"href": server.URL + r.URL.RequestURI()},
				"next": map[string]any{"href": server.URL + "/accounts/" + testAccountAddress + "/transactions?cursor=next"},
				"prev": map[string]any{"href": ""},
			},
			"_embedded": map[string]any{
				"records": pageRecords,
			},
		})
	}))
	return server
}

func TestIsFailed(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{name: "exact failed", status: "failed", expected: true},
		{name: "contains failed", status: "tx_failed (bad_seq)", expected: true},
		{name: "success", status: "success", expected: false},
		{name: "empty", status: "", expected: false},
		{name: "uppercase FAILED", status: "FAILED", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isFailed(rpc.TransactionSummary{Status: tc.status}))
		})
	}
}

func TestFilter(t *testing.T) {
	tx1 := rpc.TransactionSummary{Hash: "tx1", Status: "success"}
	tx2 := rpc.TransactionSummary{Hash: "tx2", Status: "failed"}
	tx3 := rpc.TransactionSummary{Hash: "tx3", Status: "tx_failed"}
	tx4 := rpc.TransactionSummary{Hash: "tx4", Status: "success"}

	t.Run("filters failed transactions", func(t *testing.T) {
		got := filter([]rpc.TransactionSummary{tx1, tx2, tx3, tx4}, isFailed)
		assert.Equal(t, []rpc.TransactionSummary{tx2, tx3}, got)
	})

	t.Run("returns empty slice when none match", func(t *testing.T) {
		got := filter([]rpc.TransactionSummary{tx1, tx4}, isFailed)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("handles empty input", func(t *testing.T) {
		got := filter(nil, isFailed)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}

func TestTruncateHash(t *testing.T) {
	assert.Equal(t, "short_hash", truncateHash("short_hash"))
	assert.Equal(t, "0123456789abcdef", truncateHash("0123456789abcdef"))
	assert.Equal(t, "0123456789abcdef...", truncateHash("0123456789abcdef0123"))
}

func TestSelectTransaction_EmptyAccount(t *testing.T) {
	w := New(nil).WithRenderer(terminal.NewMockRenderer())

	res, err := w.SelectTransaction(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, errors.ErrValidationFailed))
	assert.Contains(t, err.Error(), "account address required")
}

func TestSelectTransaction_NoTransactions(t *testing.T) {
	server := newMockHorizonTransactionsServer(t, []mockHorizonTx{})
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	w := New(client).WithRenderer(terminal.NewMockRenderer())
	res, err := w.SelectTransaction(context.Background(), testAccountAddress)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, errors.ErrTransactionNotFound))
	assert.Contains(t, err.Error(), "no transactions found for account")
}

func TestSelectTransaction_NoFailedTransactions_ShowsAll(t *testing.T) {
	tx1Hash := "1111111111111111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tx2Hash := "2222222222222222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	server := newMockHorizonTransactionsServer(t, []mockHorizonTx{
		{ID: tx1Hash, Hash: tx1Hash, Successful: true, CreatedAt: "2026-01-02T15:04:05Z"},
		{ID: tx2Hash, Hash: tx2Hash, Successful: true, CreatedAt: "2026-01-02T15:05:05Z"},
	})
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	renderer := newStubRenderer(2)
	w := New(client).WithRenderer(renderer)

	res, err := w.SelectTransaction(context.Background(), testAccountAddress)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, tx2Hash, res.Hash)
	assert.Equal(t, "success", res.Status)
	assert.Equal(t, "2026-01-02 15:05:05", res.CreatedAt)

	output := renderer.AllOutput()
	assert.Contains(t, output, "No failed transactions found. Showing all recent transactions:")
	assert.Contains(t, output, "1111111111111111...")
	assert.Contains(t, output, "2222222222222222...")
}

func TestSelectTransaction_FiltersFailedTransactions(t *testing.T) {
	successHash := "aaaaaaaaaaaaaaaa111111111111111111111111111111111111111111111111"
	failedHash := "bbbbbbbbbbbbbbbb222222222222222222222222222222222222222222222222"

	server := newMockHorizonTransactionsServer(t, []mockHorizonTx{
		{ID: successHash, Hash: successHash, Successful: true, CreatedAt: "2026-01-02T15:04:05Z"},
		{ID: failedHash, Hash: failedHash, Successful: false, CreatedAt: "2026-01-02T15:05:05Z"},
	})
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	renderer := newStubRenderer(1)
	w := New(client).WithRenderer(renderer)

	res, err := w.SelectTransaction(context.Background(), testAccountAddress)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, failedHash, res.Hash)
	assert.Equal(t, "failed", res.Status)

	output := renderer.AllOutput()
	assert.NotContains(t, output, "No failed transactions found")
	assert.NotContains(t, output, "aaaaaaaaaaaaaaaa...")
	assert.Contains(t, output, "[1] failed | bbbbbbbbbbbbbbbb...")
}

func TestSelectTransaction_CancelledSelection(t *testing.T) {
	failedHash := "cccccccccccccccc333333333333333333333333333333333333333333333333"
	server := newMockHorizonTransactionsServer(t, []mockHorizonTx{
		{ID: failedHash, Hash: failedHash, Successful: false, CreatedAt: "2026-01-02T15:05:05Z"},
	})
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	t.Run("out of range selection", func(t *testing.T) {
		w := New(client).WithRenderer(terminal.NewMockRenderer())

		res, err := w.SelectTransaction(context.Background(), testAccountAddress)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, errors.ErrValidationFailed))
		assert.Contains(t, err.Error(), "transaction selection cancelled")
	})

	t.Run("scan error", func(t *testing.T) {
		renderer := newStubRenderer(1)
		renderer.scanErr = stdErrors.New("eof")
		w := New(client).WithRenderer(renderer)

		res, err := w.SelectTransaction(context.Background(), testAccountAddress)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, errors.ErrValidationFailed))
		assert.Contains(t, err.Error(), "transaction selection cancelled")
	})
}

func TestSelectTransaction_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":500,"title":"internal error"}`))
	}))
	defer server.Close()

	client, err := rpc.NewClient(rpc.WithHorizonURL(server.URL))
	require.NoError(t, err)

	w := New(client).WithRenderer(terminal.NewMockRenderer())
	res, err := w.SelectTransaction(context.Background(), testAccountAddress)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, errors.ErrRPCConnectionFailed))
}
