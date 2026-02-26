// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package shutdown

import (
	"context"
	"testing"
)

func TestCoordinatorRun_LIFOAndOnce(t *testing.T) {
	c := NewCoordinator()
	order := make([]string, 0, 3)

	c.Register("first", func(ctx context.Context) error {
		_ = ctx
		order = append(order, "first")
		return nil
	})
	c.Register("second", func(ctx context.Context) error {
		_ = ctx
		order = append(order, "second")
		return nil
	})
	c.Register("third", func(ctx context.Context) error {
		_ = ctx
		order = append(order, "third")
		return nil
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	want := []string{"third", "second", "first"}
	if len(order) != len(want) {
		t.Fatalf("unexpected hook count: got %d want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("unexpected order at %d: got %s want %s", i, order[i], want[i])
		}
	}

	// Second run should be a no-op.
	order = order[:0]
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("unexpected second run error: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("expected no hooks on second run, got %d", len(order))
	}
}
