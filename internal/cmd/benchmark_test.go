// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"io"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stretchr/testify/mock"
)

// MockHorizonClient implements horizonclient.ClientInterface
type MockHorizonClient struct {
	mock.Mock
}

func (m *MockHorizonClient) TransactionDetail(txHash string) (hProtocol.Transaction, error) {
	args := m.Called(txHash)
	return args.Get(0).(hProtocol.Transaction), args.Error(1)
}

func (m *MockHorizonClient) AccountDetail(request interface{}) (hProtocol.Account, error) { return hProtocol.Account{}, nil }
func (m *MockHorizonClient) AccountData(request interface{}) (hProtocol.AccountData, error) { return hProtocol.AccountData{}, nil }
func (m *MockHorizonClient) Effects(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Assets(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Ledgers(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) LedgerDetail(sequence uint32) (hProtocol.Ledger, error) { return hProtocol.Ledger{}, nil }
func (m *MockHorizonClient) Metrics() (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Offers(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Operations(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) OperationDetail(id string) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) OrderBook(request interface{}) (hProtocol.OrderBookSummary, error) { return hProtocol.OrderBookSummary{}, nil }
func (m *MockHorizonClient) Paths(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Payments(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) TradeAggregations(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Trades(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Transactions(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) SubmitTransactionXDR(transactionXdr string) (hProtocol.TransactionSuccess, error) { return hProtocol.TransactionSuccess{}, nil }
func (m *MockHorizonClient) SubmitTransaction(transactionXdr interface{}) (hProtocol.TransactionSuccess, error) { return hProtocol.TransactionSuccess{}, nil }
func (m *MockHorizonClient) Root() (hProtocol.Root, error) { return hProtocol.Root{}, nil }
func (m *MockHorizonClient) Next(url string) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) Prev(url string) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) HomeDomainForAccount(accountID string) (string, error) { return "", nil }
func (m *MockHorizonClient) LiquidityPools(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) LiquidityPoolDetail(id string) (hProtocol.LiquidityPool, error) { return hProtocol.LiquidityPool{}, nil }
func (m *MockHorizonClient) ClaimableBalances(request interface{}) (interface{}, error) { return nil, nil }
func (m *MockHorizonClient) ClaimableBalanceDetail(id string) (hProtocol.ClaimableBalance, error) { return hProtocol.ClaimableBalance{}, nil }
func (m *MockHorizonClient) StreamTransactions(ctx interface{}, cursor string, handler interface{}) error { return nil }
func (m *MockHorizonClient) StreamLedgers(ctx interface{}, cursor string, handler interface{}) error { return nil }
func (m *MockHorizonClient) StreamOperations(ctx interface{}, cursor string, handler interface{}) error { return nil }
func (m *MockHorizonClient) StreamPayments(ctx interface{}, cursor string, handler interface{}) error { return nil }
func (m *MockHorizonClient) StreamEffects(ctx interface{}, cursor string, handler interface{}) error { return nil }
func (m *MockHorizonClient) StreamOffers(ctx interface{}, cursor string, handler interface{}) error { return nil }

// MockRunner implements simulator.RunnerInterface
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
	args := m.Called(req)
	return args.Get(0).(*simulator.SimulationResponse), args.Error(1)
}

func BenchmarkDebugCommand(b *testing.B) {
	// Setup mocks
	mockHorizon := new(MockHorizonClient)
	mockRunner := new(MockRunner)

	txHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	mockHorizon.On("TransactionDetail", txHash).Return(hProtocol.Transaction{
		EnvelopeXdr:   "AAAA...",
		ResultMetaXdr: "AAAA...",
	}, nil)

	// Inject mock client factory
	clientFactory := func(opts ...rpc.ClientOption) (*rpc.Client, error) {
		client, _ := rpc.NewClient(opts...)
		client.Horizon = mockHorizon
		return client, nil
	}

	// Create command with options
	cobraCmd := NewDebugCommand(mockRunner, WithClientFactory(clientFactory))
	cobraCmd.SetArgs([]string{txHash})
	cobraCmd.SetOut(io.Discard)
	cobraCmd.SetErr(io.Discard)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cobraCmd.Execute()
	}
}
