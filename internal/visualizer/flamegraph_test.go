// Copyright 2026 Erst Users
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

func TestGenerateInteractiveHTML_BasicGeneration(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="600">
<rect x="0" y="0" width="100" height="20" fill="red"/>
<text x="10" y="15">test_function</text>
</svg>`

	html := GenerateInteractiveHTML(svg)

	// Verify HTML structure
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("GenerateInteractiveHTML() did not generate valid HTML")
	}
	if !strings.Contains(html, "<svg") {
		t.Error("GenerateInteractiveHTML() did not embed SVG")
	}
	if !strings.Contains(html, "test_function") {
		t.Error("GenerateInteractiveHTML() lost SVG content")
	}

	// Verify interactive features
	if !strings.Contains(html, "handleMouseOver") {
		t.Error("GenerateInteractiveHTML() missing hover functionality")
	}
	if !strings.Contains(html, "handleClick") {
		t.Error("GenerateInteractiveHTML() missing click-to-zoom functionality")
	}
	if !strings.Contains(html, "performSearch") {
		t.Error("GenerateInteractiveHTML() missing search functionality")
	}

	// Verify dark mode support
	if !strings.Contains(html, "prefers-color-scheme") {
		t.Error("GenerateInteractiveHTML() missing dark mode support")
	}

	// Verify standalone (no external dependencies)
	if strings.Contains(html, "src=\"http") || strings.Contains(html, "href=\"http") {
		t.Error("GenerateInteractiveHTML() contains external dependencies")
	}
}

func TestGenerateInteractiveHTML_EmptyInput(t *testing.T) {
	result := GenerateInteractiveHTML("")
	if result != "" {
		t.Error("GenerateInteractiveHTML() should return empty string for empty input")
	}
}

func TestGenerateInteractiveHTML_PreservesSVGContent(t *testing.T) {
	svg := `<svg><g><title>main() - 50%</title><rect fill="orange"/></g></svg>`
	html := GenerateInteractiveHTML(svg)

	if !strings.Contains(html, "main() - 50%") {
		t.Error("GenerateInteractiveHTML() did not preserve SVG title content")
	}
	if !strings.Contains(html, "fill=\"orange\"") {
		t.Error("GenerateInteractiveHTML() did not preserve SVG attributes")
	}
}

func TestExportFormat_GetFileExtension(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{FormatSVG, ".flamegraph.svg"},
		{FormatHTML, ".flamegraph.html"},
		{ExportFormat("unknown"), ".flamegraph.svg"},
	}

	for _, tt := range tests {
		result := tt.format.GetFileExtension()
		if result != tt.expected {
			t.Errorf("GetFileExtension(%v) = %v, want %v", tt.format, result, tt.expected)
		}
	}
}

func TestExportFlamegraph_SVGFormat(t *testing.T) {
	svg := `<svg><rect/></svg>`
	result := ExportFlamegraph(svg, FormatSVG)

	if !strings.Contains(result, "<svg") {
		t.Error("ExportFlamegraph(FormatSVG) did not return SVG")
	}
	if !strings.Contains(result, "prefers-color-scheme") {
		t.Error("ExportFlamegraph(FormatSVG) did not inject dark mode")
	}
	if strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("ExportFlamegraph(FormatSVG) should not return HTML")
	}
}

func TestExportFlamegraph_HTMLFormat(t *testing.T) {
	svg := `<svg><rect/></svg>`
	result := ExportFlamegraph(svg, FormatHTML)

	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("ExportFlamegraph(FormatHTML) did not return HTML")
	}
	if !strings.Contains(result, "<svg") {
		t.Error("ExportFlamegraph(FormatHTML) did not embed SVG")
	}
	if !strings.Contains(result, "handleClick") {
		t.Error("ExportFlamegraph(FormatHTML) missing interactive features")
	}
}

func TestExportFlamegraph_DefaultFormat(t *testing.T) {
	svg := `<svg><rect/></svg>`
	result := ExportFlamegraph(svg, ExportFormat("invalid"))

	// Should default to SVG
	if !strings.Contains(result, "<svg") {
		t.Error("ExportFlamegraph(invalid) did not default to SVG")
	}
	if strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("ExportFlamegraph(invalid) should default to SVG, not HTML")
	}
}
