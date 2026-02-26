// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"github.com/dotandev/hintents/internal/terminal"
)

var defaultRenderer terminal.Renderer = terminal.NewANSIRenderer()

// ResetRenderer creates a new renderer (for testing)
func ResetRenderer() {
	defaultRenderer = terminal.NewANSIRenderer()
}

// ColorEnabled reports whether ANSI color output should be used.
func ColorEnabled() bool {
	return defaultRenderer.IsTTY()
}

// Colorize returns text with ANSI color if enabled, otherwise plain text.
func Colorize(text string, color string) string {
	return defaultRenderer.Colorize(text, color)
}

// Success returns a success indicator.
func Success() string {
	return defaultRenderer.Success()
}

// Warning returns a warning indicator.
func Warning() string {
	return defaultRenderer.Warning()
}

// Error returns an error indicator.
func Error() string {
	return defaultRenderer.Error()
}

// Info returns an info indicator.
func Info() string {
	if ColorEnabled() {
		return Colorize("[i]", "blue")
	}
	return "[i]"
}

// Symbol returns a symbol that may be styled; when colors disabled, returns plain ASCII equivalent.
//
//nolint:gocyclo
func Symbol(name string) string {
	return defaultRenderer.Symbol(name)
}

// ContractBoundary returns a visual separator for cross-contract call transitions.
func ContractBoundary(fromContract, toContract string) string {
	if ColorEnabled() {
		return Colorize("--- contract boundary: "+fromContract+" -> "+toContract+" ---", "magenta")
	}
	return "--- contract boundary: " + fromContract + " -> " + toContract + " ---"
}
