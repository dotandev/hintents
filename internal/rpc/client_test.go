// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	effects "github.com/stellar/go/protocols/horizon/effects"
	operations "github.com/stellar/go/protocols/horizon/operations"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
)

type mockHorizonClient struct {
	TransactionDetailFunc func(hash string) (hProtocol.Transaction, error)
	LedgerDetailFunc      func(sequence uint32) (hProtocol.Ledger, error)
}

func (m *mockHorizonClient) TransactionDetail(hash string) (hProtocol.Transaction, error) {
	return m.TransactionDetailFunc(hash)
}
func (m *mockHorizonClient) AccountData(request horizonclient.AccountRequest) (hProtocol.AccountData, error) {
	return hProtocol.AccountData{}, nil
}
func (m *mockHorizonClient) AccountDetail(request horizonclient.AccountRequest) (hProtocol.Account, error) {
	return hProtocol.Account{}, nil
}
func (m *mockHorizonClient) Accounts(request horizonclient.AccountsRequest) (hProtocol.AccountsPage, error) {
	return hProtocol.AccountsPage{}, nil
}
func (m *mockHorizonClient) Effects(request horizonclient.EffectRequest) (effects.EffectsPage, error) {
	return effects.EffectsPage{}, nil
}
func (m *mockHorizonClient) Assets(request horizonclient.AssetRequest) (hProtocol.AssetsPage, error) {
	return hProtocol.AssetsPage{}, nil
}
func (m *mockHorizonClient) Ledgers(request horizonclient.LedgerRequest) (hProtocol.LedgersPage, error) {
	return hProtocol.LedgersPage{}, nil
}
func (m *mockHorizonClient) LedgerDetail(sequence uint32) (hProtocol.Ledger, error) {
	if m.LedgerDetailFunc != nil {
		return m.LedgerDetailFunc(sequence)
	}
	return hProtocol.Ledger{}, nil
}
func (m *mockHorizonClient) FeeStats() (hProtocol.FeeStats, error) { return hProtocol.FeeStats{}, nil }
func (m *mockHorizonClient) Offers(request horizonclient.OfferRequest) (hProtocol.OffersPage, error) {
	return hProtocol.OffersPage{}, nil
}
func (m *mockHorizonClient) OfferDetails(offerID string) (hProtocol.Offer, error) {
	return hProtocol.Offer{}, nil
}
func (m *mockHorizonClient) Operations(request horizonclient.OperationRequest) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) OperationDetail(id string) (operations.Operation, error) {
	var op operations.Operation
	return op, nil
}
func (m *mockHorizonClient) StreamPayments(ctx context.Context, request horizonclient.OperationRequest, handler horizonclient.OperationHandler) error {
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
func (m *mockHorizonClient) Transactions(request horizonclient.TransactionRequest) (hProtocol.TransactionsPage, error) {
	return hProtocol.TransactionsPage{}, nil
}
func (m *mockHorizonClient) OrderBook(request horizonclient.OrderBookRequest) (hProtocol.OrderBookSummary, error) {
	return hProtocol.OrderBookSummary{}, nil
}
func (m *mockHorizonClient) Paths(request horizonclient.PathsRequest) (hProtocol.PathsPage, error) {
	return hProtocol.PathsPage{}, nil
}
func (m *mockHorizonClient) Payments(request horizonclient.OperationRequest) (operations.OperationsPage, error) {
	return operations.OperationsPage{}, nil
}
func (m *mockHorizonClient) TradeAggregations(request horizonclient.TradeAggregationRequest) (hProtocol.TradeAggregationsPage, error) {
	return hProtocol.TradeAggregationsPage{}, nil
}
func (m *mockHorizonClient) Trades(request horizonclient.TradeRequest) (hProtocol.TradesPage, error) {
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
		&Client{Horizon: mock.(*mockHorizonClient)},
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
			name: "error",
			hash: "fail",
			mockFunc: func(hash string) (hProtocol.Transaction, error) {
				return hProtocol.Transaction{}, errors.New("not found")
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
