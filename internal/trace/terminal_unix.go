// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !plan9 && !js && !wasip1

package trace

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

// ioctlWinsize mirrors the kernel winsize struct used by TIOCGWINSZ.
type ioctlWinsize struct {
	Row, Col       uint16
	Xpixel, Ypixel uint16
}

// getTermWidthSys queries the terminal width via TIOCGWINSZ ioctl.
// Returns 0 if stdout is not a terminal or the call fails.
func getTermWidthSys() int {
	var ws ioctlWinsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.Col == 0 {
		return 0
	}
	return int(ws.Col)
}

// watchResize registers ch to receive os.Signal notifications on SIGWINCH
// (terminal window resize). Call signal.Stop(ch) to deregister.
func watchResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}
