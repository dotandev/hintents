# Interactive Trace Viewer

The interactive trace viewer provides a powerful search and navigation interface for exploring Stellar Soroban transaction execution traces.

## Features

### Search Functionality

- **Case-insensitive search** by default
- **Highlights all matches** in yellow
- **Current match highlighted** in green with arrow indicator
- **Search across all fields**: contract IDs, function names, errors, events, and types
- **Match counter**: Shows "Match X of Y" status
- **Quick navigation**: Jump between matches with `n` and `N`

### Tree Navigation

- **Hierarchical view** of execution trace
- **Expand/collapse** nodes with Enter/Space
- **Expand all** with `e`
- **Collapse all** with `c`
- **Smooth scrolling** with arrow keys, PgUp/PgDn, Home/End
- **Visual indicators** for expanded (v) and collapsed (>) nodes

### Visual Styling

- **Color-coded elements**:
  - Contract IDs in cyan
  - Function names in blue
  - Errors in red
  - Search matches in yellow
  - Current match in green
- **Depth indentation** for clear hierarchy
- **Cursor indicator** (>) for current position

## Usage

### Launching the Viewer

```bash
# Debug a transaction with interactive viewer
./erst debug <transaction-hash> --interactive

# Or use the short flag
./erst debug <transaction-hash> -i
```

### Keyboard Shortcuts

#### Navigation

| Key               | Action                 |
| ----------------- | ---------------------- |
| `↑` / `k`         | Move up                |
| `↓` / `j`         | Move down              |
| `PgUp`            | Scroll up one page     |
| `PgDn`            | Scroll down one page   |
| `Home` / `g`      | Jump to start          |
| `End` / `G`       | Jump to end            |
| `Enter` / `Space` | Toggle expand/collapse |

#### Search

| Key     | Action                      |
| ------- | --------------------------- |
| `/`     | Start search                |
| `Enter` | Execute search              |
| `n`     | Next match                  |
| `N`     | Previous match              |
| `ESC`   | Clear search / Cancel input |

#### Tree Operations

| Key | Action             |
| --- | ------------------ |
| `e` | Expand all nodes   |
| `c` | Collapse all nodes |

#### Other

| Key            | Action      |
| -------------- | ----------- |
| `q` / `Ctrl+C` | Quit viewer |
| `?` / `h`      | Show shortcuts help |

## Search Examples

### Search for Contract ID

```
Press: /
Type: CDLZFC3
Press: Enter
```

Finds all nodes containing that contract ID prefix.

### Search for Error Messages

```
Press: /
Type: insufficient
Press: Enter
```

Finds all error messages containing "insufficient" (case-insensitive).

### Search for Function Names

```
Press: /
Type: transfer
Press: Enter
```

Finds all function calls named "transfer".

### Navigate Between Matches

After searching:

- Press `n` to jump to next match
- Press `N` to jump to previous match
- Navigation wraps around (last → first → second...)

## Implementation

### Components

- **node.go**: Core data structure representing execution tree
- **search.go**: Search engine with highlighting
- **viewer.go**: Interactive TUI viewer with Bubbletea
- **parser.go**: Converts simulator output to trace tree

### Testing

```bash
# Run all tests
go test ./internal/trace/... -v

# With coverage
go test ./internal/trace/... -cover

# Run benchmarks
go test ./internal/trace -bench=.
```

### Performance

- Search through 1000 nodes: ~10ms
- Match navigation: 65ns (instant)
- Zero allocations for navigation

## Requirements

- Terminal with ANSI color support
- Minimum terminal size: 80x24 recommended

## Troubleshooting

### Search Not Working

- Ensure you press `Enter` after typing search query
- Check that query is not empty
- Try clearing search with `ESC` and searching again

### Viewer Not Launching

- Ensure terminal supports ANSI colors
- Check terminal size (minimum 80x24 recommended)
- Verify bubbletea dependency is installed

## License

Part of the Erst project - see main LICENSE file.
