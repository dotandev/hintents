// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"fmt"
	"strings"
)

type MockRenderer struct {
	Output []string
	TTY    bool
}

func NewMockRenderer() *MockRenderer {
	return &MockRenderer{
		Output: make([]string, 0),
		TTY:    true,
	}
}

func (m *MockRenderer) Print(a ...any) {
	m.Output = append(m.Output, fmt.Sprint(a...))
}

func (m *MockRenderer) Printf(format string, a ...any) {
	m.Output = append(m.Output, fmt.Sprintf(format, a...))
}

func (m *MockRenderer) Println(a ...any) {
	m.Output = append(m.Output, fmt.Sprintln(a...))
}

func (m *MockRenderer) Colorize(text, color string) string {
	if !m.TTY {
		return text
	}
	return "[" + color + "]" + text + "[reset]"
}

func (m *MockRenderer) Success() string {
	if m.TTY {
		return "[green][OK][reset]"
	}
	return "[OK]"
}

func (m *MockRenderer) Warning() string {
	if m.TTY {
		return "[yellow][!][reset]"
	}
	return "[!]"
}

func (m *MockRenderer) Error() string {
	if m.TTY {
		return "[red][X][reset]"
	}
	return "[X]"
}

func (m *MockRenderer) Symbol(name string) string {
	return "[" + name + "]"
}

func (m *MockRenderer) ClearLine() {
	m.Output = append(m.Output, "[clear]")
}

func (m *MockRenderer) Scanln(a ...any) (int, error) {
	return 0, nil
}

func (m *MockRenderer) IsTTY() bool {
	return m.TTY
}

func (m *MockRenderer) LastOutput() string {
	if len(m.Output) == 0 {
		return ""
	}
	return m.Output[len(m.Output)-1]
}

func (m *MockRenderer) AllOutput() string {
	return strings.Join(m.Output, "")
}
