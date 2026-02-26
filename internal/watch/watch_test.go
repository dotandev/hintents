// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewPollerDefaults(t *testing.T) {
	poller := NewPoller(PollerConfig{})
	if poller == nil {
		t.Fatal("expected non-nil poller")
		return
	}

	if poller.config.MaxAttempts != 60 {
		t.Errorf("expected MaxAttempts 60, got %d", poller.config.MaxAttempts)
	}

	if poller.config.InitialInterval != 1*time.Second {
		t.Errorf("expected InitialInterval 1s, got %v", poller.config.InitialInterval)
	}

	if poller.config.TimeoutDuration != 30*time.Second {
		t.Errorf("expected TimeoutDuration 30s, got %v", poller.config.TimeoutDuration)
	}
}

func TestPollSuccess(t *testing.T) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		TimeoutDuration: 5 * time.Second,
	})

	attempt := 0
	checkFunc := func(ctx context.Context) (interface{}, error) {
		attempt++
		if attempt >= 3 {
			return "found", nil
		}
		return nil, fmt.Errorf("not found yet")
	}

	result, err := poller.Poll(context.Background(), checkFunc, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Found {
		t.Error("expected to find data")
	}

	if result.Data != "found" {
		t.Errorf("expected 'found', got %v", result.Data)
	}

	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestPollTimeout(t *testing.T) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     100,
		InitialInterval: 100 * time.Millisecond,
		TimeoutDuration: 200 * time.Millisecond,
	})

	checkFunc := func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Errorf("not found")
	}

	result, err := poller.Poll(context.Background(), checkFunc, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Found {
		t.Error("expected timeout, but found data")
	}

	if result.Error == nil {
		t.Error("expected timeout error")
	}
}

func TestPollMaxAttempts(t *testing.T) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		TimeoutDuration: 30 * time.Second,
	})

	attempt := 0
	checkFunc := func(ctx context.Context) (interface{}, error) {
		attempt++
		return nil, fmt.Errorf("not found")
	}

	result, err := poller.Poll(context.Background(), checkFunc, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Found {
		t.Error("expected max attempts exceeded")
	}

	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestExponentialBackoff(t *testing.T) {
	poller := NewPoller(PollerConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
	})

	tests := []struct {
		current  time.Duration
		expected time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{4 * time.Second, 8 * time.Second},
		{8 * time.Second, 10 * time.Second},
		{10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		result := poller.exponentialBackoff(tt.current)
		if result != tt.expected {
			t.Errorf("backoff(%v) = %v, expected %v", tt.current, result, tt.expected)
		}
	}
}

func TestPollWithAttemptCallback(t *testing.T) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     5,
		InitialInterval: 10 * time.Millisecond,
		TimeoutDuration: 5 * time.Second,
	})

	attempts := []int{}
	checkFunc := func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Errorf("not found")
	}

	onAttempt := func(attempt int) {
		attempts = append(attempts, attempt)
	}

	poller.Poll(context.Background(), checkFunc, onAttempt)

	if len(attempts) != 5 {
		t.Errorf("expected 5 attempts, got %d", len(attempts))
	}

	for i := 0; i < len(attempts); i++ {
		if attempts[i] != i+1 {
			t.Errorf("attempt %d: expected %d, got %d", i, i+1, attempts[i])
		}
	}
}

func TestPollContextCancellation(t *testing.T) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     100,
		InitialInterval: 100 * time.Millisecond,
		TimeoutDuration: 10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())

	checkFunc := func(ctx context.Context) (interface{}, error) {
		cancel()
		return nil, fmt.Errorf("not found")
	}

	result, err := poller.Poll(ctx, checkFunc, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Found {
		t.Error("expected cancellation")
	}
}

func TestNewSpinner(t *testing.T) {
	spinner := NewSpinner()

	if spinner == nil {
		t.Fatal("expected non-nil spinner")
		return
	}

	if len(spinner.frames) == 0 {
		t.Error("expected spinner frames")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	spinner := NewSpinner()
	spinner.Start("Testing...")
	time.Sleep(50 * time.Millisecond)
	spinner.Stop()
}

func TestSpinnerMessages(t *testing.T) {
	spinner := NewSpinner()
	spinner.Start("Testing...")
	time.Sleep(50 * time.Millisecond)
	spinner.StopWithMessage("Test completed")

	spinner2 := NewSpinner()
	spinner2.Start("Testing error...")
	time.Sleep(50 * time.Millisecond)
	spinner2.StopWithError("Test failed")
}

func TestSpinnerDoubleStart(t *testing.T) {
	spinner := NewSpinner()
	spinner.Start("Testing...")
	spinner.Start("Testing again...")
	time.Sleep(50 * time.Millisecond)
	spinner.Stop()
}

func BenchmarkPoller(b *testing.B) {
	poller := NewPoller(PollerConfig{
		MaxAttempts:     5,
		InitialInterval: 1 * time.Millisecond,
		TimeoutDuration: 5 * time.Second,
	})

	checkFunc := func(ctx context.Context) (interface{}, error) {
		return "found", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		poller.Poll(context.Background(), checkFunc, nil)
	}
}
