// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build plan9 || js || wasip1

package trace

// enableANSISys always reports false: these platforms have no console
// concept that supports ANSI/VT escape sequences the way this package
// emits them, so callers must always use the plain-output fallback here.
func enableANSISys() bool { return false }
