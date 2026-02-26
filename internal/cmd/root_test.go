// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/shutdown"
)

func TestExecuteWithSignals_InterruptReturnsSentinelAndRunsShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	coordinator := shutdown.NewCoordinator()
	ranShutdownHook := make(chan struct{}, 1)
	coordinator.Register("test-hook", func(ctx context.Context) error {
		_ = ctx
		ranShutdownHook <- struct{}{}
		return nil
	})

	done := make(chan error, 1)
	go func() {
		done <- executeWithSignals(ctx, cancel, sigCh, coordinator, func(execCtx context.Context) error {
			<-execCtx.Done()
			return execCtx.Err()
		})
	}()

	time.Sleep(30 * time.Millisecond)
	sigCh <- os.Interrupt

	select {
	case err := <-done:
		if !IsInterrupted(err) {
			t.Fatalf("expected interrupt error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for executeWithSignals to return")
	}

	select {
	case <-ranShutdownHook:
	case <-time.After(1 * time.Second):
		t.Fatal("expected shutdown hook to run")
	}
}

func TestExecuteWithSignals_NoInterruptReturnsExecError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	coordinator := shutdown.NewCoordinator()

	expectedErr := context.DeadlineExceeded
	err := executeWithSignals(ctx, cancel, sigCh, coordinator, func(execCtx context.Context) error {
		_ = execCtx
		return expectedErr
	})
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
