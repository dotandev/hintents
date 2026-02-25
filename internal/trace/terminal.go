// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"os"
	"strconv"
	"strings"
)

// getTermWidth returns the current terminal column width.
// It queries the platform-specific size first, then falls back to the COLUMNS
// environment variable, then defaults to 80.
func getTermWidth() int {
	if w := getTermWidthSys(); w > 0 {
		return w
	}
	if col := os.Getenv("COLUMNS"); col != "" {
		if w, err := strconv.Atoi(col); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

// wrapField formats a labeled field value so long content reflows within
// termW columns. Continuation lines are indented to align under the value.
//
// Example (termW=30, label="Contract"):
//
//	Contract: CDLZFC3SYJYDZT7
//	          K67VZ75HPJVIEUVN
func wrapField(label, value string, termW int) string {
	prefix := label + ": "
	avail := termW - len(prefix)
	if avail < 20 {
		avail = 20
	}
	if len(value) <= avail {
		return prefix + value
	}

	indent := strings.Repeat(" ", len(prefix))
	var b strings.Builder
	b.WriteString(prefix)
	first := true
	for len(value) > 0 {
		if !first {
			b.WriteByte('\n')
			b.WriteString(indent)
		}
		first = false
		if len(value) <= avail {
			b.WriteString(value)
			break
		}
		// Prefer breaking at a word boundary.
		cut := avail
		if idx := strings.LastIndex(value[:avail], " "); idx > 0 {
			cut = idx + 1
		}
		b.WriteString(strings.TrimRight(value[:cut], " "))
		value = strings.TrimLeft(value[cut:], " ")
	}
	return b.String()
}

// separator returns a horizontal rule capped at 60 columns or the terminal
// width, whichever is smaller, with a minimum of 10 columns.
func separator(termW int) string {
	width := termW
	if width > 60 {
		width = 60
	}
	if width < 10 {
		width = 10
	}
	return strings.Repeat("=", width)
}
