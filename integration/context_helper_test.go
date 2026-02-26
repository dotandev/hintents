package integration

import (
	"context"
	"testing"
	"time"
)

func buildTestContext(t *testing.T, d time.Duration) (interface{ Done() <-chan struct{} }, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	return ctx, cancel
}