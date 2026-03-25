// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package secutil provides stateless security utility primitives.
package secutil

import "runtime"

// Memzero overwrites b with zeros to reduce the window during which sensitive
// key material is readable in a heap or core dump.
//
// The gc compiler does not currently elide stores to heap-allocated slices;
// runtime.KeepAlive provides an additional safety margin by signalling that b
// is still live at this point, preventing any future compiler optimisation from
// removing the zeroing loop.
//
// Callers are responsible for minimising the lifetime of any string containing
// the same key material (e.g. a hex-encoded private key), as Go strings are
// immutable and cannot be cleared by the caller.
func Memzero(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
