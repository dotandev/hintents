// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"os"
)

// MouseButton represents different mouse button types
type MouseButton int

const (
	LeftButton MouseButton = iota
	MiddleButton
	RightButton
	ScrollUp
	ScrollDown
)

// MouseEvent represents a mouse event
type MouseEvent struct {
	Button MouseButton
	Col    int
	Row    int
	X      int
	Y      int
}

// MouseTracker enables and manages mouse input in the terminal
type MouseTracker struct {
	enabled bool
}

// NewMouseTracker creates a new mouse tracker
func NewMouseTracker() *MouseTracker {
	return &MouseTracker{enabled: false}
}

// Enable enables mouse tracking in the terminal
func (mt *MouseTracker) Enable() error {
	if mt.enabled {
		return nil
	}

	// Enable mouse tracking: report button presses and release
	// SGR1006 mode for better compatibility
	fmt.Fprint(os.Stdout, "\x1b[?1000h") // Enable basic mouse reporting
	fmt.Fprint(os.Stdout, "\x1b[?1006h") // Enable SGR mode (extended coordinates)
	fmt.Fprint(os.Stdout, "\x1b[?1015h") // Enable URXVT mode

	mt.enabled = true
	return nil
}

// Disable disables mouse tracking in the terminal
func (mt *MouseTracker) Disable() error {
	if !mt.enabled {
		return nil
	}

	// Disable all mouse tracking modes
	fmt.Fprint(os.Stdout, "\x1b[?1000l") // Disable basic mouse reporting
	fmt.Fprint(os.Stdout, "\x1b[?1006l") // Disable SGR mode
	fmt.Fprint(os.Stdout, "\x1b[?1015l") // Disable URXVT mode

	mt.enabled = false
	return nil
}

// ParseMouseEvent parses an ANSI mouse event sequence
// Handles both basic and SGR (1006) formats
func ParseMouseEvent(sequence string) (*MouseEvent, error) {
	if len(sequence) < 3 {
		return nil, fmt.Errorf("invalid mouse sequence")
	}

	// SGR1006 format: \x1b[<buttons;col;row;M|m
	if sequence[0] == '<' {
		return parseSGRMouseEvent(sequence)
	}

	// Basic format: \x1b[Mcxy (where x, y are column and row)
	return parseBasicMouseEvent(sequence)
}

// parseSGRMouseEvent parses SGR1006 format mouse events
func parseSGRMouseEvent(sequence string) (*MouseEvent, error) {
	// Format: \x1b[<button;col;row;M|m
	parts := make([]int, 0)
	current := ""

	for i := 1; i < len(sequence)-1; i++ {
		c := sequence[i]
		if c == ';' {
			if current != "" {
				var num int
				fmt.Sscanf(current, "%d", &num)
				parts = append(parts, num)
				current = ""
			}
		} else if c >= '0' && c <= '9' {
			current += string(c)
		}
	}

	if current != "" {
		var num int
		fmt.Sscanf(current, "%d", &num)
		parts = append(parts, num)
	}

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SGR mouse sequence")
	}

	button := parts[0]
	col := parts[1] - 1 // Convert to 0-based
	row := parts[2] - 1 // Convert to 0-based

	evt := &MouseEvent{
		Col: col,
		Row: row,
		X:   col,
		Y:   row,
	}

	// Determine button type
	switch button {
	case 0:
		evt.Button = LeftButton
	case 1:
		evt.Button = MiddleButton
	case 2:
		evt.Button = RightButton
	case 64:
		evt.Button = ScrollUp
	case 65:
		evt.Button = ScrollDown
	}

	return evt, nil
}

// parseBasicMouseEvent parses basic format mouse events
func parseBasicMouseEvent(sequence string) (*MouseEvent, error) {
	if len(sequence) < 3 {
		return nil, fmt.Errorf("invalid basic mouse sequence")
	}

	// Basic format: Mcxy where c is column, x and y are row
	buttonByte := sequence[0]
	col := int(sequence[1]) - 33
	row := int(sequence[2]) - 33

	evt := &MouseEvent{
		Col: col,
		Row: row,
		X:   col,
		Y:   row,
	}

	// Determine button type from button byte
	switch buttonByte & 0x03 {
	case 0:
		evt.Button = LeftButton
	case 1:
		evt.Button = MiddleButton
	case 2:
		evt.Button = RightButton
	}

	return evt, nil
}

// IsClickEvent returns true if this is a mouse click event
func (me *MouseEvent) IsClickEvent() bool {
	return me.Button == LeftButton || me.Button == MiddleButton || me.Button == RightButton
}

// IsScrollEvent returns true if this is a mouse scroll event
func (me *MouseEvent) IsScrollEvent() bool {
	return me.Button == ScrollUp || me.Button == ScrollDown
}
