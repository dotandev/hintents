//go:build cgo
// +build cgo

package protocolreg

import "C"

// This file exists to enable CGO for the package, 
// which is required to use import "C" in test files.
