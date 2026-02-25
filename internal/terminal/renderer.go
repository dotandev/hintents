// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package terminal

// Renderer defines the interface for terminal drawing and interaction.
type Renderer interface {
	Print(a ...any)
	Printf(format string, a ...any)
	Println(a ...any)
	Colorize(text, color string) string
	Success() string
	Warning() string
	Error() string
	Symbol(name string) string
	ClearLine()
	Scanln(a ...any) (int, error)
	IsTTY() bool
}
