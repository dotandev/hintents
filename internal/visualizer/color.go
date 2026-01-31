// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package visualizer

import (
	"os"

	"github.com/mattn/go-isatty"
)

// ANSI SGR codes
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
// Check order (NO_COLOR has highest priority):
//   - NO_COLOR (https://no-color.org/): if set (any non-empty value), colors are disabled
//   - FORCE_COLOR: if set (e.g. FORCE_COLOR=1), forces colors even when not a TTY (useful in CI)
//   - Non-TTY: when stdout is piped or redirected, colors disabled (no garbage in logs)
//   - TERM=dumb: minimal terminal, no colors
func ColorEnabled() bool {
	// NO_COLOR takes precedence over everything
	if noColor() {
		return false
	}
	// FORCE_COLOR allows colors in pipes/CI (e.g. GitHub Actions with ANSI support)
	if forceColor() {
		return true
	}
	// Not a real terminal (pipe, redirect, log file)
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return false
	}
	// Dumb terminal (e.g. emacs shell)
	if termDumb() {
		return false
	}
	return true
}

// noColor returns true if NO_COLOR is set (presence = no color per no-color.org).
func noColor() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

// forceColor returns true if FORCE_COLOR is set to a non-empty value.
func forceColor() bool {
	return os.Getenv("FORCE_COLOR") != ""
}

// termDumb returns true for minimal terminals that don't support ANSI.
func termDumb() bool {
	return os.Getenv("TERM") == "dumb"
}

// Colorize returns text with ANSI color if enabled, otherwise plain text.
func Colorize(text string, color string) string {
	if !ColorEnabled() {
		return text
	}
	return ansiWrap(text, color)
}

func ansiWrap(text, color string) string {
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

// Success returns a success indicator: colored checkmark if enabled, "[OK]" otherwise.
func Success() string {
	if ColorEnabled() {
		return sgrGreen + "✓" + sgrReset
	}
	return "[OK]"
}

// Warning returns a warning indicator: colored warning sign if enabled, "[!]" otherwise.
func Warning() string {
	if ColorEnabled() {
		return sgrYellow + "⚠" + sgrReset
	}
	return "[!]"
}

// Error returns an error indicator: colored X if enabled, "[X]" otherwise.
func Error() string {
	if ColorEnabled() {
		return sgrRed + "✗" + sgrReset
	}
	return "[X]"
}

// Symbol returns a symbol that may be styled; when colors disabled, returns plain ASCII equivalent.
func Symbol(name string) string {
	if ColorEnabled() {
		switch name {
		case "check":
			return "✓"
		case "cross":
			return "✗"
		case "warn":
			return "⚠"
		case "arrow_r":
			return "➡️"
		case "arrow_l":
			return "⬅️"
		case "target":
			return "🎯"
		case "pin":
			return "📍"
		case "wrench":
			return "🔧"
		case "chart":
			return "📊"
		case "list":
			return "📋"
		case "play":
			return "▶️"
		case "book":
			return "📖"
		case "wave":
			return "👋"
		case "magnify":
			return "🔍"
		case "logs":
			return "📋"
		case "events":
			return "📡"
		default:
			return name
		}
	}
	// Plain ASCII fallbacks for non-TTY / CI
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
