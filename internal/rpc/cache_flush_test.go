// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"testing"
)

func TestFlush_NoOp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Flush(ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
