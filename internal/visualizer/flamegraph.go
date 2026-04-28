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

  /* Search match highlight - use a high-contrast stroke in dark mode */
  rect[data-highlighted="true"] {
    stroke: #f5c2e7 !important;
    stroke-width: 2px !important;
    paint-order: stroke fill;
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
      color: #333;
    }
    @media (prefers-color-scheme: dark) {
      body { background: #1e1e2e; color: #cdd6f4; }
    }
    .container {
      max-width: 1400px;
      margin: 0 auto;
      background: white;
      border-radius: 12px;
      box-shadow: 0 4px 20px rgba(0,0,0,0.08);
      overflow: hidden;
      display: flex;
      flex-direction: column;
    }
    @media (prefers-color-scheme: dark) {
      .container { background: #181825; box-shadow: 0 4px 20px rgba(0,0,0,0.3); }
    }
    .toolbar {
      padding: 16px 24px;
      border-bottom: 1px solid #eee;
      display: flex;
      gap: 12px;
      align-items: center;
      flex-wrap: wrap;
      background: rgba(255, 255, 255, 0.8);
      backdrop-filter: blur(8px);
      position: sticky;
      top: 0;
      z-index: 100;
    }
    @media (prefers-color-scheme: dark) {
      .toolbar { border-bottom-color: #313244; background: rgba(24, 24, 37, 0.8); }
    }
    .toolbar-group {
      display: flex;
      gap: 8px;
      align-items: center;
    }
    button {
      padding: 8px 16px;
      border: 1px solid #ddd;
      background: white;
      border-radius: 6px;
      cursor: pointer;
      font-size: 14px;
      font-weight: 500;
      transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
      color: #444;
    }
    button:hover {
      background: #f8f8f8;
      border-color: #bbb;
      transform: translateY(-1px);
    }
    button:active {
      transform: translateY(0);
    }
    #resetBtn {
      background: #f0f0f0;
    }
    @media (prefers-color-scheme: dark) {
      button {
        background: #313244;
        border-color: #45475a;
        color: #cdd6f4;
      }
      button:hover {
        background: #45475a;
        border-color: #585b70;
      }
      #resetBtn { background: #45475a; }
    }
    .search-wrapper {
      position: relative;
      flex: 1;
      min-width: 250px;
      max-width: 500px;
    }
    .toolbar input {
      width: 100%;
      padding: 10px 16px;
      padding-right: 100px;
      border: 1.5px solid #ddd;
      border-radius: 8px;
      font-size: 14px;
      outline: none;
      transition: border-color 0.2s;
    }
    .toolbar input:focus {
      border-color: #1e66f5;
    }
    @media (prefers-color-scheme: dark) {
      .toolbar input {
        background: #11111b;
        border-color: #313244;
        color: #cdd6f4;
      }
      .toolbar input:focus { border-color: #89b4fa; }
    }
    .match-counter {
      position: absolute;
      right: 12px;
      top: 50%;
      transform: translateY(-50%);
      font-size: 12px;
      color: #888;
      font-weight: 600;
      pointer-events: none;
    }
    .svg-container {
      padding: 24px;
      overflow: auto;
      position: relative;
      background: #fafafa;
    }
    @media (prefers-color-scheme: dark) {
      .svg-container { background: #1e1e2e; }
    }
    svg {
      display: block;
      margin: 0 auto;
      cursor: default;
      transition: transform 0.3s ease-out;
    }
    rect[data-highlighted="true"] {
      stroke: #d20f39;
      stroke-width: 2px;
      paint-order: stroke fill;
    }
    .tooltip {
      position: fixed;
      background: rgba(17, 17, 27, 0.95);
      color: #cdd6f4;
      padding: 10px 14px;
      border-radius: 8px;
      font-size: 13px;
      pointer-events: none;
      z-index: 1000;
      display: none;
      max-width: 450px;
      word-wrap: break-word;
      box-shadow: 0 4px 12px rgba(0,0,0,0.2);
      border: 1px solid #313244;
    }
    .info {
      padding: 12px 24px;
      font-size: 13px;
      color: #6c7086;
      border-top: 1px solid #eee;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    @media (prefers-color-scheme: dark) {
      .info { color: #a6adc8; border-top-color: #313244; }
    }
    .kdb {
      background: #eee;
      border-radius: 3px;
      padding: 2px 5px;
      font-family: monospace;
      font-size: 11px;
    }
    @media (prefers-color-scheme: dark) {
      .kdb { background: #313244; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="toolbar">
      <div class="toolbar-group">
        <button id="resetBtn" title="Reset view to original size">Reset View</button>
      </div>
      <div class="search-wrapper">
        <input type="text" id="searchInput" placeholder="Search frames, contracts, functions..." autocomplete="off">
        <span class="match-counter" id="matchCounter"></span>
      </div>
      <div class="toolbar-group">
        <button id="clearBtn">Clear</button>
      </div>
    </div>
    <div class="svg-container">
      {{SVG_CONTENT}}
    </div>
    <div class="info">
      <div>
        <span class="kdb">Hover</span> Details &bull; 
        <span class="kdb">Click</span> Zoom &bull; 
        <span class="kdb">Ctrl+F</span> Search
      </div>
      <div>Flamegraph Visualizer</div>
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
      const clearBtn = document.getElementById('clearBtn');
      const matchCounter = document.getElementById('matchCounter');

      let zoomStack = [];
      let originalViewBox = null;
      let cachedNodes = [];
      let debounceTimer = null;

      // Initialize
      if (svg) {
        originalViewBox = svg.getAttribute('viewBox') || '0 0 ' + svg.getAttribute('width') + ' ' + svg.getAttribute('height');
        
        // Cache nodes for performance
        cachedNodes = Array.from(svg.querySelectorAll('g')).map(g => {
          const rect = g.querySelector('rect');
          const title = g.querySelector('title');
          return {
            g,
            rect,
            label: title ? title.textContent.toLowerCase() : '',
            originalFill: rect ? rect.getAttribute('fill') : null,
            originalStroke: rect ? rect.getAttribute('stroke') : null,
            originalStrokeWidth: rect ? rect.getAttribute('stroke-width') : null
          };
        }).filter(n => n.rect);

        setupInteractivity();
      }

      function setupInteractivity() {
        svg.addEventListener('mouseover', handleMouseOver);
        svg.addEventListener('mouseout', handleMouseOut);
        svg.addEventListener('mousemove', handleMouseMove);
        svg.addEventListener('click', handleClick);

        resetBtn.addEventListener('click', resetZoom);
        clearBtn.addEventListener('click', () => {
          searchInput.value = '';
          performSearch();
        });

        searchInput.addEventListener('input', () => {
          clearTimeout(debounceTimer);
          debounceTimer = setTimeout(performSearch, 200);
        });

        // Shortcuts
        window.addEventListener('keydown', (e) => {
          if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
            e.preventDefault();
            searchInput.focus();
          }
        });
      }

      function handleMouseOver(e) {
        const target = e.target;
        const g = target.tagName === 'g' ? target : (target.tagName === 'rect' || target.tagName === 'text' ? target.parentElement : null);
        if (g && g.tagName === 'g') {
          const title = g.querySelector('title');
          if (title) {
            tooltip.innerHTML = title.textContent.replace(/\n/g, '<br>');
            tooltip.style.display = 'block';
          }
        }
      }

      function handleMouseOut() {
        tooltip.style.display = 'none';
      }

      function handleMouseMove(e) {
        if (tooltip.style.display === 'block') {
          const x = e.clientX + 15;
          const y = e.clientY + 15;
          
          // Keep tooltip within viewport
          const width = tooltip.offsetWidth;
          const height = tooltip.offsetHeight;
          const windowWidth = window.innerWidth;
          const windowHeight = window.innerHeight;
          
          tooltip.style.left = (x + width > windowWidth ? x - width - 20 : x) + 'px';
          tooltip.style.top = (y + height > windowHeight ? y - height - 20 : y) + 'px';
        }
      }

      function handleClick(e) {
        let target = e.target;
        if (target.tagName !== 'rect') {
          const g = target.closest('g');
          if (g) target = g.querySelector('rect');
        }
        if (!target || target.tagName !== 'rect') return;

        zoomToRect(target);
      }

      function zoomToRect(rect) {
        const x = parseFloat(rect.getAttribute('x') || 0);
        const y = parseFloat(rect.getAttribute('y') || 0);
        const width = parseFloat(rect.getAttribute('width') || 0);
        const height = parseFloat(rect.getAttribute('height') || 0);

        if (width > 0 && height > 0) {
          zoomStack.push(svg.getAttribute('viewBox'));
          
          const padding = width * 0.05;
          const newX = Math.max(0, x - padding);
          const newWidth = width + (padding * 2);
          
          // Maintain aspect ratio or focus on vertical context
          const currentVB = svg.getAttribute('viewBox').split(' ');
          const currentAspect = parseFloat(currentVB[2]) / parseFloat(currentVB[3]);
          const newHeight = newWidth / currentAspect;
          const newY = Math.max(0, y - (newHeight * 0.1));

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
        
        // Clear previous highlights
        cachedNodes.forEach(node => {
          if (node.rect.getAttribute('data-highlighted') === 'true') {
            node.rect.setAttribute('fill', node.originalFill);
            if (node.originalStroke) node.rect.setAttribute('stroke', node.originalStroke);
            else node.rect.removeAttribute('stroke');
            
            if (node.originalStrokeWidth) node.rect.setAttribute('stroke-width', node.originalStrokeWidth);
            else node.rect.removeAttribute('stroke-width');
            
            node.rect.removeAttribute('data-highlighted');
          }
        });

        if (!query) {
          matchCounter.textContent = '';
          return;
        }

        let matches = [];
        cachedNodes.forEach(node => {
          if (node.label.includes(query)) {
            node.rect.setAttribute('data-highlighted', 'true');
            // Highlight color - use a distinct color that works in light/dark
            node.rect.setAttribute('fill', 'rgb(230, 100, 230)'); 
            node.rect.setAttribute('stroke', '#d20f39');
            node.rect.setAttribute('stroke-width', '2');
            matches.push(node);
          }
        });

        matchCounter.textContent = matches.length > 0 ? matches.length + ' matches' : 'No matches';
        
        if (matches.length > 0) {
          // Zoom to the first match if it's a fresh search
          zoomToRect(matches[0].rect);
        }
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
