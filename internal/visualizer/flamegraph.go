// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import "strings"

// darkModeCSS contains CSS media queries that adapt flamegraph colors
// when the developer's system is set to dark mode.
const darkModeCSS = `
/* Dark mode support for flamegraph SVGs */
@media (prefers-color-scheme: dark) {
  /* Invert the background from white to a dark surface */
  svg { background-color: #1e1e2e; }

  /* Main text (function names, labels) */
  text { fill: #cdd6f4 !important; }

  /* Title and subtitle */
  text.title { fill: #cdd6f4 !important; }

  /* Details / info bar at the bottom */
  rect.background { fill: #1e1e2e !important; }

  /* Slightly desaturate the flame rectangles for better contrast on dark bg */
  rect[fill] {
    opacity: 0.92;
  }

  /* Search match highlight */
  rect[fill="rgb(230,0,230)"] {
    fill: rgb(200,80,200) !important;
  }
}
`

// InjectDarkMode takes a raw SVG string produced by the inferno crate and
// returns a new SVG string with an embedded <style> block containing CSS
// media queries for dark mode. The injection point is right after the
// opening <svg ...> tag so the styles apply to the entire document.
//
// If the SVG already contains a prefers-color-scheme rule (idempotency guard)
// or does not look like a valid SVG, the original string is returned unchanged.
func InjectDarkMode(svg string) string {
	if svg == "" {
		return svg
	}

	// Idempotency: don't inject twice.
	if strings.Contains(svg, "prefers-color-scheme") {
		return svg
	}

	// Find the end of the opening <svg ...> tag.
	idx := strings.Index(svg, ">")
	if idx < 0 {
		return svg
	}

	// Verify that the tag we found is actually an <svg> tag (very basic check).
	prefix := strings.ToLower(svg[:idx])
	if !strings.Contains(prefix, "<svg") {
		return svg
	}

	// Insert the <style> block right after the opening <svg> tag.
	styleBlock := "\n<style type=\"text/css\">" + darkModeCSS + "</style>\n"
	return svg[:idx+1] + styleBlock + svg[idx+1:]
}
