// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

// MinRecommendedFDLimit is the minimum recommended soft file descriptor limit for network mode.
// On macOS, the default limit is typically 256, which is insufficient for concurrent RPC/socket pools.
const MinRecommendedFDLimit uint64 = 4096

// FormatFDLimitWarning builds a bold formatted warning message informing the user
// that their system open file descriptor limit is too low for network mode operations.
func FormatFDLimitWarning(currentLimit, minLimit uint64) string {
	return fmt.Sprintf("\033[1;33m⚠️  WARNING:\033[0m \033[1mCurrent file descriptor limit (%d) is below recommended minimum (%d) for network mode.\033[0m\n"+
		"   Running in full network mode on macOS/Linux may cause socket exhaustion or abrupt termination.\n"+
		"   To increase the limit for this shell session, run:\n"+
		"       \033[1;32mulimit -n %d\033[0m\n", currentLimit, minLimit, minLimit)
}

// WarnIfLowFDLimit checks the process file descriptor limit on Unix-like operating systems
// and prints a bold warning to w if the limit is lower than MinRecommendedFDLimit.
// It returns true if a warning was emitted, or false otherwise.
func WarnIfLowFDLimit(w io.Writer) bool {
	if runtime.GOOS == "windows" {
		return false
	}

	limit, err := getSystemFDLimit()
	if err != nil || limit == 0 {
		return false
	}

	if limit < MinRecommendedFDLimit {
		if w == nil {
			w = os.Stderr
		}
		fmt.Fprint(w, FormatFDLimitWarning(limit, MinRecommendedFDLimit))
		return true
	}

	return false
}
