// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ipc

import (
	"testing"

	"github.com/dotandev/hintents/internal/errors"
)

func TestToErstErrorMemoryLimitByCode(t *testing.T) {
	e := (&Error{
		Code:    "ERR_MEMORY_LIMIT_EXCEEDED",
		Message: "ERR_MEMORY_LIMIT_EXCEEDED: consumed 2048 bytes, limit 1024 bytes",
	}).ToErstError()

	if e.Code != errors.CodeSimMemoryLimitExceeded {
		t.Fatalf("expected %s, got %s", errors.CodeSimMemoryLimitExceeded, e.Code)
	}
}

func TestToErstErrorMemoryLimitByMessage(t *testing.T) {
	e := (&Error{
		Message: "memory limit exceeded while simulating contract",
	}).ToErstError()

	if e.Code != errors.CodeSimMemoryLimitExceeded {
		t.Fatalf("expected %s, got %s", errors.CodeSimMemoryLimitExceeded, e.Code)
	}
}
