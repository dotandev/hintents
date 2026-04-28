// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build cgo
// +build cgo

package protocolreg

import "C"

// This file exists to enable CGO for the package, 
// which is required to use import "C" in test files.
