// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bufio"
	"fmt"
	"os"
)

type Key int

const (
	KeyUnknown Key = iota
	KeyTab
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyQuit
	KeySlash
	KeyEscape
)

func (k Key) String() string {
	switch k {
	case KeyTab:
		return "Tab"
	case KeyUp:
		return "↑/k"
	case KeyDown:
		return "↓/j"
	case KeyLeft:
		return "←/h"
	case KeyRight:
		return "→/l"
	case KeyEnter:
		return "Enter"
	case KeyQuit:
		return "q"
	case KeySlash:
		return "/"
	case KeyEscape:
		return "Esc"
	default:
		return "?"
	}
}

// KeyHelp returns a compact one-line help string for the status bar.
func KeyHelp() string {
	return "Tab:switch-pane  ↑↓:navigate  Enter:expand  q:quit  /:search"
}

type KeyReader struct {
	r *bufio.Reader
}

// NewKeyReader creates a KeyReader reading from os.Stdin.
func NewKeyReader() *KeyReader {
	return &KeyReader{r: bufio.NewReader(os.Stdin)}
}

func (kr *KeyReader) Read() (Key, error) {
	b, err := kr.r.ReadByte()
	if err != nil {
		return KeyUnknown, err
	}

	switch b {
	case '\t': // ASCII 0x09
		return KeyTab, nil
	case '\r', '\n': // CR / LF
		return KeyEnter, nil
	case 'q', 'Q':
		return KeyQuit, nil
	case 'k':
		return KeyUp, nil
	case 'j':
		return KeyDown, nil
	case 'h':
		return KeyLeft, nil
	case 'l':
		return KeyRight, nil
	case '/':
		return KeySlash, nil
	case 0x1b: // ESC — may be start of an ANSI escape sequence
		return kr.readEscape()
	case 0x03: // Ctrl-C
		return KeyQuit, nil
	}
	return KeyUnknown, nil
}

// readEscape parses ANSI CSI sequences after the leading ESC byte.
func (kr *KeyReader) readEscape() (Key, error) {
	next, err := kr.r.ReadByte()
	if err != nil {
		return KeyEscape, nil // bare Esc
	}
	if next != '[' {
		return KeyEscape, nil // ESC not followed by '[' — treat as Esc
	}

	// Read CSI parameter bytes until a final byte in 0x40–0x7E
	var seq []byte
	for {
		c, err := kr.r.ReadByte()
		if err != nil {
			break
		}
		seq = append(seq, c)
		if c >= 0x40 && c <= 0x7E {
			break
		}
	}

	if len(seq) == 0 {
		return KeyUnknown, nil
	}

	switch seq[len(seq)-1] {
	case 'A': // ESC[A
		return KeyUp, nil
	case 'B': // ESC[B
		return KeyDown, nil
	case 'C': // ESC[C
		return KeyRight, nil
	case 'D': // ESC[D
		return KeyLeft, nil
	}

	return KeyUnknown, nil
}

func TermSize() (width, height int) {
	width = readEnvInt("COLUMNS", 80)
	height = readEnvInt("LINES", 24)
	return width, height
}

func readEnvInt(name string, fallback int) int {
	val := os.Getenv(name)
	if val == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
		return n
	}
	return fallback
}
