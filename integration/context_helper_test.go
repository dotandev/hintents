// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"testing"
	"time"
)

func buildTestContext(t *testing.T, d time.Duration) (context.Context, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	return ctx, cancel
}
