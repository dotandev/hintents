// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	hProtocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/render/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetLedgerHeader_Success tests successful ledger header retrieval
func TestGetLedgerHeader_Success(t *testing.T) {
	closeTime := time.Now().UTC()
	expectedSequence := uint32(12345678)
	failedTxCount := int32(5)

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	// Override LedgerDetail to return test data
	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence:                   int32(expectedSequence),
			Hash:                       "abc123hash",
			PrevHash:                   "prev456hash",
			ClosedAt:                   closeTime,
			ProtocolVersion:            20,
			BaseFee:                    100,
			BaseReserve:                5000000,
			MaxTxSetSize:               1000,
			TotalCoins:                 "1000000000000",
			FeePool:                    "1000000",
			HeaderXDR:                  "AAAA...",
			SuccessfulTransactionCount: 50,
			FailedTransactionCount:     &failedTxCount,
			OperationCount:             200,
		}, nil
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	header, err := client.GetLedgerHeader(ctx, expectedSequence)
	require.NoError(t, err)
	require.NotNil(t, header)

	// Verify all fields
	assert.Equal(t, expectedSequence, header.Sequence)
	assert.Equal(t, "abc123hash", header.Hash)
	assert.Equal(t, "prev456hash", header.PrevHash)
	assert.Equal(t, closeTime, header.CloseTime)
	assert.Equal(t, uint32(20), header.ProtocolVersion)
	assert.Equal(t, int32(100), header.BaseFee)
	assert.Equal(t, int32(5000000), header.BaseReserve)
	assert.Equal(t, int32(1000), header.MaxTxSetSize)
	assert.Equal(t, "1000000000000", header.TotalCoins)
	assert.Equal(t, "1000000", header.FeePool)
	assert.Equal(t, "AAAA...", header.HeaderXDR)
	assert.Equal(t, int32(50), header.SuccessfulTxCount)
	assert.Equal(t, int32(5), header.FailedTxCount)
	assert.Equal(t, int32(200), header.OperationCount)
}

// TestGetLedgerHeader_NotFound tests handling of non-existent ledgers
func TestGetLedgerHeader_NotFound(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 404,
				Detail: "Ledger not found",
			},
		}
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 999999999)
	require.Error(t, err)
	assert.True(t, IsLedgerNotFound(err), "should be ledger not found error")

	notFoundErr, ok := err.(*LedgerNotFoundError)
	require.True(t, ok)
	assert.Equal(t, uint32(999999999), notFoundErr.Sequence)
	assert.Contains(t, notFoundErr.Message, "not found")
}

// TestGetLedgerHeader_Archived tests handling of archived ledgers
func TestGetLedgerHeader_Archived(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 410,
				Detail: "Ledger has been archived",
			},
		}
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 1)
	require.Error(t, err)
	assert.True(t, IsLedgerArchived(err), "should be ledger archived error")

	archivedErr, ok := err.(*LedgerArchivedError)
	require.True(t, ok)
	assert.Equal(t, uint32(1), archivedErr.Sequence)
	assert.Contains(t, archivedErr.Message, "archived")
}

// TestGetLedgerHeader_RateLimit tests handling of rate limit errors
func TestGetLedgerHeader_RateLimit(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 429,
				Detail: "Rate limit exceeded",
			},
		}
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.True(t, IsRateLimitError(err), "should be rate limit error")

	rateLimitErr, ok := err.(*RateLimitError)
	require.True(t, ok)
	assert.Contains(t, rateLimitErr.Message, "rate limit")
}

// TestGetLedgerHeader_Timeout tests context timeout handling
func TestGetLedgerHeader_Timeout(t *testing.T) {
	var testCtx context.Context
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		select {
		case <-time.After(2 * time.Second):
			return hProtocol.Ledger{}, nil
		case <-testCtx.Done():
			return hProtocol.Ledger{}, testCtx.Err()
		}
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	testCtx = ctx

	_, err := client.GetLedgerHeader(ctx, 12345)
	assert.Error(t, err)
}

// TestGetLedgerHeader_GenericError tests handling of generic errors
func TestGetLedgerHeader_GenericError(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, errors.New("network error")
	}

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.False(t, IsLedgerNotFound(err))
	assert.False(t, IsLedgerArchived(err))
	assert.False(t, IsRateLimitError(err))
	assert.Contains(t, err.Error(), "failed to fetch ledger")
}

// TestFromHorizonLedger tests the conversion from Horizon ledger to our structure
func TestFromHorizonLedger(t *testing.T) {
	closeTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	failedTxCount := int32(5)

	horizonLedger := hProtocol.Ledger{
		Sequence:                   12345,
		Hash:                       "abcd1234",
		PrevHash:                   "prev5678",
		ClosedAt:                   closeTime,
		ProtocolVersion:            20,
		BaseFee:                    100,
		BaseReserve:                5000000,
		MaxTxSetSize:               1000,
		TotalCoins:                 "1000000000000",
		FeePool:                    "1000000",
		HeaderXDR:                  "AAAA...",
		SuccessfulTransactionCount: 50,
		FailedTransactionCount:     &failedTxCount,
		OperationCount:             200,
	}

	result := FromHorizonLedger(horizonLedger)

	assert.Equal(t, uint32(12345), result.Sequence)
	assert.Equal(t, "abcd1234", result.Hash)
	assert.Equal(t, "prev5678", result.PrevHash)
	assert.Equal(t, closeTime, result.CloseTime)
	assert.Equal(t, uint32(20), result.ProtocolVersion)
	assert.Equal(t, int32(100), result.BaseFee)
	assert.Equal(t, int32(5000000), result.BaseReserve)
	assert.Equal(t, int32(1000), result.MaxTxSetSize)
	assert.Equal(t, "1000000000000", result.TotalCoins)
	assert.Equal(t, "1000000", result.FeePool)
	assert.Equal(t, "AAAA...", result.HeaderXDR)
	assert.Equal(t, int32(50), result.SuccessfulTxCount)
	assert.Equal(t, int32(5), result.FailedTxCount)
	assert.Equal(t, int32(200), result.OperationCount)
}

// TestErrorTypes tests the error type checking functions
func TestErrorTypes(t *testing.T) {
	notFoundErr := &LedgerNotFoundError{Sequence: 123, Message: "not found"}
	archivedErr := &LedgerArchivedError{Sequence: 456, Message: "archived"}
	rateLimitErr := &RateLimitError{Message: "rate limited"}
	genericErr := errors.New("generic error")

	// Test IsLedgerNotFound
	assert.True(t, IsLedgerNotFound(notFoundErr))
	assert.False(t, IsLedgerNotFound(archivedErr))
	assert.False(t, IsLedgerNotFound(rateLimitErr))
	assert.False(t, IsLedgerNotFound(genericErr))

	// Test IsLedgerArchived
	assert.True(t, IsLedgerArchived(archivedErr))
	assert.False(t, IsLedgerArchived(notFoundErr))
	assert.False(t, IsLedgerArchived(rateLimitErr))
	assert.False(t, IsLedgerArchived(genericErr))

	// Test IsRateLimitError
	assert.True(t, IsRateLimitError(rateLimitErr))
	assert.False(t, IsRateLimitError(notFoundErr))
	assert.False(t, IsRateLimitError(archivedErr))
	assert.False(t, IsRateLimitError(genericErr))
}

// TestErrorMessages tests that error messages are descriptive
func TestErrorMessages(t *testing.T) {
	notFoundErr := &LedgerNotFoundError{
		Sequence: 123,
		Message:  "ledger 123 not found (may be archived or not yet created)",
	}
	assert.Contains(t, notFoundErr.Error(), "123")
	assert.Contains(t, notFoundErr.Error(), "not found")

	archivedErr := &LedgerArchivedError{
		Sequence: 456,
		Message:  "ledger 456 has been archived and is no longer available",
	}
	assert.Contains(t, archivedErr.Error(), "456")
	assert.Contains(t, archivedErr.Error(), "archived")

	rateLimitErr := &RateLimitError{
		Message: "rate limit exceeded, please try again later",
	}
	assert.Contains(t, rateLimitErr.Error(), "rate limit")
}

// TestGetLedgerHeader_DifferentNetworks tests that the client works with different networks
func TestGetLedgerHeader_DifferentNetworks(t *testing.T) {
	networks := []Network{Testnet, Mainnet, Futurenet}

	for _, network := range networks {
		t.Run(string(network), func(t *testing.T) {
			mock := &mockHorizonClient{
				TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
					return hProtocol.Transaction{}, nil
				},
			}

			mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
				return hProtocol.Ledger{
					Sequence:        int32(sequence),
					Hash:            "test_hash",
					ProtocolVersion: 20,
				}, nil
			}

			client := &Client{Horizon: mock, Network: network}
			ctx := context.Background()

			header, err := client.GetLedgerHeader(ctx, 12345)
			require.NoError(t, err)
			assert.NotNil(t, header)
			assert.Equal(t, uint32(12345), header.Sequence)
		})
	}
}

// TestGetLedgerHeader_ContextWithDeadline tests that existing context deadlines are respected
func TestGetLedgerHeader_ContextWithDeadline(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence: int32(sequence),
			Hash:     "test_hash",
		}, nil
	}

	client := &Client{Horizon: mock, Network: Testnet}

	// Create context with deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	header, err := client.GetLedgerHeader(ctx, 12345)
	require.NoError(t, err)
	assert.NotNil(t, header)
}

// TestGetLedgerHeader_ContextWithoutDeadline tests that a default timeout is added
func TestGetLedgerHeader_ContextWithoutDeadline(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence: int32(sequence),
			Hash:     "test_hash",
		}, nil
	}

	client := &Client{Horizon: mock, Network: Testnet}

	// Create context without deadline
	ctx := context.Background()

	header, err := client.GetLedgerHeader(ctx, 12345)
	require.NoError(t, err)
	assert.NotNil(t, header)
}
