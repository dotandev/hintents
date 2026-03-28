// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
)

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

// interactiveHTML contains the HTML template for an interactive flamegraph.
// It embeds the SVG and adds JavaScript for hover tooltips, click-to-zoom,
// and search/highlight functionality. All assets are inlined for a standalone file.
const interactiveHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Interactive Flamegraph</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: #f5f5f5;
      padding: 20px;
    }
    @media (prefers-color-scheme: dark) {
      body { background: #1e1e2e; color: #cdd6f4; }
    }
    .container {
      max-width: 1400px;
      margin: 0 auto;
      background: white;
      border-radius: 8px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.1);
      overflow: hidden;
    }
    @media (prefers-color-scheme: dark) {
      .container { background: #181825; box-shadow: 0 2px 8px rgba(0,0,0,0.3); }
    }
    .toolbar {
      padding: 15px 20px;
      border-bottom: 1px solid #e0e0e0;
      display: flex;
      gap: 10px;
      align-items: center;
      flex-wrap: wrap;
    }
    @media (prefers-color-scheme: dark) {
      .toolbar { border-bottom-color: #313244; }
    }
    .toolbar button {
      padding: 8px 16px;
      border: 1px solid #ccc;
      background: white;
      border-radius: 4px;
      cursor: pointer;
      font-size: 14px;
      transition: all 0.2s;
    }
    .toolbar button:hover {
      background: #f0f0f0;
      border-color: #999;
    }
    @media (prefers-color-scheme: dark) {
      .toolbar button {
        background: #313244;
        border-color: #45475a;
        color: #cdd6f4;
      }
      .toolbar button:hover {
        background: #45475a;
        border-color: #585b70;
      }
    }
    .toolbar input {
      padding: 8px 12px;
      border: 1px solid #ccc;
      border-radius: 4px;
      font-size: 14px;
      flex: 1;
      min-width: 200px;
      max-width: 400px;
    }
    @media (prefers-color-scheme: dark) {
      .toolbar input {
        background: #313244;
        border-color: #45475a;
        color: #cdd6f4;
      }
    }
    .svg-container {
      padding: 20px;
      overflow: auto;
      position: relative;
    }
    svg {
      display: block;
      margin: 0 auto;
      cursor: default;
    }
    .tooltip {
      position: fixed;
      background: rgba(0, 0, 0, 0.9);
      color: white;
      padding: 8px 12px;
      border-radius: 4px;
      font-size: 12px;
      pointer-events: none;
      z-index: 1000;
      display: none;
      max-width: 400px;
      word-wrap: break-word;
    }
    @media (prefers-color-scheme: dark) {
      .tooltip { background: rgba(30, 30, 46, 0.95); border: 1px solid #45475a; }
    }
    .info {
      padding: 10px 20px;
      font-size: 13px;
      color: #666;
      border-top: 1px solid #e0e0e0;
    }
    @media (prefers-color-scheme: dark) {
      .info { color: #a6adc8; border-top-color: #313244; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="toolbar">
      <button id="resetBtn">Reset Zoom</button>
      <input type="text" id="searchInput" placeholder="Search frames (e.g., function name)...">
      <button id="searchBtn">Search</button>
      <button id="clearBtn">Clear</button>
    </div>
    <div class="svg-container">
      {{SVG_CONTENT}}
    </div>
    <div class="info">
      <strong>Interactions:</strong> Hover for details • Click to zoom • Search to highlight
    </div>
  </div>
  <div class="tooltip" id="tooltip"></div>

  <script>
    (function() {
      'use strict';

      const svg = document.querySelector('svg');
      const tooltip = document.getElementById('tooltip');
      const resetBtn = document.getElementById('resetBtn');
      const searchInput = document.getElementById('searchInput');
      const searchBtn = document.getElementById('searchBtn');
      const clearBtn = document.getElementById('clearBtn');

      let zoomStack = [];
      let originalViewBox = null;

      // Initialize
      if (svg) {
        originalViewBox = svg.getAttribute('viewBox') || '0 0 ' + svg.getAttribute('width') + ' ' + svg.getAttribute('height');
        setupInteractivity();
      }

      function setupInteractivity() {
        // Hover tooltips
        svg.addEventListener('mouseover', handleMouseOver);
        svg.addEventListener('mouseout', handleMouseOut);
        svg.addEventListener('mousemove', handleMouseMove);

        // Click to zoom
        svg.addEventListener('click', handleClick);

        // Toolbar actions
        resetBtn.addEventListener('click', resetZoom);
        searchBtn.addEventListener('click', performSearch);
        clearBtn.addEventListener('click', clearSearch);
        searchInput.addEventListener('keypress', (e) => {
          if (e.key === 'Enter') performSearch();
        });
      }

      function handleMouseOver(e) {
        const target = e.target;
        if (target.tagName === 'rect' || target.tagName === 'g') {
          const g = target.tagName === 'g' ? target : target.parentElement;
          const title = g.querySelector('title');
          if (title) {
            tooltip.textContent = title.textContent;
            tooltip.style.display = 'block';
          }
        }
      }

      function handleMouseOut(e) {
        tooltip.style.display = 'none';
      }

      function handleMouseMove(e) {
        if (tooltip.style.display === 'block') {
          tooltip.style.left = (e.clientX + 10) + 'px';
          tooltip.style.top = (e.clientY + 10) + 'px';
        }
      }

      function handleClick(e) {
        const target = e.target;
        if (target.tagName !== 'rect') return;

        const g = target.parentElement;
        const rect = target;

        // Get bounding box
        const x = parseFloat(rect.getAttribute('x') || 0);
        const y = parseFloat(rect.getAttribute('y') || 0);
        const width = parseFloat(rect.getAttribute('width') || 0);
        const height = parseFloat(rect.getAttribute('height') || 0);

        if (width > 0 && height > 0) {
          // Save current viewBox to stack
          zoomStack.push(svg.getAttribute('viewBox'));

          // Calculate new viewBox with some padding
          const padding = width * 0.1;
          const newX = Math.max(0, x - padding);
          const newY = Math.max(0, y - padding);
          const newWidth = width + (padding * 2);
          const newHeight = height * 10; // Show more vertical context

          svg.setAttribute('viewBox', newX + ' ' + newY + ' ' + newWidth + ' ' + newHeight);
        }
      }

      function resetZoom() {
        if (originalViewBox) {
          svg.setAttribute('viewBox', originalViewBox);
          zoomStack = [];
        }
      }

      function performSearch() {
        const query = searchInput.value.trim().toLowerCase();
        if (!query) return;

        clearSearch();

        const allGroups = svg.querySelectorAll('g');
        let matchCount = 0;

        allGroups.forEach(g => {
          const title = g.querySelector('title');
          const rect = g.querySelector('rect');

          if (title && rect && title.textContent.toLowerCase().includes(query)) {
            // Highlight matching frames
            const originalFill = rect.getAttribute('fill');
            rect.setAttribute('data-original-fill', originalFill);
            rect.setAttribute('fill', 'rgb(230, 100, 230)');
            rect.setAttribute('data-highlighted', 'true');
            matchCount++;
          }
        });

        if (matchCount === 0) {
          alert('No matches found for: ' + query);
        }
      }

      function clearSearch() {
        const highlighted = svg.querySelectorAll('rect[data-highlighted="true"]');
        highlighted.forEach(rect => {
          const originalFill = rect.getAttribute('data-original-fill');
          if (originalFill) {
            rect.setAttribute('fill', originalFill);
          }
          rect.removeAttribute('data-original-fill');
          rect.removeAttribute('data-highlighted');
        });
      }
    })();
  </script>
</body>
</html>`

// GenerateInteractiveHTML takes an SVG flamegraph string and wraps it in a
// standalone HTML file with interactive features including:
// - Hover tooltips showing frame details
// - Click-to-zoom functionality with reset
// - Search/highlight to find frames by name
// - Responsive design with dark mode support
//
// The output is a single self-contained HTML file with all CSS and JavaScript
// inlined, requiring no external dependencies or network requests.
func GenerateInteractiveHTML(svg string) string {
	if svg == "" {
		return ""
	}

	// Inject dark mode CSS into the SVG first
	enhancedSVG := InjectDarkMode(svg)

	// Embed the SVG into the HTML template
	html := strings.Replace(interactiveHTML, "{{SVG_CONTENT}}", enhancedSVG, 1)

	return html
}

// ExportFormat represents the output format for flamegraph export
type ExportFormat string

const (
	// FormatSVG exports a raw SVG file with dark mode support
	FormatSVG ExportFormat = "svg"
	// FormatHTML exports an interactive standalone HTML file
	FormatHTML ExportFormat = "html"
)

// GetFileExtension returns the appropriate file extension for the export format
func (f ExportFormat) GetFileExtension() string {
	switch f {
	case FormatHTML:
		return ".flamegraph.html"
	case FormatSVG:
		return ".flamegraph.svg"
	default:
		return ".flamegraph.svg"
	}
}

// ExportFlamegraph generates the appropriate output format for a flamegraph
func ExportFlamegraph(svg string, format ExportFormat) string {
	switch format {
	case FormatHTML:
		return GenerateInteractiveHTML(svg)
	case FormatSVG:
		return InjectDarkMode(svg)
	default:
		return InjectDarkMode(svg)
	}
}
