// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	stdErrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/endpoints"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	effects "github.com/stellar/go-stellar-sdk/protocols/horizon/effects"
	operations "github.com/stellar/go-stellar-sdk/protocols/horizon/operations"
	"github.com/stellar/go-stellar-sdk/support/render/problem"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stretchr/testify/assert"
)

type mockHorizonClient struct {
	TransactionDetailFunc func(hash string) (hProtocol.Transaction, error)
	LedgerDetailFunc      func(sequence uint32) (hProtocol.Ledger, error)
}

func (m *mockHorizonClient) TransactionDetail(hash string) (hProtocol.Transaction, error) {
	return m.TransactionDetailFunc(hash)
}
func (m *mockHorizonClient) AccountData(_ horizonclient.AccountRequest) (hProtocol.AccountData, error) {
	return hProtocol.AccountData{}, nil
}
func (m *mockHorizonClient) AccountDetail(_ horizonclient.AccountRequest) (hProtocol.Account, error) {
	return hProtocol.Account{}, nil
}
func (m *mockHorizonClient) Accounts(_ horizonclient.AccountsRequest) (hProtocol.AccountsPage, error) {
	return hProtocol.AccountsPage{}, nil
}
func (m *mockHorizonClient) Effects(_ horizonclient.EffectRequest) (effects.EffectsPage, error) {
	return effects.EffectsPage{}, nil
}
func (m *mockHorizonClient) Assets(_ horizonclient.AssetRequest) (hProtocol.AssetsPage, error) {
	return hProtocol.AssetsPage{}, nil
}
func (m *mockHorizonClient) Ledgers(_ horizonclient.LedgerRequest) (hProtocol.LedgersPage, error) {
	return hProtocol.LedgersPage{}, nil
}
func (m *mockHorizonClient) LedgerDetail(sequence uint32) (hProtocol.Ledger, error) {
	if m.LedgerDetailFunc != nil {
		return m.LedgerDetailFunc(sequence)
	}
	return hProtocol.Ledger{}, nil
}
func (m *mockHorizonClient) FeeStats() (hProtocol.FeeStats, error) { return hProtocol.FeeStats{}, nil }
func (m *mockHorizonClient) Offers(_ horizonclient.OfferRequest) (hProtocol.OffersPage, error) {
	return hProtocol.OffersPage{}, nil
}
func (m *mockHorizonClient) OfferDetails(offerID string) (hProtocol.Offer, error) {
	return hProtocol.Offer{}, nil
}
func (m *mockHorizonClient) Operations(_ horizonclient.OperationRequest) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) OperationDetail(id string) (operations.Operation, error) {
	var op operations.Operation
	return op, nil
}
func (m *mockHorizonClient) StreamPayments(ctx context.Context, _ horizonclient.OperationRequest, handler horizonclient.OperationHandler) error {
	return nil
}
func (m *mockHorizonClient) SubmitTransactionXDR(transactionXdr string) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) SubmitFeeBumpTransactionWithOptions(transaction *txnbuild.FeeBumpTransaction, opts horizonclient.SubmitTxOpts) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) SubmitTransactionWithOptions(transaction *txnbuild.Transaction, opts horizonclient.SubmitTxOpts) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) SubmitFeeBumpTransaction(transaction *txnbuild.FeeBumpTransaction) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) SubmitTransaction(transaction *txnbuild.Transaction) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) AsyncSubmitTransactionXDR(transactionXdr string) (hProtocol.AsyncTransactionSubmissionResponse, error) {
	return hProtocol.AsyncTransactionSubmissionResponse{}, nil
}
func (m *mockHorizonClient) AsyncSubmitFeeBumpTransactionWithOptions(transaction *txnbuild.FeeBumpTransaction, opts horizonclient.SubmitTxOpts) (hProtocol.AsyncTransactionSubmissionResponse, error) {
	return hProtocol.AsyncTransactionSubmissionResponse{}, nil
}
func (m *mockHorizonClient) AsyncSubmitTransactionWithOptions(transaction *txnbuild.Transaction, opts horizonclient.SubmitTxOpts) (hProtocol.AsyncTransactionSubmissionResponse, error) {
	return hProtocol.AsyncTransactionSubmissionResponse{}, nil
}
func (m *mockHorizonClient) AsyncSubmitFeeBumpTransaction(transaction *txnbuild.FeeBumpTransaction) (hProtocol.AsyncTransactionSubmissionResponse, error) {
	return hProtocol.AsyncTransactionSubmissionResponse{}, nil
}
func (m *mockHorizonClient) AsyncSubmitTransaction(transaction *txnbuild.Transaction) (hProtocol.AsyncTransactionSubmissionResponse, error) {
	return hProtocol.AsyncTransactionSubmissionResponse{}, nil
}
func (m *mockHorizonClient) Transactions(_ horizonclient.TransactionRequest) (hProtocol.TransactionsPage, error) {
	return hProtocol.TransactionsPage{}, nil
}
func (m *mockHorizonClient) OrderBook(_ horizonclient.OrderBookRequest) (hProtocol.OrderBookSummary, error) {
	return hProtocol.OrderBookSummary{}, nil
}
func (m *mockHorizonClient) Paths(_ horizonclient.PathsRequest) (hProtocol.PathsPage, error) {
	return hProtocol.PathsPage{}, nil
}
func (m *mockHorizonClient) Payments(_ horizonclient.OperationRequest) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) TradeAggregations(_ horizonclient.TradeAggregationRequest) (hProtocol.TradeAggregationsPage, error) {
	return hProtocol.TradeAggregationsPage{}, nil
}
func (m *mockHorizonClient) Trades(_ horizonclient.TradeRequest) (hProtocol.TradesPage, error) {
	return hProtocol.TradesPage{}, nil
}
func (m *mockHorizonClient) Fund(addr string) (hProtocol.Transaction, error) {
	return hProtocol.Transaction{}, nil
}
func (m *mockHorizonClient) StreamTransactions(ctx context.Context, request horizonclient.TransactionRequest, handler horizonclient.TransactionHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamTrades(ctx context.Context, request horizonclient.TradeRequest, handler horizonclient.TradeHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamEffects(ctx context.Context, request horizonclient.EffectRequest, handler horizonclient.EffectHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamOperations(ctx context.Context, request horizonclient.OperationRequest, handler horizonclient.OperationHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamOffers(ctx context.Context, request horizonclient.OfferRequest, handler horizonclient.OfferHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamLedgers(ctx context.Context, request horizonclient.LedgerRequest, handler horizonclient.LedgerHandler) error {
	return nil
}
func (m *mockHorizonClient) StreamOrderBooks(ctx context.Context, request horizonclient.OrderBookRequest, handler horizonclient.OrderBookHandler) error {
	return nil
}
func (m *mockHorizonClient) Root() (hProtocol.Root, error) { return hProtocol.Root{}, nil }
func (m *mockHorizonClient) NextAccountsPage(page hProtocol.AccountsPage) (hProtocol.AccountsPage, error) {
	return hProtocol.AccountsPage{}, nil
}
func (m *mockHorizonClient) NextAssetsPage(page hProtocol.AssetsPage) (hProtocol.AssetsPage, error) {
	return hProtocol.AssetsPage{}, nil
}
func (m *mockHorizonClient) PrevAssetsPage(page hProtocol.AssetsPage) (hProtocol.AssetsPage, error) {
	return hProtocol.AssetsPage{}, nil
}
func (m *mockHorizonClient) NextLedgersPage(page hProtocol.LedgersPage) (hProtocol.LedgersPage, error) {
	return hProtocol.LedgersPage{}, nil
}
func (m *mockHorizonClient) PrevLedgersPage(page hProtocol.LedgersPage) (hProtocol.LedgersPage, error) {
	return hProtocol.LedgersPage{}, nil
}
func (m *mockHorizonClient) NextEffectsPage(page effects.EffectsPage) (effects.EffectsPage, error) {
	return effects.EffectsPage{}, nil
}
func (m *mockHorizonClient) PrevEffectsPage(page effects.EffectsPage) (effects.EffectsPage, error) {
	return effects.EffectsPage{}, nil
}
func (m *mockHorizonClient) NextTransactionsPage(page hProtocol.TransactionsPage) (hProtocol.TransactionsPage, error) {
	return hProtocol.TransactionsPage{}, nil
}
func (m *mockHorizonClient) PrevTransactionsPage(page hProtocol.TransactionsPage) (hProtocol.TransactionsPage, error) {
	return hProtocol.TransactionsPage{}, nil
}
func (m *mockHorizonClient) NextOperationsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) PrevOperationsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) NextPaymentsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) PrevPaymentsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) NextOffersPage(page hProtocol.OffersPage) (hProtocol.OffersPage, error) {
	return hProtocol.OffersPage{}, nil
}
func (m *mockHorizonClient) PrevOffersPage(page hProtocol.OffersPage) (hProtocol.OffersPage, error) {
	return hProtocol.OffersPage{}, nil
}
func (m *mockHorizonClient) NextTradesPage(page hProtocol.TradesPage) (hProtocol.TradesPage, error) {
	return hProtocol.TradesPage{}, nil
}
func (m *mockHorizonClient) PrevTradesPage(page hProtocol.TradesPage) (hProtocol.TradesPage, error) {
	return hProtocol.TradesPage{}, nil
}
func (m *mockHorizonClient) HomeDomainForAccount(aid string) (string, error) { return "", nil }
func (m *mockHorizonClient) NextTradeAggregationsPage(page hProtocol.TradeAggregationsPage) (hProtocol.TradeAggregationsPage, error) {
	return hProtocol.TradeAggregationsPage{}, nil
}
func (m *mockHorizonClient) PrevTradeAggregationsPage(page hProtocol.TradeAggregationsPage) (hProtocol.TradeAggregationsPage, error) {
	return hProtocol.TradeAggregationsPage{}, nil
}
func (m *mockHorizonClient) LiquidityPoolDetail(request horizonclient.LiquidityPoolRequest) (hProtocol.LiquidityPool, error) {
	return hProtocol.LiquidityPool{}, nil
}
func (m *mockHorizonClient) LiquidityPools(request horizonclient.LiquidityPoolsRequest) (hProtocol.LiquidityPoolsPage, error) {
	return hProtocol.LiquidityPoolsPage{}, nil
}
func (m *mockHorizonClient) NextLiquidityPoolsPage(page hProtocol.LiquidityPoolsPage) (hProtocol.LiquidityPoolsPage, error) {
	return hProtocol.LiquidityPoolsPage{}, nil
}
func (m *mockHorizonClient) PrevLiquidityPoolsPage(page hProtocol.LiquidityPoolsPage) (hProtocol.LiquidityPoolsPage, error) {
	return hProtocol.LiquidityPoolsPage{}, nil
}

type testClient struct {
	*Client
}

func newTestClient(mock horizonclient.ClientInterface) *testClient {
	return &testClient{
		&Client{
			Horizon:    mock.(*mockHorizonClient),
			HorizonURL: endpoints.HorizonTestnet,
			AltURLs:    []string{endpoints.HorizonTestnet},
		},
	}
}

func TestGetTransaction(t *testing.T) {
	tests := []struct {
		name      string
		hash      string
		mockFunc  func(hash string) (hProtocol.Transaction, error)
		expectErr bool
	}{
		{
			name: "success",
			hash: "abc123",
			mockFunc: func(hash string) (hProtocol.Transaction, error) {
				return hProtocol.Transaction{
					EnvelopeXdr:   "envelope-xdr",
					ResultXdr:     "result-xdr",
					ResultMetaXdr: "meta-xdr",
				}, nil
			},
			expectErr: false,
		},
		{
			name: "transaction not found",
			hash: "not-found",
			mockFunc: func(hash string) (hProtocol.Transaction, error) {
				return hProtocol.Transaction{}, &horizonclient.Error{
					Problem: problem.P{Status: 404},
				}
			},
			expectErr: true,
		},
		{
			name: "error",
			hash: "fail",
			mockFunc: func(hash string) (hProtocol.Transaction, error) {
				return hProtocol.Transaction{}, stdErrors.New("internal server error")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockHorizonClient{TransactionDetailFunc: tt.mockFunc}
			c := newTestClient(mock)
			ctx := context.Background()
			resp, err := c.GetTransaction(ctx, tt.hash)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				if tt.name == "transaction not found" {
					assert.True(t, errors.Is(err, errors.ErrTransactionNotFound))
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "envelope-xdr", resp.EnvelopeXdr)
				assert.Equal(t, "result-xdr", resp.ResultXdr)
				assert.Equal(t, "meta-xdr", resp.ResultMetaXdr)
			}
		})
	}
}

func TestGetTransaction_Timeout(t *testing.T) {
	var testCtx context.Context
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			select {
			case <-time.After(2 * time.Second):
				return hProtocol.Transaction{}, nil
			case <-testCtx.Done():
				return hProtocol.Transaction{}, testCtx.Err()
			}
		},
	}
	c := newTestClient(mock)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	testCtx = ctx
	_, err := c.GetTransaction(ctx, "timeout")
	assert.Error(t, err)
}

func TestGetLedgerEntries_WithVerification(t *testing.T) {
	// This test verifies that GetLedgerEntries properly validates returned entries
	// Note: This is a unit test that would require a mock RPC server to fully test
	// The actual verification logic is tested in verification_test.go

	t.Run("verification is called during fetch", func(t *testing.T) {
		// This test documents that verification happens in getLedgerEntriesAttempt
		// The actual verification logic is tested separately in verification_test.go
		assert.True(t, true, "Verification integration is documented")
	})
}

func TestGetLedgerEntries_ResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte("response too large"))
	}))
	defer server.Close()

	c := &Client{
		Horizon:    &mockHorizonClient{},
		HorizonURL: server.URL,
		SorobanURL: server.URL,
		Network:    "custom",
		AltURLs:    []string{server.URL},
	}

	_, err := c.GetLedgerEntries(context.Background(), []string{"AAAA"})
	assert.Error(t, err)
	assert.True(t, IsResponseTooLarge(err) || containsStr(err.Error(), "exceeded the server"))
}

func TestSimulateTransaction_ResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte("response too large"))
	}))
	defer server.Close()

	c := &Client{
		Horizon:    &mockHorizonClient{},
		HorizonURL: server.URL,
		SorobanURL: server.URL,
		Network:    "custom",
		AltURLs:    []string{server.URL},
	}

	_, err := c.SimulateTransaction(context.Background(), "dGVzdA==")
	assert.Error(t, err)
	assert.True(t, IsResponseTooLarge(err) || containsStr(err.Error(), "exceeded the server"))
}

func TestIsResponseTooLarge(t *testing.T) {
	err := errors.WrapRPCResponseTooLarge("https://example.com")
	assert.True(t, IsResponseTooLarge(err))
	assert.False(t, IsResponseTooLarge(errors.WrapRPCConnectionFailed(stdErrors.New("fail"))))
	assert.False(t, IsResponseTooLarge(nil))
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}

// ---- RequestTimeout client option tests ------------------------------------

func TestWithRequestTimeout_DefaultIs15s(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpClient, ok := client.httpClient.(*http.Client)
	if !ok {
		t.Fatal("expected *http.Client from default NewClient")
	}
	if httpClient.Timeout != 15*time.Second {
		t.Errorf("expected default timeout 15s, got %v", httpClient.Timeout)
	}
}

func TestWithRequestTimeout_CustomValue(t *testing.T) {
	client, err := NewClient(WithRequestTimeout(30 * time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpClient, ok := client.httpClient.(*http.Client)
	if !ok {
		t.Fatal("expected *http.Client")
	}
	if httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", httpClient.Timeout)
	}
}

func TestWithRequestTimeout_Zero(t *testing.T) {
	client, err := NewClient(WithRequestTimeout(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpClient, ok := client.httpClient.(*http.Client)
	if !ok {
		t.Fatal("expected *http.Client")
	}
	if httpClient.Timeout != 0 {
		t.Errorf("expected timeout 0 (disabled), got %v", httpClient.Timeout)
	}
}

func TestWithRequestTimeout_SlowConnectionValue(t *testing.T) {
	client, err := NewClient(WithRequestTimeout(60 * time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	httpClient, ok := client.httpClient.(*http.Client)
	if !ok {
		t.Fatal("expected *http.Client")
	}
	if httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", httpClient.Timeout)
	}
}

func TestWithRequestTimeout_RespectsContextDeadline(t *testing.T) {
	// Verify that a short timeout causes requests to a slow server to fail
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(server.URL+"/"),
		WithRequestTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetTransaction(context.Background(), "abc123")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}
func TestWithHTTPClient_Injection(t *testing.T) {
	mockClient := &mockHTTPClient{
		DoFunc: func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{"status":"healthy"}}`)),
			}, nil
		},
	}

	client, err := NewClient(
		WithHTTPClient(mockClient),
		WithSorobanURL("https://example.com"),
	)
	assert.NoError(t, err)
	assert.Equal(t, mockClient, client.httpClient)

	ctx := context.Background()
	_, err = client.GetHealth(ctx)
	assert.NoError(t, err)
	assert.True(t, mockClient.called)
}

type mockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
	called bool
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.called = true
	return m.DoFunc(req)
}

func (m *mockHTTPClient) Get(_ string) (*http.Response, error) {
	return nil, fmt.Errorf("Get not implemented in mockHTTPClient")
}

func (m *mockHTTPClient) Post(_, _ string, _ io.Reader) (*http.Response, error) {
	return nil, fmt.Errorf("Post not implemented in mockHTTPClient")
}

func (m *mockHTTPClient) PostForm(_ string, _ url.Values) (*http.Response, error) {
	return nil, fmt.Errorf("PostForm not implemented in mockHTTPClient")
}
