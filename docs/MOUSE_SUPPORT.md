# Mouse Support in Terminal UI

## Overview

The trace tree viewer now supports mouse interactions, enabling users to click on expand/collapse symbols `[+]` instead of relying solely on keyboard navigation.

## Features

### Mouse Interactions

1. **Click to Expand/Collapse**: Click directly on the `[+]` or `[-]` symbols to toggle node expansion
2. **Mouse Scrolling**: Use scroll wheel to navigate through the tree
3. **Click to Select**: Click on any node to select it
4. **Multi-platform**: Works with SGR1006 mouse reporting format for broad terminal compatibility

### Terminal Support

The implementation supports:
- SGR1006 mode (standard, recommended)
- URXVT mode (fallback)
- Basic X11 mode (fallback)

Compatible terminals:
- xterm
- gnome-terminal
- konsole
- iterm2
- kitty
- Windows Terminal (WSL)
- Most modern terminal emulators

## Usage

### Launching the Tree Viewer with Mouse Support

```bash
./erst trace <trace-file> --mouse
# or
./erst trace <trace-file> -m
```

### Mouse Controls

| Action | Result |
|--------|--------|
| Click `[+]` symbol | Expand node |
| Click `[-]` symbol | Collapse node |
| Click node text | Select node |
| Scroll up | Navigate up |
| Scroll down | Navigate down |

### Keyboard Controls (Compatible)

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Space` / `Enter` | Toggle expand/collapse |
| `e` | Expand all |
| `c` | Collapse all |
| `h` | Show help |
| `q` / `Ctrl+C` | Quit |

## Implementation Details

### Components

#### MouseTracker (`mouse.go`)
- Enables/disables terminal mouse reporting
- Handles SGR1006, URXVT, and basic X11 formats
- Parses mouse events from escape sequences

#### TreeRenderer (`treeui.go`)
- Renders trace tree with proper formatting
- Tracks visual positions of nodes on screen
- Calculates expand box positions for click detection
- Manages selected row and scroll offset
- Supports keyboard navigation

#### TreeViewerWithMouse (`treeviewer_mouse.go`)
- Unified interactive viewer with mouse support
- Event loop for handling user input
- Terminal state management (raw mode)
- View rendering with header/footer

### Mouse Event Parsing

The implementation supports multiple mouse reporting formats:

**SGR1006 Format (Recommended)**
```
\x1b[<button;col;row;M
```
- Button: 0=left, 1=middle, 2=right, 64=scroll-up, 65=scroll-down
- Column/Row: 1-based coordinates

**Basic Format**
```
\x1b[Mcxy
```
- Older format for compatibility
- Limited to 223×223 coordinate space

## Testing

Tests are provided in `internal/trace/mouse_test.go`:

```bash
go test ./internal/trace -v -run TestMouseTracker
go test ./internal/trace -v -run TestTreeRenderer
go test ./internal/trace -v -run TestParseMouseEvent
```

## Architecture

```
┌─────────────────────────────────────────────┐
│     TreeViewerWithMouse (Main Controller)   │
├─────────────────────────────────────────────┤
│  • Event loop (keyboard + mouse)            │
│  • Terminal state management                │
│  • View rendering                           │
└────────────┬────────────────────────────────┘
             │
    ┌────────┴────────┬──────────────┐
    │                 │              │
┌───▼────┐      ┌─────▼────┐   ┌────▼─────┐
│TreeUI   │      │MouseTracker│  │TreeRend- │
│Nodes    │      │            │  │erer      │
└────────┘      └────────────┘  └──────────┘
    │                 │              │
    └─────────────────┼──────────────┘
                      │
              ┌───────▼─────────┐
              │ Terminal I/O    │
              └─────────────────┘
```

## Technical Specifications

### Mouse Event Structure

```go
type MouseEvent struct {
    Button MouseButton  // Type of button pressed
    Col    int          // Column (0-based)
    Row    int          // Row (0-based)
    X      int          // X coordinate
    Y      int          // Y coordinate
}
```

### Terminal Escape Sequences

| Sequence | Purpose |
|----------|---------|
| `\x1b[?1000h` | Enable basic mouse reporting |
| `\x1b[?1006h` | Enable SGR1006 (extended) mode |
| `\x1b[?1015h` | Enable URXVT mode |
| `\x1b[?25l` | Hide cursor |
| `\x1b[?25h` | Show cursor |

## Future Enhancements

1. **Double-click Support**: Expand node and all children
2. **Right-click Context Menu**: Show options for node operations
3. **Drag Selection**: Select multiple nodes
4. **Copy to Clipboard**: Right-click to copy node info
5. **Terminal Size Detection**: Auto-adjust to terminal dimensions
6. **Customizable Colors**: Highlight nodes with colors
7. **Search Integration**: Filter nodes while maintaining tree structure

## Troubleshooting

### Mouse not working
- Ensure terminal supports mouse reporting (most modern terminals do)
- Try a different terminal emulator
- Check if mouse support is enabled in terminal settings

### Coordinates incorrect
- May indicate terminal with different coordinate system
- Try running in different terminal emulator

### Performance issues
- Large trees (1000+ nodes) may scroll slowly
- Consider filtering or pagination for large traces
