// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/render/problem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultLedgerRetryConfig verifies the default backoff settings for ledger header fetches.
func TestDefaultLedgerRetryConfig(t *testing.T) {
	cfg := DefaultLedgerRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries, "default max retries should be 3")
	assert.Equal(t, 500*time.Millisecond, cfg.InitialBackoff, "default initial backoff should be 500ms")
	assert.Equal(t, 2*time.Second, cfg.MaxBackoff, "default max backoff should be 2s")
	assert.Equal(t, float64(0), cfg.JitterFraction, "default jitter should be 0 for deterministic backoff")
	assert.Contains(t, cfg.StatusCodesToRetry, 429)
	assert.Contains(t, cfg.StatusCodesToRetry, 500)
	assert.Contains(t, cfg.StatusCodesToRetry, 503)
}

// TestGetLedgerHeader_ExponentialBackoffSequence verifies the retry backoff progression
// (e.g., doubling: initial -> initial*2 -> initial*4 -> capped at max).
func TestGetLedgerHeader_ExponentialBackoffSequence(t *testing.T) {
	var callTimes []time.Time
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}
	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		callTimes = append(callTimes, time.Now())
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 429,
				Detail: "rate limit exceeded",
			},
		}
	}

	initialBackoff := 20 * time.Millisecond
	maxBackoff := 100 * time.Millisecond
	client, err := NewClient(
		WithNetwork(Testnet),
		WithLedgerBackoff(initialBackoff, maxBackoff, 3),
	)
	require.NoError(t, err)
	client.Horizon = mock

	ctx := context.Background()
	startTime := time.Now()
	_, err = client.GetLedgerHeader(ctx, 100)
	totalDuration := time.Since(startTime)

	require.Error(t, err)
	assert.True(t, IsRateLimitError(err))
	// 1 initial attempt + 3 retries = 4 total calls
	require.Len(t, callTimes, 4, "expected 4 calls: 1 initial attempt + 3 retries")

	// Backoff durations between calls should roughly be:
	// Gap 0->1: ~20ms
	// Gap 1->2: ~40ms
	// Gap 2->3: ~80ms
	// Total minimum duration: 20 + 40 + 80 = 140ms
	expectedMinDuration := 20*time.Millisecond + 40*time.Millisecond + 80*time.Millisecond
	assert.GreaterOrEqual(t, totalDuration, expectedMinDuration-10*time.Millisecond,
		"total duration should reflect exponential backoff delays")

	gap1 := callTimes[1].Sub(callTimes[0])
	gap2 := callTimes[2].Sub(callTimes[1])
	gap3 := callTimes[3].Sub(callTimes[2])

	assert.GreaterOrEqual(t, gap1, 15*time.Millisecond, "gap1 should be >= ~20ms")
	assert.GreaterOrEqual(t, gap2, 30*time.Millisecond, "gap2 should be >= ~40ms")
	assert.GreaterOrEqual(t, gap3, 60*time.Millisecond, "gap3 should be >= ~80ms")
}

// TestGetLedgerHeader_SucceedsOnRetry verifies that when a retryable error occurs
// on the first attempt, the client backs off and succeeds on the subsequent attempt.
func TestGetLedgerHeader_SucceedsOnRetry(t *testing.T) {
	var attempts int32
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}
	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		att := atomic.AddInt32(&attempts, 1)
		if att == 1 {
			return hProtocol.Ledger{}, &horizonclient.Error{
				Problem: problem.P{
					Status: http.StatusTooManyRequests,
					Detail: "Rate limit exceeded",
				},
			}
		}
		return hProtocol.Ledger{
			Sequence: int32(sequence),
			Hash:     "successful_hash",
		}, nil
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithLedgerBackoff(10*time.Millisecond, 50*time.Millisecond, 2),
	)
	require.NoError(t, err)
	client.Horizon = mock

	header, err := client.GetLedgerHeader(context.Background(), 12345)
	require.NoError(t, err)
	require.NotNil(t, header)
	assert.Equal(t, uint32(12345), header.Sequence)
	assert.Equal(t, "successful_hash", header.Hash)
	assert.Equal(t, int32(2), atomic.LoadInt32(&attempts), "should have succeeded on attempt 2")
}

// TestGetLedgerHeader_NonRetryableErrors ensures that 404 Not Found, 410 Archived,
// and 413 Response Too Large do not trigger retries or backoff delays.
func TestGetLedgerHeader_NonRetryableErrors(t *testing.T) {
	testCases := []struct {
		name          string
		status        int
		detail        string
		checkErr      func(error) bool
		expectedError string
	}{
		{
			name:     "NotFound_404",
			status:   404,
			detail:   "Ledger not found",
			checkErr: IsLedgerNotFound,
		},
		{
			name:     "Archived_410",
			status:   410,
			detail:   "Ledger archived",
			checkErr: IsLedgerArchived,
		},
		{
			name:     "TooLarge_413",
			status:   413,
			detail:   "Response too large",
			checkErr: IsResponseTooLarge,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			mock := &mockHorizonClient{
				TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
					return hProtocol.Transaction{}, nil
				},
			}
			mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
				atomic.AddInt32(&attempts, 1)
				return hProtocol.Ledger{}, &horizonclient.Error{
					Problem: problem.P{
						Status: tc.status,
						Detail: tc.detail,
					},
				}
			}

			client, err := NewClient(
				WithNetwork(Testnet),
				WithLedgerBackoff(100*time.Millisecond, 500*time.Millisecond, 3),
			)
			require.NoError(t, err)
			client.Horizon = mock

			start := time.Now()
			_, err = client.GetLedgerHeader(context.Background(), 999999)
			duration := time.Since(start)

			require.Error(t, err)
			assert.True(t, tc.checkErr(err), "error check failed for %s", tc.name)
			assert.Equal(t, int32(1), atomic.LoadInt32(&attempts), "should make only 1 attempt for non-retryable error")
			assert.Less(t, duration, 50*time.Millisecond, "non-retryable error must fail fast without backoff sleep")
		})
	}
}

// TestGetLedgerHeader_ContextCancellation verifies that canceling the context
// during a backoff sleep causes immediate termination without hanging.
func TestGetLedgerHeader_ContextCancellation(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}
	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, errors.New("network failure")
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithLedgerBackoff(500*time.Millisecond, 2*time.Second, 3),
	)
	require.NoError(t, err)
	client.Horizon = mock

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context shortly after the first attempt fails and backoff sleep begins
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = client.GetLedgerHeader(ctx, 12345)
	duration := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "should return context.Canceled")
	assert.Less(t, duration, 200*time.Millisecond, "context cancellation should interrupt backoff sleep immediately")
}

// TestGetLedgerHeader_FailoverWithBackoff verifies that with multiple URLs,
// the client waits exponential backoff before rotating to the next URL.
func TestGetLedgerHeader_FailoverWithBackoff(t *testing.T) {
	var attempts int32
	var urlsTried []string

	mock1 := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}
	mock1.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		atomic.AddInt32(&attempts, 1)
		urlsTried = append(urlsTried, "http://node1.example.com")
		return hProtocol.Ledger{}, errors.New("node1 down")
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithAltURLs([]string{"http://node1.example.com", "http://node2.example.com"}),
		WithLedgerBackoff(10*time.Millisecond, 50*time.Millisecond, 1),
	)
	require.NoError(t, err)
	client.Horizon = mock1

	_, err = client.GetLedgerHeader(context.Background(), 12345)
	require.Error(t, err)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(1))
}

// TestClient_CalculateNextLedgerBackoff tests the exponential calculation and capping.
func TestClient_CalculateNextLedgerBackoff(t *testing.T) {
	client := &Client{
		ledgerRetryConfig: RetryConfig{
			InitialBackoff: 500 * time.Millisecond,
			MaxBackoff:     2 * time.Second,
			JitterFraction: 0,
		},
	}

	b1 := client.calculateNextLedgerBackoff(500 * time.Millisecond)
	assert.Equal(t, 1*time.Second, b1, "500ms doubled should be 1s")

	b2 := client.calculateNextLedgerBackoff(b1)
	assert.Equal(t, 2*time.Second, b2, "1s doubled should be 2s")

	b3 := client.calculateNextLedgerBackoff(b2)
	assert.Equal(t, 2*time.Second, b3, "2s doubled capped at MaxBackoff should be 2s")
}

// TestClient_WithLedgerRetryConfig_Option verifies the functional option sets the config.
func TestClient_WithLedgerRetryConfig_Option(t *testing.T) {
	customCfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 250 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		JitterFraction: 0.2,
	}

	client, err := NewClient(
		WithLedgerRetryConfig(customCfg),
	)
	require.NoError(t, err)

	got := client.GetLedgerRetryConfig()
	assert.Equal(t, 5, got.MaxRetries)
	assert.Equal(t, 250*time.Millisecond, got.InitialBackoff)
	assert.Equal(t, 5*time.Second, got.MaxBackoff)
	assert.Equal(t, 0.2, got.JitterFraction)

	// Test SetLedgerRetryConfig
	newCfg := DefaultLedgerRetryConfig()
	client.SetLedgerRetryConfig(newCfg)
	assert.Equal(t, 3, client.GetLedgerRetryConfig().MaxRetries)
}

// TestClient_IsRetryableLedgerError verifies error categorization.
func TestClient_IsRetryableLedgerError(t *testing.T) {
	ctx := context.Background()

	// Nil error
	assert.False(t, isRetryableLedgerError(ctx, nil))

	// Canceled context
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	assert.False(t, isRetryableLedgerError(canceledCtx, errors.New("any error")))

	// Sentinel non-retryable errors
	assert.False(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 404}}))
	assert.False(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 410}}))
	assert.False(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 413}}))
	assert.False(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 400}}))
	assert.False(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 401}}))

	// Retryable errors
	assert.True(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 429}}))
	assert.True(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 500}}))
	assert.True(t, isRetryableLedgerError(ctx, &horizonclient.Error{Problem: problem.P{Status: 503}}))
	assert.True(t, isRetryableLedgerError(ctx, errors.New("connection reset by peer")))
}
