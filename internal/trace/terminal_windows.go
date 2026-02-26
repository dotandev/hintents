// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build windows || plan9 || js || wasip1

package trace

import "os"

// getTermWidthSys returns 0 on platforms where TIOCGWINSZ is unavailable.
// getTermWidth falls back to COLUMNS or 80.
func getTermWidthSys() int { return 0 }

// watchResize is a no-op on Windows and plan9 (no SIGWINCH equivalent).
func watchResize(_ chan<- os.Signal) {}
