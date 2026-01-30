// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetLedgerHeader_Integration_Testnet tests fetching a real ledger from Stellar testnet
// This test requires network access and is skipped in short mode
func TestGetLedgerHeader_Integration_Testnet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(Testnet, "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch a recent ledger (testnet ledgers are typically in the millions)
	// We'll try to fetch a ledger that's likely to exist
	// Note: This sequence number should be updated periodically to a recent ledger
	sequence := uint32(3000000) // Update this to a recent testnet ledger sequence

	header, err := client.GetLedgerHeader(ctx, sequence)

	// The ledger might be archived or not exist, which is acceptable for this test
	if err != nil {
		if IsLedgerNotFound(err) || IsLedgerArchived(err) {
			t.Logf("Ledger %d not available (archived or not found): %v", sequence, err)
			t.Skip("Ledger not available for testing")
		}
		require.NoError(t, err, "unexpected error fetching ledger")
	}

	// If we got a header, verify its structure
	require.NotNil(t, header)
	assert.Equal(t, sequence, header.Sequence)
	assert.NotEmpty(t, header.Hash, "hash should not be empty")
	assert.NotEmpty(t, header.PrevHash, "prev hash should not be empty")
	assert.NotZero(t, header.CloseTime, "close time should not be zero")
	assert.Greater(t, header.ProtocolVersion, uint32(0), "protocol version should be positive")
	assert.NotEmpty(t, header.HeaderXDR, "header XDR should not be empty")
	assert.Greater(t, header.BaseFee, int32(0), "base fee should be positive")
	assert.Greater(t, header.BaseReserve, int32(0), "base reserve should be positive")

	t.Logf("Successfully fetched ledger %d:", sequence)
	t.Logf("  Hash: %s", header.Hash)
	t.Logf("  Protocol Version: %d", header.ProtocolVersion)
	t.Logf("  Close Time: %s", header.CloseTime)
	t.Logf("  Base Fee: %d stroops", header.BaseFee)
	t.Logf("  Successful Transactions: %d", header.SuccessfulTxCount)
	t.Logf("  Failed Transactions: %d", header.FailedTxCount)
	t.Logf("  Operations: %d", header.OperationCount)
}

// TestGetLedgerHeader_Integration_FutureLedger tests handling of future ledgers
func TestGetLedgerHeader_Integration_FutureLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(Testnet, "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to fetch a ledger far in the future
	farFutureSequence := uint32(999999999)

	_, err := client.GetLedgerHeader(ctx, farFutureSequence)
	require.Error(t, err)
	assert.True(t, IsLedgerNotFound(err), "should be ledger not found error")

	t.Logf("Correctly handled future ledger request: %v", err)
}

// TestGetLedgerHeader_Integration_MultipleNetworks tests that different networks work
func TestGetLedgerHeader_Integration_MultipleNetworks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		network  Network
		sequence uint32
	}{
		{Testnet, 3000000},
		// Note: Mainnet and Futurenet tests are commented out to avoid hitting rate limits
		// Uncomment if you want to test against those networks
		// {Mainnet, 50000000},
		// {Futurenet, 1000000},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			client := NewClient(tt.network, "")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			header, err := client.GetLedgerHeader(ctx, tt.sequence)

			// Accept archived/not found as valid responses
			if err != nil {
				if IsLedgerNotFound(err) || IsLedgerArchived(err) {
					t.Logf("Ledger not available on %s: %v", tt.network, err)
					return
				}
				require.NoError(t, err)
			}

			require.NotNil(t, header)
			assert.Equal(t, tt.sequence, header.Sequence)
			t.Logf("%s ledger %d fetched successfully", tt.network, tt.sequence)
		})
	}
}

// TestGetLedgerHeader_Integration_RecentLedger attempts to fetch a very recent ledger
func TestGetLedgerHeader_Integration_RecentLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(Testnet, "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, get the root endpoint to find the latest ledger
	// For this test, we'll just try a high sequence number
	// In production, you'd want to query the latest ledger first
	recentSequence := uint32(10000000) // Adjust based on current testnet state

	header, err := client.GetLedgerHeader(ctx, recentSequence)

	if err != nil {
		if IsLedgerNotFound(err) || IsLedgerArchived(err) {
			t.Logf("Recent ledger %d not available: %v", recentSequence, err)
			t.Skip("Recent ledger not available")
		}
		require.NoError(t, err)
	}

	require.NotNil(t, header)
	assert.Equal(t, recentSequence, header.Sequence)

	// Verify the ledger is relatively recent (within last year)
	oneYearAgo := time.Now().AddDate(-1, 0, 0)
	assert.True(t, header.CloseTime.After(oneYearAgo),
		"ledger should be from within the last year, got: %s", header.CloseTime)

	t.Logf("Recent ledger fetched: %d (closed at %s)", header.Sequence, header.CloseTime)
}
