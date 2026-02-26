// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"

	"github.com/mattn/go-isatty"
)

// ANSI SGR codes used for colorized output.
const (
	sgrReset   = "\033[0m"
	sgrRed     = "\033[31m"
	sgrGreen   = "\033[32m"
	sgrYellow  = "\033[33m"
	sgrBlue    = "\033[34m"
	sgrMagenta = "\033[35m"
	sgrCyan    = "\033[36m"
	sgrDim     = "\033[2m"
	sgrBold    = "\033[1m"
)

// ColorEnabled reports whether ANSI color output should be used.
func ColorEnabled() bool {
	// NO_COLOR must always take precedence.
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

// Colorize returns text with ANSI color if enabled, otherwise plain text.
func Colorize(text string, color string) string {
	if !ColorEnabled() {
		return text
	}

	var code string
	switch color {
	case "red":
		code = sgrRed
	case "green":
		code = sgrGreen
	case "yellow":
		code = sgrYellow
	case "blue":
		code = sgrBlue
	case "magenta":
		code = sgrMagenta
	case "cyan":
		code = sgrCyan
	case "dim":
		code = sgrDim
	case "bold":
		code = sgrBold
	default:
		return text
	}

	return code + text + sgrReset
}

// ContractBoundary returns a visual separator for cross-contract call transitions.
func ContractBoundary(fromContract, toContract string) string {
	line := "--- contract boundary: " + fromContract + " -> " + toContract + " ---"
	if !ColorEnabled() {
		return line
	}
	return sgrMagenta + sgrBold + line + sgrReset
}

// Success returns a success indicator.
func Success() string {
	if !ColorEnabled() {
		return "[OK]"
	}
	return themeColors("success") + "[OK]" + sgrReset
}

// Warning returns a warning indicator.
func Warning() string {
	if !ColorEnabled() {
		return "[!]"
	}
	return themeColors("warning") + "[!]" + sgrReset
}

// Error returns an error indicator.
func Error() string {
	if !ColorEnabled() {
		return "[X]"
	}
	return themeColors("error") + "[X]" + sgrReset
}

// Info returns an info indicator.
func Info() string {
	if !ColorEnabled() {
		return "[i]"
	}
	return themeColors("info") + "[i]" + sgrReset
}

// Symbol returns a symbol name rendered as ASCII markers.
//
//nolint:gocyclo
func Symbol(name string) string {
	if ColorEnabled() {
		switch name {
		case "check":
			return "[OK]"
		case "cross":
			return "[FAIL]"
		case "warn":
			return "[!]"
		case "arrow_r":
			return "->"
		case "arrow_l":
			return "<-"
		case "target":
			return "[TARGET]"
		case "pin":
			return "*"
		case "wrench":
			return "[TOOL]"
		case "chart":
			return "[STATS]"
		case "list":
			return "[LIST]"
		case "play":
			return "[PLAY]"
		case "book":
			return "[DOC]"
		case "wave":
			return "[HELLO]"
		case "magnify":
			return "[SEARCH]"
		case "logs":
			return "[LOGS]"
		case "events":
			return "[NET]"
		default:
			return name
		}
	}

	switch name {
	case "check":
		return "[OK]"
	case "cross":
		return "[X]"
	case "warn":
		return "[!]"
	case "arrow_r":
		return "->"
	case "arrow_l":
		return "<-"
	case "target":
		return ">>"
	case "pin":
		return "*"
	case "wrench":
		return "[*]"
	case "chart":
		return "[#]"
	case "list":
		return "[.]"
	case "play":
		return ">"
	case "book":
		return "[?]"
	case "wave":
		return ""
	case "magnify":
		return "[?]"
	case "logs":
		return "[Logs]"
	case "events":
		return "[Events]"
	default:
		return name
	}
}
