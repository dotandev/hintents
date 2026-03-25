// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sourcemap

import (
	"fmt"
	"os"
	"syscall"
)

func (sc *SourceCache) acquireLock(entryPath string, exclusive bool) (*os.File, error) {
	lp := sc.lockPath(entryPath)
	lf, err := os.OpenFile(lp, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %q: %w", lp, err)
	}

	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(lf.Fd()), how); err != nil {
		_ = lf.Close()
		return nil, fmt.Errorf("flock failed on %q: %w", lp, err)
	}
	return lf, nil
}

func (sc *SourceCache) releaseLock(lf *os.File) {
	_ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
	_ = lf.Close()
}
