# Flamegraph Export Architecture

## Overview

This document describes the architecture and data flow for the interactive flamegraph export feature.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Command                             │
│                                                                   │
│  $ erst debug --profile --profile-format html <tx-hash>         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    CLI Layer (Go)                                │
│                  internal/cmd/debug.go                           │
│                                                                   │
│  1. Parse flags (--profile, --profile-format)                   │
│  2. Execute transaction simulation                               │
│  3. Receive simulation response with flamegraph SVG             │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                 Rust Simulator (erst-sim)                        │
│                  simulator/src/main.rs                           │
│                                                                   │
│  1. Execute transaction in soroban-env-host                     │
│  2. Collect CPU/Memory profiling data                           │
│  3. Generate folded stack format                                 │
│  4. Use inferno crate to create SVG flamegraph                  │
│  5. Return SVG in simulation response                            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              Visualizer Layer (Go)                               │
│           internal/visualizer/flamegraph.go                      │
│                                                                   │
│  ┌──────────────────────────────────────────────────┐          │
│  │  ExportFlamegraph(svg, format)                   │          │
│  │                                                   │          │
│  │  Switch on format:                               │          │
│  │  ┌─────────────────┬─────────────────┐          │          │
│  │  │   FormatHTML    │    FormatSVG    │          │          │
│  │  └────────┬────────┴────────┬────────┘          │          │
│  │           │                  │                    │          │
│  │           ▼                  ▼                    │          │
│  │  ┌────────────────┐  ┌──────────────┐          │          │
│  │  │ Generate       │  │ InjectDark   │          │          │
│  │  │ Interactive    │  │ Mode(svg)    │          │          │
│  │  │ HTML(svg)      │  │              │          │          │
│  │  └────────┬───────┘  └──────┬───────┘          │          │
│  │           │                  │                    │          │
│  │           ▼                  ▼                    │          │
│  │  ┌────────────────┐  ┌──────────────┐          │          │
│  │  │ HTML with      │  │ SVG with     │          │          │
│  │  │ embedded SVG,  │  │ dark mode    │          │          │
│  │  │ CSS, and JS    │  │ CSS          │          │          │
│  │  └────────────────┘  └──────────────┘          │          │
│  └──────────────────────────────────────────────────┘          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    File System                                   │
│                                                                   │
│  Write to disk:                                                  │
│  • <tx-hash>.flamegraph.html  (interactive)                     │
│  • <tx-hash>.flamegraph.svg   (raw SVG)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    User Browser                                  │
│                                                                   │
│  Open HTML file → Interactive features:                         │
│  • Hover tooltips (JavaScript event listeners)                  │
│  • Click-to-zoom (SVG viewBox manipulation)                     │
│  • Search/highlight (DOM manipulation)                           │
│  • Dark mode (CSS media queries)                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. Profiling Data Collection (Rust)

```rust
// simulator/src/main.rs
if request.profile.unwrap_or(false) {
    let folded_data = format!("Total;CPU {}\nTotal;Memory {}\n", cpu_insns, mem_bytes);
    let mut result_vec = Vec::new();
    let mut options = inferno::flamegraph::Options::default();
    
    inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result_vec)?;
    flamegraph_svg = Some(String::from_utf8_lossy(&result_vec).to_string());
}
```

### 2. SVG Enhancement (Go)

```go
// internal/visualizer/flamegraph.go
func InjectDarkMode(svg string) string {
    // Find opening <svg> tag
    idx := strings.Index(svg, ">")
    
    // Insert <style> block with dark mode CSS
    styleBlock := "\n<style type=\"text/css\">" + darkModeCSS + "</style>\n"
    return svg[:idx+1] + styleBlock + svg[idx+1:]
}
```

### 3. HTML Generation (Go)

```go
// internal/visualizer/flamegraph.go
func GenerateInteractiveHTML(svg string) string {
    // Inject dark mode CSS into SVG
    enhancedSVG := InjectDarkMode(svg)
    
    // Embed SVG into HTML template
    html := strings.Replace(interactiveHTML, "{{SVG_CONTENT}}", enhancedSVG, 1)
    
    return html
}
```

### 4. Export Decision (Go)

```go
// internal/cmd/debug.go
var format visualizer.ExportFormat
switch ProfileFormatFlag {
case "html":
    format = visualizer.FormatHTML
case "svg":
    format = visualizer.FormatSVG
default:
    format = visualizer.FormatHTML
}

content := visualizer.ExportFlamegraph(lastSimResp.Flamegraph, format)
filename := txHash + format.GetFileExtension()
os.WriteFile(filename, []byte(content), 0644)
```

## Interactive Features Implementation

### Hover Tooltips

```javascript
// Embedded in HTML template
svg.addEventListener('mouseover', handleMouseOver);

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
```

### Click-to-Zoom

```javascript
// Embedded in HTML template
function handleClick(e) {
    const rect = e.target;
    const x = parseFloat(rect.getAttribute('x') || 0);
    const y = parseFloat(rect.getAttribute('y') || 0);
    const width = parseFloat(rect.getAttribute('width') || 0);
    const height = parseFloat(rect.getAttribute('height') || 0);
    
    // Save current viewBox to stack
    zoomStack.push(svg.getAttribute('viewBox'));
    
    // Calculate new viewBox
    const newViewBox = `${x} ${y} ${width} ${height * 10}`;
    svg.setAttribute('viewBox', newViewBox);
}
```

### Search and Highlight

```javascript
// Embedded in HTML template
function performSearch() {
    const query = searchInput.value.trim().toLowerCase();
    const allGroups = svg.querySelectorAll('g');
    
    allGroups.forEach(g => {
        const title = g.querySelector('title');
        const rect = g.querySelector('rect');
        
        if (title && rect && title.textContent.toLowerCase().includes(query)) {
            const originalFill = rect.getAttribute('fill');
            rect.setAttribute('data-original-fill', originalFill);
            rect.setAttribute('fill', 'rgb(230, 100, 230)');
            rect.setAttribute('data-highlighted', 'true');
        }
    });
}
```

## File Structure

```
internal/
├── visualizer/
│   ├── flamegraph.go           # Core implementation
│   │   ├── InjectDarkMode()    # Add dark mode CSS to SVG
│   │   ├── GenerateInteractiveHTML()  # Wrap SVG in HTML
│   │   ├── ExportFlamegraph()  # Main export function
│   │   └── ExportFormat        # Format enum (HTML/SVG)
│   └── flamegraph_test.go      # Comprehensive tests
│
├── cmd/
│   ├── root.go                 # CLI flag definitions
│   │   ├── ProfileFlag         # Enable profiling
│   │   └── ProfileFormatFlag   # Choose format
│   └── debug.go                # Export logic
│       └── Export flamegraph after simulation
│
└── simulator/
    └── schema.go               # Response structure
        └── Flamegraph field    # SVG string from Rust

simulator/
└── src/
    └── main.rs                 # Rust simulator
        └── Generate SVG using inferno crate

docs/
├── INTERACTIVE_FLAMEGRAPH.md  # Full documentation
├── FLAMEGRAPH_QUICK_START.md  # Quick reference
├── FLAMEGRAPH_ARCHITECTURE.md # This file
└── examples/
    └── sample_flamegraph.html  # Live demo
```

## Design Decisions

### 1. Why Inline Everything?

**Decision**: Embed all CSS and JavaScript directly in the HTML file.

**Rationale**:
- No external dependencies or CDN requirements
- Works offline without network access
- Single file is easy to share and archive
- No version conflicts or CDN outages
- Better security (no external code execution)

**Trade-off**: Slightly larger file size (~10KB overhead), but acceptable for the benefits.

### 2. Why Default to HTML?

**Decision**: Make HTML the default export format instead of SVG.

**Rationale**:
- Better user experience with interactive features
- More accessible for non-technical users
- Easier to explore and understand flamegraphs
- SVG still available for those who need it

**Trade-off**: Breaking change for existing workflows, but documented and easy to override.

### 3. Why Client-Side JavaScript?

**Decision**: Use vanilla JavaScript for interactivity instead of a framework.

**Rationale**:
- No build step or dependencies
- Smaller file size
- Faster load time
- Works in all modern browsers
- Easier to maintain and debug

**Trade-off**: More verbose code, but acceptable for the simplicity.

### 4. Why CSS Media Queries for Dark Mode?

**Decision**: Use `prefers-color-scheme` media query instead of a toggle.

**Rationale**:
- Automatic adaptation to system theme
- No UI clutter with theme toggle
- Consistent with OS-level preferences
- Zero JavaScript required for theme switching

**Trade-off**: Users can't override system preference, but this is the standard approach.

## Performance Characteristics

### File Size
- Base SVG: ~50-500KB (depends on complexity)
- HTML wrapper: ~10KB (CSS + JS)
- Total overhead: ~2-20% increase

### Load Time
- Instant (local file, no network requests)
- No external dependencies to fetch
- No build or compilation step

### Runtime Performance
- Hover: O(1) - Direct event listener
- Click-to-zoom: O(1) - SVG viewBox manipulation
- Search: O(n) - Linear scan of all frames
- Memory: O(n) - Proportional to number of frames

### Browser Compatibility
- Chrome/Edge 88+
- Firefox 78+
- Safari 14+
- Opera 74+

## Security Considerations

### No External Dependencies
- All code is inlined (no CDN or external scripts)
- No network requests after initial file load
- No third-party libraries or frameworks

### Content Security Policy
The generated HTML is compatible with strict CSP:
```
Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'
```

### XSS Protection
- SVG content is generated by trusted Rust code (inferno crate)
- No user input is directly embedded in HTML
- All dynamic content is from simulation results

## Testing Strategy

### Unit Tests
- `TestGenerateInteractiveHTML_BasicGeneration` - HTML structure
- `TestGenerateInteractiveHTML_EmptyInput` - Edge cases
- `TestGenerateInteractiveHTML_PreservesSVGContent` - Content integrity
- `TestExportFormat_GetFileExtension` - Format mapping
- `TestExportFlamegraph_SVGFormat` - SVG export path
- `TestExportFlamegraph_HTMLFormat` - HTML export path
- `TestExportFlamegraph_DefaultFormat` - Default behavior

### Integration Tests
- Verify CLI flag parsing
- Test export logic in debug command
- Validate file creation and permissions

### Manual Tests
- Open HTML in multiple browsers
- Test interactive features (hover, click, search)
- Verify dark mode adaptation
- Check for console errors
- Verify no network requests

## Future Enhancements

### Planned Features
1. **Export to PNG/PDF** - For presentations and reports
2. **Configurable Color Schemes** - Custom color palettes
3. **Diff View** - Compare two flamegraphs side-by-side
4. **Flame Chart View** - Time-based visualization
5. **Keyboard Shortcuts** - Navigate without mouse
6. **Permalink Support** - Share specific views via URL hash

### Technical Improvements
1. **Lazy Loading** - For very large flamegraphs
2. **Virtual Scrolling** - Better performance with many frames
3. **WebAssembly** - Faster search and filtering
4. **Service Worker** - Offline caching for web version
5. **Progressive Enhancement** - Graceful degradation for older browsers

## References

- [Flamegraph Visualization](https://www.brendangregg.com/flamegraphs.html)
- [Inferno Flamegraph Library](https://github.com/jonhoo/inferno)
- [SVG Specification](https://www.w3.org/TR/SVG2/)
- [CSS Media Queries](https://developer.mozilla.org/en-US/docs/Web/CSS/Media_Queries)
- [Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP)
