// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	stderrors "errors"
)

const InterruptExitCode = 130

var ErrInterrupted = stderrors.New("interrupt received")

func IsInterrupted(err error) bool {
	return stderrors.Is(err, ErrInterrupted)
}

func IsCancellation(err error) bool {
	return stderrors.Is(err, context.Canceled)
}
