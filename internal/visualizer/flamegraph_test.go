// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"
)

func TestInjectDarkMode_BasicInjection(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="600"><rect x="0" y="0" width="100" height="20"/></svg>`
	result := InjectDarkMode(svg)

	if !strings.Contains(result, "prefers-color-scheme: dark") {
		t.Error("InjectDarkMode() did not inject dark mode CSS")
	}
	if !strings.Contains(result, "<style type=\"text/css\">") {
		t.Error("InjectDarkMode() did not inject <style> tag")
	}
	// Ensure the SVG still starts and ends correctly
	if !strings.HasPrefix(result, "<svg") {
		t.Error("InjectDarkMode() corrupted SVG start tag")
	}
	if !strings.HasSuffix(result, "</svg>") {
		t.Error("InjectDarkMode() corrupted SVG end tag")
	}
}

func TestInjectDarkMode_Idempotency(t *testing.T) {
	svg := `<svg><style>@media (prefers-color-scheme: dark) {}</style><rect/></svg>`
	result := InjectDarkMode(svg)

	if result != svg {
		t.Error("InjectDarkMode() should not double-inject when prefers-color-scheme already present")
	}
}

func TestInjectDarkMode_EmptyString(t *testing.T) {
	result := InjectDarkMode("")
	if result != "" {
		t.Error("InjectDarkMode() should return empty string for empty input")
	}
}

func TestInjectDarkMode_InvalidSVG(t *testing.T) {
	input := "this is not an svg"
	result := InjectDarkMode(input)
	if result != input {
		t.Error("InjectDarkMode() should return input unchanged for non-SVG content")
	}
}

func TestInjectDarkMode_PreservesContent(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>hello</text></svg>`
	result := InjectDarkMode(svg)

	if !strings.Contains(result, "<text>hello</text>") {
		t.Error("InjectDarkMode() lost original SVG content")
	}
	if !strings.Contains(result, "background-color: #1e1e2e") {
		t.Error("InjectDarkMode() missing dark background rule")
	}
	if !strings.Contains(result, "fill: #cdd6f4") {
		t.Error("InjectDarkMode() missing dark text color rule")
	}
}
