// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package shutdown

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type HookFunc func(context.Context) error

type hook struct {
	name string
	fn   HookFunc
}

// Coordinator runs registered shutdown hooks exactly once in LIFO order.
type Coordinator struct {
	mu   sync.Mutex
	hook []hook
	ran  bool
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		hook: make([]hook, 0),
	}
}

func (c *Coordinator) Register(name string, fn HookFunc) {
	if fn == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ran {
		return
	}

	c.hook = append(c.hook, hook{name: name, fn: fn})
}

func (c *Coordinator) Run(ctx context.Context) error {
	c.mu.Lock()
	if c.ran {
		c.mu.Unlock()
		return nil
	}
	c.ran = true
	hooks := make([]hook, len(c.hook))
	copy(hooks, c.hook)
	c.mu.Unlock()

	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]

		hookCtx, cancel := perHookContext(ctx, i+1)
		err := h.fn(hookCtx)
		cancel()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", h.name, err))
		}
	}

	return errors.Join(errs...)
}

func perHookContext(ctx context.Context, hooksRemaining int) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok || hooksRemaining <= 0 {
		return ctx, func() {}
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.WithTimeout(ctx, 1*time.Millisecond)
	}

	perHook := remaining / time.Duration(hooksRemaining)
	if perHook <= 0 {
		perHook = remaining
	}
	return context.WithTimeout(ctx, perHook)
}
