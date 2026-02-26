// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"github.com/dotandev/hintents/internal/terminal"
)

var defaultRenderer terminal.Renderer = terminal.NewANSIRenderer()

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
// ContractBoundary returns a visual separator for cross-contract call transitions.
func ContractBoundary(fromContract, toContract string) string {
	if ColorEnabled() {
		return sgrMagenta + sgrBold + "--- contract boundary: " + fromContract + " -> " + toContract + " ---" + sgrReset
	}
	return "--- contract boundary: " + fromContract + " -> " + toContract + " ---"
}

// Success returns a success indicator: colored checkmark if enabled, "[OK]" otherwise.
func Success() string {
	if ColorEnabled() {
		return themeColors("success") + "[OK]" + sgrReset
	}
	return "[OK]"
}

// Warning returns a warning indicator.
func Warning() string {
	return defaultRenderer.Warning()
	if ColorEnabled() {
		return themeColors("warning") + "[!]" + sgrReset
	}
	return "[!]"
}

// Error returns an error indicator.
func Error() string {
	return defaultRenderer.Error()
}

// Symbol returns a symbol that may be styled.
	if ColorEnabled() {
		return themeColors("error") + "[X]" + sgrReset
	}
	return "[X]"
}

// Info returns an info indicator with theme-aware coloring.
func Info() string {
	if ColorEnabled() {
		return themeColors("info") + "[i]" + sgrReset
	}
	return "[i]"
}

// Symbol returns a symbol that may be styled; when colors disabled, returns plain ASCII equivalent.
//
//nolint:gocyclo
func Symbol(name string) string {
	return defaultRenderer.Symbol(name)
}
