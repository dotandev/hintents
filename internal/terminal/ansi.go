// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"fmt"
	"os"
	"sync"

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

type ANSIRenderer struct {
	isTTY   bool
	ttyOnce sync.Once
}

func NewANSIRenderer() *ANSIRenderer {
	return &ANSIRenderer{}
}

func (r *ANSIRenderer) IsTTY() bool {
	r.ttyOnce.Do(func() {
		r.isTTY = r.checkTTY()
	})
	return r.isTTY
}

func (r *ANSIRenderer) checkTTY() bool {
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

func (r *ANSIRenderer) Print(a ...any) {
	fmt.Print(a...)
}

func (r *ANSIRenderer) Printf(format string, a ...any) {
	fmt.Printf(format, a...)
}

func (r *ANSIRenderer) Println(a ...any) {
	fmt.Println(a...)
}

func (r *ANSIRenderer) ClearLine() {
	fmt.Print("\r\033[K")
}

func (r *ANSIRenderer) Scanln(a ...any) (int, error) {
	return fmt.Scanln(a...)
}

func (r *ANSIRenderer) Colorize(text, color string) string {
	if !r.IsTTY() {
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

func (r *ANSIRenderer) Success() string {
	if r.IsTTY() {
		return sgrGreen + "[OK]" + sgrReset
	}
	return "[OK]"
}

func (r *ANSIRenderer) Warning() string {
	if r.IsTTY() {
		return sgrYellow + "[!]" + sgrReset
	}
	return "[!]"
}

func (r *ANSIRenderer) Error() string {
	if r.IsTTY() {
		return sgrRed + "[X]" + sgrReset
	}
	return "[X]"
}

func (r *ANSIRenderer) Symbol(name string) string {
	if r.IsTTY() {
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
